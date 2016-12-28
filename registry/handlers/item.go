package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/docker/distribution/registry/api/errcode"
	"github.com/gorilla/handlers"
)

func imageItemDispatcher(ctx *Context, r *http.Request) http.Handler {
	imageItemHandler := &itemHandler{
		Context: ctx,
	}

	return handlers.MethodHandler{
		"GET":  http.HandlerFunc(imageItemHandler.GetImageItem),
		"POST": http.HandlerFunc(imageItemHandler.SaveImageItem),
	}
}

func imageItemListDispatcher(ctx *Context, r *http.Request) http.Handler {
	imageItemHandler := &itemHandler{
		Context: ctx,
	}

	return handlers.MethodHandler{
		"GET": http.HandlerFunc(imageItemHandler.GetImageItemNameList),
	}
}

func tagItemListDispatcher(ctx *Context, r *http.Request) http.Handler {
	tagItemHandler := &itemHandler{
		Context: ctx,
	}

	return handlers.MethodHandler{
		"GET": http.HandlerFunc(tagItemHandler.GetTagItemNameList),
	}
}

func tagItemDispatcher(ctx *Context, r *http.Request) http.Handler {
	tagItemHandler := &itemHandler{
		Context: ctx,
	}

	return handlers.MethodHandler{
		"GET":  http.HandlerFunc(tagItemHandler.GetTagItem),
		"POST": http.HandlerFunc(tagItemHandler.SaveTagItem),
	}
}

type itemHandler struct {
	*Context
}

type customizedfileinfo struct {
	name    string
	content string
}

func (ih *itemHandler) GetImageItemNameList(w http.ResponseWriter, r *http.Request) {
	cacheservice := ih.Repository.Caches(ih)
	names, err := cacheservice.GetImageItemList(ih)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	// Add a link header if there are more entries to retrieve
	enc := json.NewEncoder(w)
	if err != nil {
		ih.Errors = append(ih.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
		return
	}
	if err := enc.Encode(&names); err != nil {
		ih.Errors = append(ih.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
		return
	}

}

func (ih *itemHandler) GetTagItemNameList(w http.ResponseWriter, r *http.Request) {
	tag := getTag(ih)
	cacheservice := ih.Repository.Caches(ih)
	names, err := cacheservice.GetTagItemList(ih, tag)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	// Add a link header if there are more entries to retrieve
	enc := json.NewEncoder(w)
	if err != nil {
		if err := enc.Encode(&names); err != nil {
			ih.Errors = append(ih.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
			return
		}
	}
	if err := enc.Encode(&names); err != nil {
		ih.Errors = append(ih.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
		return
	}

}

func (ih *itemHandler) GetImageItem(w http.ResponseWriter, r *http.Request) {
	item := getItem(ih)
	cacheservice := ih.Repository.Caches(ih)
	err := cacheservice.GetImageItem(ih, w, r, item)
	if err != nil {
		ih.Errors = append(ih.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
		return
	}
}

func (ih *itemHandler) GetTagItem(w http.ResponseWriter, r *http.Request) {
	item := getItem(ih)
	tag := getTag(ih)
	cacheservice := ih.Repository.Caches(ih)
	err := cacheservice.GetTagItem(ih, w, r, tag, item)
	if err != nil {
		ih.Errors = append(ih.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
		return
	}
}

func (ih *itemHandler) SaveImageItem(w http.ResponseWriter, r *http.Request) {
	item := getItem(ih)
	cacheservice := ih.Repository.Caches(ih)
	err := cacheservice.SaveImageItem(ih, w, r, item)
	if err != nil {
		ih.Errors = append(ih.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
		return
	}
}

func (ih *itemHandler) SaveTagItem(w http.ResponseWriter, r *http.Request) {
	item := getItem(ih)
	tag := getTag(ih)
	cacheservice := ih.Repository.Caches(ih)
	err := cacheservice.SaveTagItem(ih, w, r, tag, item)
	if err != nil {
		ih.Errors = append(ih.Errors, errcode.ErrorCodeUnknown.WithDetail(err))
		return
	}
}
