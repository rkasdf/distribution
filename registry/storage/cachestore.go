package storage

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
)

var _ distribution.CacheService = &cacheStore{}

const maxSize = 100000

// tagStore provides methods to manage manifest tags in a backend storage driver.
// This implementation uses the same on-disk layout as the (now deleted) tag
// store.  This provides backward compatibility with current registry deployments
// which only makes use of the Digest field of the returned distribution.Descriptor
// but does not enable full roundtripping of Descriptor objects
type cacheStore struct {
	repository *repository
	blobCache  *blobCache
}

type catalog struct {
	Repositories []string `json:"repositories"`
}

type tagList struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

func (cs *cacheStore) CreateCatalogCache(ctx context.Context, size int) error {
	if size < maxSize {
		size = maxSize
	}
	repos := make([]string, size)
	filled, err := cs.repository.Repositories(ctx, repos, "")
	if err != nil && err != io.EOF {
		return err
	}
	content, err := json.Marshal(catalog{
		Repositories: repos[0:filled],
	})
	if err != nil {
		return err
	}
	return cs.blobCache.CacheCatalog(ctx, content)
}

func (cs *cacheStore) CreateTagListCache(ctx context.Context) error {
	ts := cs.repository.Tags(ctx)
	tags, err := ts.All(ctx)
	if err != nil {
		return err
	}
	name := cs.repository.Named().Name()
	if err != nil {
		return err
	}
	content, err := json.Marshal(tagList{
		Name: name,
		Tags: tags,
	})
	return cs.blobCache.CacheTagList(ctx, content, name)

}

func (cs *cacheStore) GetCatalog(ctx context.Context) ([]string, error) {
	content, err := cs.blobCache.GetCatalog(ctx)
	if err != nil {
		return nil, err
	}
	var c catalog
	err = json.Unmarshal(content, &c)
	if err != nil {
		return nil, err
	}
	return c.Repositories, err
}

func (cs *cacheStore) GetTagList(ctx context.Context) ([]string, error) {
	name := cs.repository.Named().Name()
	content, err := cs.blobCache.GetTagList(ctx, name)
	if err != nil {
		return nil, err
	}
	var tl tagList
	err = json.Unmarshal(content, &tl)
	if err != nil {
		return nil, err
	}
	return tl.Tags, nil
}

func (cs *cacheStore) SaveTagInfo(ctx context.Context, tag string, content []byte) error {
	name := cs.repository.Named().Name()
	return cs.blobCache.SaveTagInfo(ctx, content, name, tag)
}

func (cs *cacheStore) GetTagInfo(ctx context.Context, tag string) ([]byte, error) {
	name := cs.repository.Named().Name()
	return cs.blobCache.GetTagInfo(ctx, name, tag)
}

func (cs *cacheStore) GetImageItemList(ctx context.Context) ([]string, error) {
	name := cs.repository.Named().Name()
	return cs.blobCache.GetImageItemList(ctx, name)
}

func (cs *cacheStore) GetTagItemList(ctx context.Context, tag string) ([]string, error) {
	name := cs.repository.Named().Name()
	return cs.blobCache.GetTagItemList(ctx, name, tag)
}

func (cs *cacheStore) SaveImageItem(ctx context.Context, w http.ResponseWriter, r *http.Request, item string) error {
	name := cs.repository.Named().Name()
	return cs.blobCache.SaveImageItem(ctx, w, r, name, item)
}

func (cs *cacheStore) GetImageItem(ctx context.Context, w http.ResponseWriter, r *http.Request, item string) error {
	name := cs.repository.Named().Name()
	isp, err := pathFor(imageItemSavePathSpec{
		name: name,
		item: item,
	})
	if err != nil {
		return err
	}
	return cs.blobCache.ServeItem(ctx, w, r, isp, item)
}

func (cs *cacheStore) SaveTagItem(ctx context.Context, w http.ResponseWriter, r *http.Request, tag, item string) error {
	name := cs.repository.Named().Name()
	return cs.blobCache.SaveTagItem(ctx, w, r, name, tag, item)

}

func (cs *cacheStore) GetTagItem(ctx context.Context, w http.ResponseWriter, r *http.Request, tag, item string) error {
	name := cs.repository.Named().Name()
	sps, err := pathFor(tagItemSavePathSpec{
		name: name,
		tag:  tag,
		item: item,
	})
	if err != nil {
		return err
	}
	return cs.blobCache.ServeItem(ctx, w, r, sps, item)
}
