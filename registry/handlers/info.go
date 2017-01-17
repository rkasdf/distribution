package handlers

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/registry/api/errcode"
	"github.com/gorilla/handlers"
)

func cataloginfoDispatcher(ctx *Context, r *http.Request) http.Handler {
	cataloginfoHandler := &infoHandler{
		Context: ctx,
	}
	return handlers.MethodHandler{
		"GET": http.HandlerFunc(cataloginfoHandler.GetCatalogInfo),
	}
}

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
	Name          string    `json:"name"`
	Tag           string    `json:"tag"`
	CreateTime    time.Time `json:"createTime"`
	DownloadCount int       `json:"downloadCount"`
	Size          int64     `json:"size"`
}

type imageinfoAPIResponse struct {
	Name          string               `json:"name"`
	Tags          []taginfoAPIResponse `json:"tags"`
	Size          int                  `json:"size"`
	DownloadCount int                  `json:"downloadCount"`
	LastModified  time.Time            `json:"lastModified"`
	CreateTime    time.Time            `json:"createTime"`
}

type cataloginfoAPIResponse struct {
	ImageInfos []imageinfoAPIResponse `json:"imageInfos"`
}

var catalogLock sync.RWMutex

func (ih *infoHandler) GetImageInfo(w http.ResponseWriter, r *http.Request) {
	cacheservice := ih.Repository.Caches(ih)
	content, err := cacheservice.GetImageInfo(ih)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)

	if err != nil {
		ih.Errors = append(ih.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
		return
	}
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

// UpdateDownloadCount for pull image, when pulling image the downloadCount will add 1
func updateDownloadCount(imh *imageManifestHandler, name, tag string) error {
	cacheservice := imh.Repository.Caches(imh)
	taginfocontent, err := cacheservice.GetTagInfo(imh, tag)
	var taginfo taginfoAPIResponse
	if err != nil {
		if taginfo, err = createAndSaveTagInfo(imh, name); err != nil {
			return err
		}
	} else {
		if err = json.Unmarshal(taginfocontent, &taginfo); err != nil {
			return err
		}
		if taginfo.DownloadCount < 1 {
			taginfo.DownloadCount = 1
		} else {
			taginfo.DownloadCount++
		}
		if taginfocontent, err = json.Marshal(taginfo); err != nil {
			return err
		}
		if err = cacheservice.SaveTagInfo(imh, tag, taginfocontent); err != nil {
			return err
		}
	}
	return nil
}

// CreateAndSaveImageInfo for push image use
// When info file exist, the downloadCount will add 1, if not exist, will create a new file
func createAndSaveImageInfo(ctx *Context, name string) (imageinfoAPIResponse, error) {
	tagservice := ctx.Repository.Tags(ctx)
	cacheservice := ctx.Repository.Caches(ctx)
	taglist, err := tagservice.All(ctx)
	if err != nil {
		return imageinfoAPIResponse{}, err
	}
	tags := make([]taginfoAPIResponse, len(taglist))
	var lastModified time.Time
	downloadCount := 0
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
		downloadCount += taginfo.DownloadCount
		tags[i] = taginfo
	}
	imageinfo := imageinfoAPIResponse{
		Name:          name,
		Tags:          tags,
		Size:          len(taglist),
		DownloadCount: downloadCount,
		LastModified:  lastModified,
		CreateTime:    lastModified,
	}
	infocontent, err := cacheservice.GetImageInfo(ctx)
	if err == nil {
		var existinfo imageinfoAPIResponse
		if err = json.Unmarshal(infocontent, &existinfo); err == nil && !existinfo.CreateTime.IsZero() {
			imageinfo.CreateTime = existinfo.CreateTime
		}
	}

	jsonContent, _ := json.Marshal(imageinfo)
	cacheservice.SaveImageInfo(ctx, jsonContent)

	return imageinfo, nil
}

func (ih *infoHandler) GetCatalogInfo(w http.ResponseWriter, r *http.Request) {
	blobCache := ih.registry.BlobCache()
	content, err := blobCache.GetCatalogInfo(ih)
	if err != nil {
		ih.Errors = append(ih.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
		return
	}
	enc := json.NewEncoder(w)
	var response cataloginfoAPIResponse
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

func (ih *infoHandler) GetTaginfo(w http.ResponseWriter, r *http.Request) {
	tag := getTag(ih)
	cacheservice := ih.Repository.Caches(ih)
	content, err := cacheservice.GetTagInfo(ih, tag)
	var response taginfoAPIResponse
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	// Add a link header if there are more entries to retrieve
	enc := json.NewEncoder(w)
	if err != nil {
		ih.Errors = append(ih.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
		return
	}
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

func createAndSaveTagInfo(imh *imageManifestHandler, name string) (taginfoAPIResponse, error) {
	schema2manifest, convertedManifest, err := getTagManifests(imh)
	if err != nil {
		return taginfoAPIResponse{}, err
	}
	var scheme1manifest *schema1.SignedManifest
	if convertedManifest != nil {
		scheme1manifest, _ = convertedManifest.(*schema1.SignedManifest)
	} else {
		scheme1manifest, _ = schema2manifest.(*schema1.SignedManifest)
	}

	type v1Compatibility struct {
		ID              string    `json:"id"`
		Parent          string    `json:"parent,omitempty"`
		Comment         string    `json:"comment,omitempty"`
		Created         time.Time `json:"created"`
		Size            int64     `json:"size"`
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
	blobs := imh.Repository.Blobs(imh)
	var size int64
	for _, d := range schema2manifest.References() {
		desc, err := blobs.Stat(imh, d.Digest)
		if err != nil {
			return taginfoAPIResponse{}, err
		}
		size += desc.Size
	}
	response := taginfoAPIResponse{
		Name:          name,
		Tag:           imh.Tag,
		DownloadCount: 0,
		CreateTime:    createTime,
		Size:          size,
	}
	cacheservice := imh.Repository.Caches(imh)
	infocontent, err := cacheservice.GetTagInfo(imh, imh.Tag)
	if err == nil {
		var existinfo taginfoAPIResponse
		if err = json.Unmarshal(infocontent, &existinfo); err == nil && existinfo.DownloadCount > 0 {
			response.DownloadCount = existinfo.DownloadCount
		}
	}
	jsonContent, err := json.Marshal(response)
	cacheservice.SaveTagInfo(imh, imh.Tag, jsonContent)
	return response, nil

}

func updateCatalogInfo(ctx *Context, imageInfo imageinfoAPIResponse) error {
	catalogLock.Lock()
	defer catalogLock.Unlock()
	cacheService := ctx.Repository.Caches(ctx)
	content, err := cacheService.GetCatalogInfo(ctx)
	if err != nil {
		return err
	}
	var cataloginfo cataloginfoAPIResponse
	if err = json.Unmarshal(content, &cataloginfo); err != nil {
		return err
	}
	for i, image := range cataloginfo.ImageInfos {
		if image.Name == imageInfo.Name {
			cataloginfo.ImageInfos[i] = imageInfo
			break
		}
	}
	if content, err = json.Marshal(cataloginfo); err != nil {
		return err
	}
	return cacheService.SaveCatalogInfo(ctx, content)
}
