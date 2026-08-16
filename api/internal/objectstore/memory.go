package objectstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
)

// ErrNotFound is returned by MemoryStore.Get for a path nothing was ever
// Put to.
var ErrNotFound = errors.New("objectstore: object not found")

// MemoryStore is an in-memory ObjectStore, injected into handler tests
// instead of a real GCS bucket, per #56's testing decisions.
type MemoryStore struct {
	mu      sync.Mutex
	objects map[string][]byte
}

// NewMemoryStore returns an empty MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{objects: map[string][]byte{}}
}

// Put reads r fully into memory and stores it under path, overwriting any
// existing object there.
func (m *MemoryStore) Put(_ context.Context, path, _ string, r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("objectstore: memory put %s: %w", path, err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[path] = data
	return nil
}

// Get returns a reader over path's stored bytes, or ErrNotFound.
func (m *MemoryStore) Get(_ context.Context, path string) (io.ReadCloser, error) {
	m.mu.Lock()
	data, ok := m.objects[path]
	m.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("objectstore: memory get %s: %w", path, ErrNotFound)
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}
