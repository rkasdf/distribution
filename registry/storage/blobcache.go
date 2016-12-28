package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"strings"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/registry/storage/driver"
)

// blobWriter is used to control the various aspects of resumable
// blob upload.
type itemNameList struct {
	NameList []string `json:"nameList"`
}

type blobCache struct {
	driver driver.StorageDriver
}

var _ distribution.BlobCache = &blobCache{}

func (bc *blobCache) CacheCatalog(ctx context.Context, content []byte) error {
	cp, err := pathFor(catalogCachePathSpec{})
	if err != nil {
		return err
	}
	return bc.driver.PutContent(ctx, cp, content)
}

func (bc *blobCache) GetCatalog(ctx context.Context) ([]byte, error) {
	cp, err := pathFor(catalogCachePathSpec{})
	if err != nil {
		return nil, err
	}
	return bc.driver.GetContent(ctx, cp)
}

func (bc *blobCache) CacheTagList(ctx context.Context, content []byte, name string) error {
	tp, err := pathFor(tagListCachePathSpec{
		name: name,
	})
	if err != nil {
		return err
	}
	return bc.driver.PutContent(ctx, tp, content)
}

func (bc *blobCache) GetTagList(ctx context.Context, name string) ([]byte, error) {
	tp, err := pathFor(tagListCachePathSpec{
		name: name,
	})
	if err != nil {
		return nil, err
	}
	return bc.driver.GetContent(ctx, tp)
}

func (bc *blobCache) SaveTagInfo(ctx context.Context, content []byte, name string, tag string) error {
	tp, err := pathFor(tagInfoCachePathSpec{
		name: name,
		tag:  tag,
	})
	if err != nil {
		return err
	}
	return bc.driver.PutContent(ctx, tp, content)
}

func (bc *blobCache) GetTagInfo(ctx context.Context, name, tag string) ([]byte, error) {
	tp, err := pathFor(tagInfoCachePathSpec{
		name: name,
		tag:  tag,
	})
	if err != nil {
		return nil, err
	}
	return bc.driver.GetContent(ctx, tp)
}

func (bc *blobCache) GetImageItemList(ctx context.Context, name string) ([]string, error) {
	ilp, err := pathFor(imageItemInfoPathSpec{
		name: name,
	})
	if err != nil {
		return nil, err
	}
	content, err := bc.driver.GetContent(ctx, ilp)
	if err != nil {
		return nil, err
	}
	var inl itemNameList
	err = json.Unmarshal(content, &inl)
	if err != nil {
		return nil, err
	}
	return inl.NameList, nil

}
func (bc *blobCache) GetTagItemList(ctx context.Context, name, tag string) ([]string, error) {
	ilp, err := pathFor(tagItemInfoPathSpec{
		name: name,
		tag:  tag,
	})
	if err != nil {
		return nil, err
	}
	content, err := bc.driver.GetContent(ctx, ilp)
	if err != nil {
		return nil, err
	}
	var inl itemNameList
	err = json.Unmarshal(content, &inl)
	if err != nil {
		return nil, err
	}
	return inl.NameList, nil

}
func (bc *blobCache) SaveImageItem(ctx context.Context, w http.ResponseWriter, r *http.Request, name, item string) error {
	isp, err := pathFor(imageItemSavePathSpec{
		name: name,
		item: item,
	})
	if err != nil {
		return err
	}
	fw, err := bc.driver.Writer(ctx, isp, false)
	if err != nil {
		return err
	}
	err = writeItem(ctx, w, r, fw)
	if err != nil {
		return err
	}
	listpath, err := pathFor(imageItemListPathSpec{
		name: name,
	})
	if err != nil {
		return err
	}
	savepath, err := pathFor(imageItemInfoPathSpec{
		name: name,
	})
	if err != nil {
		return err
	}
	iil, err := bc.GetImageItemList(ctx, name)
	if err != nil {
		return updateItemList(ctx, bc, listpath, savepath)
	}
	return addItemIntoItemList(ctx, bc, name, item, listpath, savepath, iil)
}

