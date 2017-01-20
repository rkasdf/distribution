package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"strings"

	"github.com/docker/distribution/registry/api/errcode"
	"github.com/docker/distribution/registry/storage/driver"
	"github.com/gorilla/handlers"
)

const maximumReturnedEntries = 100

const cachedMaxEntries = 100000000

type catalog struct {
	Repositories []string `json:"repositories"`
}

func catalogDispatcher(ctx *Context, r *http.Request) http.Handler {
	catalogHandler := &catalogHandler{
		Context: ctx,
	}

	return handlers.MethodHandler{
		"GET": http.HandlerFunc(catalogHandler.GetCatalog),
	}
}

type catalogHandler struct {
	*Context
}

type catalogAPIResponse struct {
	Repositories []string `json:"repositories"`
}

func (ch *catalogHandler) GetCatalog(w http.ResponseWriter, r *http.Request) {
	var moreEntries = true

	q := r.URL.Query()
	lastEntry := q.Get("last")
	maxEntries, err := strconv.Atoi(q.Get("n"))
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	if ch.isEnhanced {
		if err != nil || maxEntries < 0 {
			maxEntries = maximumReturnedEntries
		}
		cacheservice := ch.App.registry.BlobCache()
		content, err := cacheservice.GetCatalog(ch)

		if err == nil {
			var c catalog
			err = json.Unmarshal(content, &c)
			if err != nil {
				ch.Errors = append(ch.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
				return
			}
			cacherepos := c.Repositories
			var start, end int
			if len(cacherepos) <= maxEntries {
				if len(cacherepos) < cachedMaxEntries {
					maxEntries = len(cacherepos)
					if lastEntry != "" {
						start, end = maxEntries, maxEntries
						for index, name := range cacherepos {
							if strings.EqualFold(string(name), lastEntry) {
								start = index
								break
							}
						}
					} else {
						start, end = 0, maxEntries
					}
					enc := json.NewEncoder(w)
					if err := enc.Encode(catalogAPIResponse{
						Repositories: cacherepos[start:end],
					}); err != nil {
						ch.Errors = append(ch.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
						return
					}
					return
				}
			
		}
	}

	repos := make([]string, maxEntries)

	filled, err := ch.App.registry.Repositories(ch.Context, repos, lastEntry)
	_, pathNotFound := err.(driver.PathNotFoundError)

	if err == io.EOF || pathNotFound {
		moreEntries = false
	} else if err != nil {
		ch.Errors = append(ch.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
		return
	}
	// Add a link header if there are more entries to retrieve
	if moreEntries {
		lastEntry = repos[len(repos)-1]
		urlStr, err := createLinkEntry(r.URL.String(), maxEntries, lastEntry)
		if err != nil {
			ch.Errors = append(ch.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
			return
		}
		w.Header().Set("Link", urlStr)
	}

	enc := json.NewEncoder(w)
	if err := enc.Encode(catalogAPIResponse{
		Repositories: repos[0:filled],
	}); err != nil {
		ch.Errors = append(ch.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
		return
	}
}

// Use the original URL from the request to create a new URL for
// the link header
func createLinkEntry(origURL string, maxEntries int, lastEntry string) (string, error) {
	calledURL, err := url.Parse(origURL)
	if err != nil {
		return "", err
	}

	v := url.Values{}
	v.Add("n", strconv.Itoa(maxEntries))
	v.Add("last", lastEntry)

	calledURL.RawQuery = v.Encode()

	calledURL.Fragment = ""
	urlStr := fmt.Sprintf("<%s>; rel=\"next\"", calledURL.String())

	return urlStr, nil
}
