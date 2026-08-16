// Package objectstore is the seam over binary attachment storage that #60
// (Message attachments) puts in front of the Go BFF's handlers so a
// create-Message request never calls the GCS SDK directly. GCSStore is the
// real, GCS-backed implementation; MemoryStore is the in-memory fake
// injected by tests, per #56's "no real GCS bucket reachable from api/
// tests" testing decision. Intended to be reused by a future Contract-PDF
// feature without a new interface.
package objectstore

import (
	"context"
	"io"
)

// ObjectStore puts and gets a single object by its path. Callers validate
// size and content type themselves before calling Put -- the store does no
// validation of its own, per #60's "not delegated to the store" AC.
type ObjectStore interface {
	// Put uploads r's contents to path with the given contentType,
	// overwriting any existing object there.
	Put(ctx context.Context, path, contentType string, r io.Reader) error
	// Get opens a reader for path's contents. Callers must Close it.
	Get(ctx context.Context, path string) (io.ReadCloser, error)
}
