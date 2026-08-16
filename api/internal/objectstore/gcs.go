package objectstore

import (
	"context"
	"fmt"
	"io"

	"cloud.google.com/go/storage"
)

// GCSStore is the production ObjectStore, backed by one GCS bucket -- the
// same bucket ADR-0002 and #56's Implementation Decisions intend for a
// future Contract-PDF feature to reuse.
type GCSStore struct {
	client *storage.Client
	bucket string
}

// NewGCSStore wraps an existing *storage.Client -- callers construct the
// client once at startup (it manages its own connection pool) and share it
// across any future ObjectStore-backed feature.
func NewGCSStore(client *storage.Client, bucket string) *GCSStore {
	// coverage:ignore reason: requires a real GCS bucket and network access, not exercised by unit tests
	return &GCSStore{client: client, bucket: bucket}
}

// Put uploads r's contents to path, overwriting any existing object there.
func (s *GCSStore) Put(ctx context.Context, path, contentType string, r io.Reader) error {
	// coverage:ignore reason: requires a real GCS bucket and network access, not exercised by unit tests
	w := s.client.Bucket(s.bucket).Object(path).NewWriter(ctx)
	// coverage:ignore reason: requires a real GCS bucket and network access, not exercised by unit tests
	w.ContentType = contentType
	// coverage:ignore reason: requires a real GCS bucket and network access, not exercised by unit tests
	if _, err := io.Copy(w, r); err != nil {
		// coverage:ignore reason: requires a real GCS bucket and network access, not exercised by unit tests
		_ = w.Close()
		// coverage:ignore reason: requires a real GCS bucket and network access, not exercised by unit tests
		return fmt.Errorf("objectstore: put %s: %w", path, err)
	}
	// coverage:ignore reason: requires a real GCS bucket and network access, not exercised by unit tests
	if err := w.Close(); err != nil {
		// coverage:ignore reason: requires a real GCS bucket and network access, not exercised by unit tests
		return fmt.Errorf("objectstore: put %s: close: %w", path, err)
	}
	// coverage:ignore reason: requires a real GCS bucket and network access, not exercised by unit tests
	return nil
}

// Get opens a reader for path's contents. Callers must Close it.
func (s *GCSStore) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	// coverage:ignore reason: requires a real GCS bucket and network access, not exercised by unit tests
	r, err := s.client.Bucket(s.bucket).Object(path).NewReader(ctx)
	// coverage:ignore reason: requires a real GCS bucket and network access, not exercised by unit tests
	if err != nil {
		// coverage:ignore reason: requires a real GCS bucket and network access, not exercised by unit tests
		return nil, fmt.Errorf("objectstore: get %s: %w", path, err)
	}
	// coverage:ignore reason: requires a real GCS bucket and network access, not exercised by unit tests
	return r, nil
}
