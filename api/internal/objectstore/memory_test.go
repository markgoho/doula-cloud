package objectstore_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"doula-cloud/api/internal/objectstore"
)

func TestMemoryStore_PutGetRoundTrip(t *testing.T) {
	store := objectstore.NewMemoryStore()

	if err := store.Put(t.Context(), "path/one", "image/png", bytes.NewReader([]byte("bytes"))); err != nil {
		t.Fatalf("put: %v", err)
	}

	r, err := store.Get(t.Context(), "path/one")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer r.Close()

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "bytes" {
		t.Fatalf("data = %q, want %q", data, "bytes")
	}
}

func TestMemoryStore_GetMissingReturnsErrNotFound(t *testing.T) {
	store := objectstore.NewMemoryStore()

	_, err := store.Get(t.Context(), "does/not/exist")
	if !errors.Is(err, objectstore.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

type errReader struct{}

func (errReader) Read(_ []byte) (int, error) {
	return 0, errors.New("boom")
}

func TestMemoryStore_PutReadErrorPropagates(t *testing.T) {
	store := objectstore.NewMemoryStore()

	err := store.Put(t.Context(), "path/two", "image/png", errReader{})
	if err == nil {
		t.Fatal("want error, got nil")
	}
}
