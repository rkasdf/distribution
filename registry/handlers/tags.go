package handlers

import (
	"encoding/json"
	"net/http"

	"strings"

	"github.com/docker/distribution"
	"github.com/docker/distribution/registry/api/errcode"
	"github.com/docker/distribution/registry/api/v2"
	"github.com/docker/distribution/registry/storage/driver"
	"github.com/gorilla/handlers"
)

// tagsDispatcher constructs the tags handler api endpoint.
func tagsDispatcher(ctx *Context, r *http.Request) http.Handler {
	tagsHandler := &tagsHandler{
		Context: ctx,
	}

	return handlers.MethodHandler{
		"GET": http.HandlerFunc(tagsHandler.GetTags),
	}
}

// tagsHandler handles requests for lists of tags under a repository name.
type tagsHandler struct {
	*Context
}

type tagsAPIResponse struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

// GetTags returns a json list of tags for a specific image name.
func (th *tagsHandler) GetTags(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	q := r.URL.Query()
	cached := q.Get("cache")
	var err error
	var tags []string
	if th.isEnhanced && !strings.EqualFold(cached, "0") {
		cacheService := th.Repository.Caches(th)
		tags, err = cacheService.GetTagList(th)
		_, pathNotFound := err.(driver.PathNotFoundError)
		if pathNotFound {
			err = cacheService.CreateTagListCache(th)
			tags, err = cacheService.GetTagList(th)
		}
	} else {
		ts := th.Repository.Tags(th)
		tags, err = ts.All(th)
	}
	if err != nil {
		switch err := err.(type) {
		case distribution.ErrRepositoryUnknown:
			th.Errors = append(th.Errors, v2.ErrorCodeNameUnknown.WithDetail(map[string]string{"name": th.Repository.Named().Name()}))
		case errcode.Error:
			th.Errors = append(th.Errors, err)
		default:
			th.Errors = append(th.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
		}
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	enc := json.NewEncoder(w)
	if err := enc.Encode(tagsAPIResponse{
		Name: th.Repository.Named().Name(),
		Tags: tags,
	}); err != nil {
		th.Errors = append(th.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
		return
	}
}
