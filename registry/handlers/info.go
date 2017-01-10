package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/registry/api/errcode"
	"github.com/gorilla/handlers"
)

func imageinfoDispatcher(ctx *Context, r *http.Request) http.Handler {
	imageinfoHandler := &infoHandler{
		Context: ctx,
	}
	return handlers.MethodHandler{
		"GET": http.HandlerFunc(imageinfoHandler.GetImageInfo),
	}
}

func taginfoDispatcher(ctx *Context, r *http.Request) http.Handler {
	taginfoHandler := &infoHandler{
		Context: ctx,
	}

	return handlers.MethodHandler{
		"GET": http.HandlerFunc(taginfoHandler.GetTaginfo),
	}
}

type infoHandler struct {
	*Context
}

type taginfoAPIResponse struct {
	Name       string    `json:"name"`
	Tag        string    `json:"tag"`
	CreateTime time.Time `json:"createTime"`
	Size       int64     `json:"size"`
}

type imageinfoAPIResponse struct {
	Name         string               `json:"name"`
	Tags         []taginfoAPIResponse `json:"tags"`
	Size         int                  `json:"size"`
	LastModified time.Time            `json:"lastModified"`
}

func (ih *infoHandler) GetImageInfo(w http.ResponseWriter, r *http.Request) {
	name := getName(ih)
	cacheservice := ih.Repository.Caches(ih)
	content, err := cacheservice.GetImageInfo(ih)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)

	if err != nil {
		imageinfo, err := CreateAndSaveImageInfo(ih.Context, name)
		if err != nil {
			ih.Errors = append(ih.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
			return
		}
		if err := enc.Encode(&imageinfo); err != nil {
			ih.Errors = append(ih.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
			return
		}
	} else {
		var imageinfo imageinfoAPIResponse
		err = json.Unmarshal(content, &imageinfo)
		if err != nil {
			ih.Errors = append(ih.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
			return
		}
		if err := enc.Encode(&imageinfo); err != nil {
			ih.Errors = append(ih.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
			return
		}
	}
}

func CreateAndSaveImageInfo(ctx *Context, name string) (imageinfoAPIResponse, error) {
	tagservice := ctx.Repository.Tags(ctx)
	cacheservice := ctx.Repository.Caches(ctx)
	taglist, err := tagservice.All(ctx)
	if err != nil {
		return imageinfoAPIResponse{}, err
	}
	tags := make([]taginfoAPIResponse, len(taglist))
	var lastModified time.Time
	for i, tagname := range taglist {
		content, err := cacheservice.GetTagInfo(ctx, tagname)
		if err != nil {
			continue
		}
		var taginfo taginfoAPIResponse
		if err = json.Unmarshal(content, &taginfo); err != nil {
			continue
		}
		if lastModified.Before(taginfo.CreateTime) {
			lastModified = taginfo.CreateTime
		}
		tags[i] = taginfo
	}
	imageinfo := imageinfoAPIResponse{
		Name:         name,
		Tags:         tags,
		Size:         len(taglist),
		LastModified: lastModified,
	}
	jsonContent, _ := json.Marshal(imageinfo)
	cacheservice.SaveImageInfo(ctx, jsonContent)
	return imageinfo, nil
}

func (ih *infoHandler) GetTaginfo(w http.ResponseWriter, r *http.Request) {
	tag := getTag(ih)
	name := getName(ih)
	cacheservice := ih.Repository.Caches(ih)
	content, err := cacheservice.GetTagInfo(ih, tag)
	var response taginfoAPIResponse
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	// Add a link header if there are more entries to retrieve
	enc := json.NewEncoder(w)
	if err != nil {
		imh := &imageManifestHandler{
			Context: ih.Context,
			Tag:     tag,
		}
		response, err := CreateAndSaveTagInfo(imh, name)
		if err != nil {
			ih.Errors = append(ih.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
			return
		}
		if err := enc.Encode(&response); err != nil {
			ih.Errors = append(ih.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
			return
		}
	} else {
		err = json.Unmarshal(content, &response)
		if err != nil {
			ih.Errors = append(ih.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
			return
		}
		if err := enc.Encode(&response); err != nil {
			ih.Errors = append(ih.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
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
