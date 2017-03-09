package distribution

import (
	"net/http"

	"github.com/docker/distribution/context"
)

// CacheService provides access to information about cached objects.
type CacheService interface {
	// Create catalog cache will cache the catalog list so that when use
	// catalog api will have a fast speed.
	CreateCatalogCache(ctx context.Context, size int) error

	UpdateCatalogCache(ctx context.Context, imageName string) error

	CreateTagListCache(ctx context.Context) error

	DeleteTagFromTagListCache(ctx context.Context, tag string) error

	GetCatalog(ctx context.Context) ([]string, error)

	GetTagList(ctx context.Context) ([]string, error)

	SaveTagInfo(ctx context.Context, tag string, content []byte) error

	SaveImageInfo(ctx context.Context, content []byte) error

	GetImageInfo(ctx context.Context) ([]byte, error)

	GetTagInfo(ctx context.Context, tag string) ([]byte, error)

	GetImageItemList(ctx context.Context) ([]string, error)

	GetTagItemList(ctx context.Context, tag string) ([]string, error)

	SaveImageItem(ctx context.Context, w http.ResponseWriter, r *http.Request, item string) error

	DeleteImageItem(ctx context.Context, item string) error

	DeleteAllImageItems(ctx context.Context) error

	GetImageItem(ctx context.Context, w http.ResponseWriter, r *http.Request, item string) error

	SaveTagItem(ctx context.Context, w http.ResponseWriter, r *http.Request, tag, item string) error

	GetTagItem(ctx context.Context, w http.ResponseWriter, r *http.Request, tag, item string) error

	DeleteTagItem(ctx context.Context, tag, item string) error

	DeleteAllTagItems(ctx context.Context, tag string) error

	InitItem(ctx context.Context, tag string) error

	SaveCatalogInfo(ctx context.Context, content []byte) error

	GetCatalogInfo(ctx context.Context) ([]byte, error)

	DeleteImageRepository(ctx context.Context) error

	DeleteImageFromCatalogCache(ctx context.Context, imageName string) error

	// Get(ctx context.Context, tag string) (Descriptor, error)

	// // Tag associates the tag with the provided descriptor, updating the
	// // current association, if needed.
	// Tag(ctx context.Context, tag string, desc Descriptor) error

	// // Untag removes the given tag association
	// Untag(ctx context.Context, tag string) error

	// // All returns the set of tags managed by this tag service
	// All(ctx context.Context) ([]string, error)

	// // Lookup returns the set of tags referencing the given digest.
	// Lookup(ctx context.Context, digest Descriptor) ([]string, error)
}
