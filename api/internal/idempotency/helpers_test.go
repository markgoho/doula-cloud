package idempotency_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/objectstore"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// doulaRole is named once so golangci-lint's goconst check doesn't see
// repeated "doula" literals across this package's test surface.
const doulaRole = "doula"

// pngBytes is a minimal valid 1x1 PNG, enough for http.DetectContentType
// to recognize it as image/png -- mirrors message_test's constant of the
// same name, kept as its own copy since that one is unexported.
var pngBytes = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89, 0x00, 0x00, 0x00,
	0x0a, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49,
	0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
}

// countingPutStore wraps a MemoryStore and counts Put calls, so a
// multipart create-Message retry test can prove idempotency.Wrap's replay
// skips the handler entirely -- no second object upload, not just no
// second messages row.
type countingPutStore struct {
	*objectstore.MemoryStore
	puts atomic.Int32
}

func newCountingPutStore() *countingPutStore {
	return &countingPutStore{MemoryStore: objectstore.NewMemoryStore()}
}

func (s *countingPutStore) Put(ctx context.Context, path, contentType string, r io.Reader) error {
	s.puts.Add(1)
	return s.MemoryStore.Put(ctx, path, contentType, r) //nolint:wrapcheck // test double, callers expect the underlying store's raw error
}

// countingHandler returns an http.Handler that increments a counter on
// every invocation and writes a JSON body reporting the call number, at
// the given status. Used to prove whether Wrap actually re-ran the
// wrapped handler or replayed a stored response instead.
func countingHandler(calls *int, status int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		*calls++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"n":` + strconv.Itoa(*calls) + `}`))
	})
}

// newIdempotencyServer wires countingHandler behind
// staffauth.Middleware(...)(idempotency.Wrap(...)), the same composition
// main.go uses for portal-invite.
func newIdempotencyServer(t *testing.T, db *testdb.DB, uid string, calls *int, status int) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("POST /practices/{practiceId}/widgets",
		staffauth.Middleware(db.App)(idempotency.Wrap(countingHandler(calls, status))))
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

func postWidget(t *testing.T, srv *httptest.Server, session, practiceID, idempotencyKey string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/practices/"+practiceID+"/widgets", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}
