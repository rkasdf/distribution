package storage

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"

	"strings"

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

var catalogLock sync.RWMutex

func (cs *cacheStore) CreateCatalogCache(ctx context.Context, size int) error {
	catalogLock.Lock()
	defer catalogLock.Unlock()
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

func (cs *cacheStore) UpdateCatalogCache(ctx context.Context, imageName string) error {
	catalogLock.Lock()
	defer catalogLock.Unlock()
	repos, err := cs.GetCatalog(ctx)
	if err != nil {
		return cs.CreateCatalogCache(ctx, 1)
	}
	begin, end := 0, len(repos)-1
	for begin < end {
		middle := begin + (end-begin)/2
		ret := strings.Compare(repos[middle], imageName)
		if ret > 0 {
			end = middle - 1
		} else if ret < 0 {
			begin = middle + 1
		} else {
			return nil
		}
	}
	flag := begin
	if strings.Compare(repos[begin], imageName) < 0 {
		flag++
	} else if strings.Compare(repos[begin], imageName) == 0 {
		return nil
	}
	tmp := append([]string{}, repos[flag:]...)
	repos = append(append(repos[:flag], imageName), tmp...)
	content, err := json.Marshal(catalog{
		Repositories: repos,
	})
	if err != nil {
		return err
	}
	return cs.blobCache.CacheCatalog(ctx, content)

}

func (cs *cacheStore) DeleteImageFromCatalogCache(ctx context.Context, imageName string) error {
	repos, err := cs.GetCatalog(ctx)
	if err != nil {
		return cs.CreateCatalogCache(ctx, 1)
	}
	begin, end := 0, len(repos)-1
	flag := -1
	for begin < end {
		middle := begin + (end-begin)/2
		ret := strings.Compare(repos[middle], imageName)
		if ret > 0 {
			end = middle - 1
		} else if ret < 0 {
			begin = middle + 1
		} else {
			flag = middle
			break
		}
	}
	if flag > -1 || strings.Compare(repos[begin], imageName) == 0 {
		repos = append(repos[0:begin], repos[begin+1:]...)
		content, err := json.Marshal(catalog{
			Repositories: repos,
		})
		if err != nil {
			return err
		}
		return cs.blobCache.CacheCatalog(ctx, content)
	}
	return nil

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
	if err != nil {
		return err
	}
	return cs.blobCache.CacheTagList(ctx, content, name)

}

func (cs *cacheStore) DeleteTagFromTagListCache(ctx context.Context, tag string) error {
	tags, err := cs.GetTagList(ctx)
	if err != nil {
		return cs.CreateTagListCache(ctx)
	}
	index := -1
	for i, tagname := range tags {
		if strings.EqualFold(tag, tagname) {
			index = i
			break
		}
	}
	if index > -1 {
		tags = append(tags[:index], tags[index+1:]...)
		name := cs.repository.Named().Name()
		if err != nil {
			return err
		}
		content, err := json.Marshal(tagList{
			Name: name,
			Tags: tags,
		})
		if err != nil {
			return err
		}
		return cs.blobCache.CacheTagList(ctx, content, name)
	}
	return nil
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

func (cs *cacheStore) SaveImageInfo(ctx context.Context, content []byte) error {
	name := cs.repository.Named().Name()
	return cs.blobCache.SaveImageInfo(ctx, content, name)
}

func (cs *cacheStore) GetImageInfo(ctx context.Context) ([]byte, error) {
	name := cs.repository.Named().Name()
	return cs.blobCache.GetImageInfo(ctx, name)
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

func (cs *cacheStore) DeleteImageItem(ctx context.Context, item string) error {
	name := cs.repository.Named().Name()
	return cs.blobCache.DeleteImageItem(ctx, name, item)
}

func (cs *cacheStore) DeleteAllImageItems(ctx context.Context) error {
	name := cs.repository.Named().Name()
	return cs.blobCache.DeleteAllImageItems(ctx, name)
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

func (cs *cacheStore) DeleteTagItem(ctx context.Context, tag, item string) error {
	name := cs.repository.Named().Name()
	return cs.blobCache.DeleteTagItem(ctx, name, tag, item)

}

func (cs *cacheStore) DeleteAllTagItems(ctx context.Context, tag string) error {
	name := cs.repository.Named().Name()
	return cs.blobCache.DeleteAllTagItems(ctx, name, tag)
}

func (cs *cacheStore) InitItem(ctx context.Context, tag string) error {
	name := cs.repository.Named().Name()
	return cs.blobCache.InitItem(ctx, name, tag)
}

func (cs *cacheStore) SaveCatalogInfo(ctx context.Context, content []byte) error {
	return cs.blobCache.CacheCatalogInfo(ctx, content)
}

func (cs *cacheStore) GetCatalogInfo(ctx context.Context) ([]byte, error) {
	return cs.blobCache.GetCatalogInfo(ctx)
}

func (cs *cacheStore) DeleteImageRepository(ctx context.Context) error {
	name := cs.repository.Named().Name()
	return cs.blobCache.DeleteImageRepository(ctx, name)
}