func (bc *blobCache) SaveTagItem(ctx context.Context, w http.ResponseWriter, r *http.Request, name, tag, item string) error {
	sps, err := pathFor(tagItemSavePathSpec{
		name: name,
		tag:  tag,
		item: item,
	})
	if err != nil {
		return err
	}
	fw, err := bc.driver.Writer(ctx, sps, false)
	if err != nil {
		return err
	}
	err = writeItem(ctx, w, r, fw)
	if err != nil {
		return err
	}
	listpath, err := pathFor(tagItemListPathSpec{
		name: name,
		tag:  tag,
	})
	if err != nil {
		return err
	}
	savepath, err := pathFor(tagItemInfoPathSpec{
		name: name,
		tag:  tag,
	})
	if err != nil {
		return err
	}
	til, err := bc.GetTagItemList(ctx, name, tag)
	if err != nil {
		return updateItemList(ctx, bc, listpath, savepath)
	}
	return addItemIntoItemList(ctx, bc, name, item, listpath, savepath, til)

}

func (bc *blobCache) ServeItem(ctx context.Context, w http.ResponseWriter, r *http.Request, path, item string) error {
	desc, err := bc.driver.Stat(ctx, path)
	if err != nil {
		return err
	}
	br, err := newFileReader(ctx, bc.driver, path, desc.Size())
	if err != nil {
		return err
	}
	defer br.Close()

	// w.Header().Set("ETag", fmt.Sprintf(`"%s"`, item)) // If-None-Match handled by ServeContent

	// if w.Header().Get("Docker-Content-Digest") == "" {
	// 	w.Header().Set("Docker-Content-Digest", item)
	// }

	if w.Header().Get("Content-Length") == "" {
		// Set the content length if not already set.
		w.Header().Set("Content-Length", fmt.Sprint(desc.Size()))
	}
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/octet-stream")
	}

	http.ServeContent(w, r, item, time.Time{}, br)
	return nil
}

func updateItemList(ctx context.Context, bc *blobCache, listpath, savepath string) error {
	iil, err := bc.driver.List(ctx, listpath)
	if err != nil {
		return err
	}
	names := make([]string, len(iil))
	for i, itemname := range iil {
		names[i] = strings.TrimPrefix(itemname, listpath+"/")
	}
	content, err := json.Marshal(itemNameList{
		NameList: names,
	})
	if err != nil {
		return err
	}
	return bc.driver.PutContent(ctx, savepath, content)
}

func addItemIntoItemList(ctx context.Context, bc *blobCache, name, item, listpath, path string, nameList []string) error {
	flag := false
	for _, itemname := range nameList {
		if strings.EqualFold(item, itemname) {
			flag = true
			break
		}
	}
	if !flag {
		nameList = append(nameList, item)
		content, err := json.Marshal(itemNameList{
			NameList: nameList,
		})
		if err != nil {
			return err
		}
		return bc.driver.PutContent(ctx, path, content)
	}
	return nil
}

func writeItem(ctx context.Context, responseWriter http.ResponseWriter, r *http.Request, destWriter driver.FileWriter) error {
	// Get a channel that tells us if the client disconnects
	var clientClosed <-chan bool
	if notifier, ok := responseWriter.(http.CloseNotifier); ok {
		clientClosed = notifier.CloseNotify()
	} else {
		context.GetLogger(ctx).Warnf("the ResponseWriter does not implement CloseNotifier (type: %T)", responseWriter)
	}

	// Read in the data, if any.
	copied, err := io.Copy(destWriter, r.Body)
	destWriter.Commit()
	if clientClosed != nil && (err != nil || (r.ContentLength > 0 && copied < r.ContentLength)) {
		// Didn't receive as much content as expected. Did the client
		// disconnect during the request? If so, avoid returning a 400
		// error to keep the logs cleaner.
		select {
		case <-clientClosed:
			// Set the response code to "499 Client Closed Request"
			// Even though the connection has already been closed,
			// this causes the logger to pick up a 499 error
			// instead of showing 0 for the HTTP status.
			responseWriter.WriteHeader(499)

			context.GetLoggerWithFields(ctx, map[interface{}]interface{}{
				"error":         err,
				"copied":        copied,
				"contentLength": r.ContentLength,
			}, "error", "copied", "contentLength").Error("client disconnected during upload item.")
			return errors.New("client disconnected")
		default:
		}
	}

	if err != nil {
		context.GetLogger(ctx).Errorf("unknown error reading request payload: %v", err)
		return err
	}

	return nil
}
