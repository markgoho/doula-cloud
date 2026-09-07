package objectstore

import (
	"errors"
	"testing"

	"cloud.google.com/go/storage"
)

// TestWrapGetError_NotExistMapsToErrNotFound proves GCSStore.Get's error
// mapping without a real GCS bucket: fed storage.ErrObjectNotExist
// directly, the same sentinel the GCS client library returns for a
// missing object, rather than one earned over the network.
func TestWrapGetError_NotExistMapsToErrNotFound(t *testing.T) {
	err := wrapGetError("some/path", storage.ErrObjectNotExist)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

// TestWrapGetError_OtherErrorsPassThrough proves a non-not-found failure
// (a genuine storage-layer problem) is neither swallowed nor misreported
// as ErrNotFound.
func TestWrapGetError_OtherErrorsPassThrough(t *testing.T) {
	cause := errors.New("simulated network failure")
	err := wrapGetError("some/path", cause)
	if errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want not ErrNotFound", err)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("err = %v, want it to wrap %v", err, cause)
	}
}
