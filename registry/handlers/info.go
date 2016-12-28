package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/registry/api/errcode"
	"github.com/gorilla/handlers"
)

func taginfoDispatcher(ctx *Context, r *http.Request) http.Handler {
	taginfoHandler := &taginfoHandler{
		Context: ctx,
	}

	return handlers.MethodHandler{
		"GET": http.HandlerFunc(taginfoHandler.GetTaginfo),
	}
}

type taginfoHandler struct {
	*Context
}

type taginfoAPIResponse struct {
	Name       string    `json:"name"`
	Tag        string    `json:"tag"`
	CreateTime time.Time `json:"createTime"`
	Size       int64     `json:"size"`
}

func (ch *taginfoHandler) GetTaginfo(w http.ResponseWriter, r *http.Request) {
	tag := getTag(ch)
	name := getName(ch)
	cacheservice := ch.Repository.Caches(ch)
	content, err := cacheservice.GetTagInfo(ch, tag)
	var response taginfoAPIResponse
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	// Add a link header if there are more entries to retrieve
	enc := json.NewEncoder(w)
	if err != nil {
		imh := &imageManifestHandler{
			Context: ch.Context,
			Tag:     tag,
		}
		response, err := CreateAndSaveTagInfo(imh, name)
		if err != nil {
			ch.Errors = append(ch.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
			return
		}
		if err := enc.Encode(&response); err != nil {
			ch.Errors = append(ch.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
			return
		}
	} else {
		err = json.Unmarshal(content, &response)
		if err != nil {
			ch.Errors = append(ch.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
			return
		}
		if err := enc.Encode(&response); err != nil {
			ch.Errors = append(ch.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
			return
		}
	}

}

func CreateAndSaveTagInfo(imh *imageManifestHandler, name string) (taginfoAPIResponse, error) {
	schema2manifest, err := GetTagManifests(imh)
	if err != nil {
		return taginfoAPIResponse{}, err
	}
	manifest, err := imh.convertSchema2Manifest(schema2manifest)
	if err != nil {
		return taginfoAPIResponse{}, err
	}
	scheme1manifest, _ := manifest.(*schema1.SignedManifest)
	type v1Compatibility struct {
		ID              string    `json:"id"`
		Parent          string    `json:"parent,omitempty"`
		Comment         string    `json:"comment,omitempty"`
		Created         time.Time `json:"created"`
		ContainerConfig struct {
			Cmd []string
		} `json:"container_config,omitempty"`
		Author    string `json:"author,omitempty"`
		ThrowAway bool   `json:"throwaway,omitempty"`
	}
	var createTime time.Time
	for _, history := range scheme1manifest.History {
		var historyinfo v1Compatibility
		json.Unmarshal([]byte(history.V1Compatibility), &historyinfo)
		if createTime.Before(historyinfo.Created) {
			createTime = historyinfo.Created
		}
	}
	var size int64
	for _, d := range schema2manifest.References() {
		size += d.Size
	}
	response := taginfoAPIResponse{
		Name:       name,
		Tag:        imh.Tag,
		CreateTime: createTime,
		Size:       size,
	}
	jsonContent, err := json.Marshal(response)
	cacheservice := imh.Repository.Caches(imh)
	cacheservice.SaveTagInfo(imh, imh.Tag, jsonContent)
	return response, nil

}
