package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"

	"doula-cloud/api/internal/dbpool"
	"doula-cloud/api/internal/testdb"
)

func TestMain(m *testing.M) {
	os.Exit(testdb.Main(m))
}

// wantRoutes is every route doula-ops serves, written out rather than
// derived, so a route cannot be added unseen (ADR-0062).
var wantRoutes = []string{
	"GET /api/health",
	"POST /api/session",
	"GET /api/session",
	"DELETE /api/session",
	"POST /api/session/step-up",
}

func TestRoutes_AreTheOnesOnTheRecord(t *testing.T) {
	_, got := routes(Deps{})
	slices.Sort(got)
	want := slices.Sorted(slices.Values(wantRoutes))
	if !slices.Equal(got, want) {
		t.Errorf("doula-ops routes:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestHealth_AnswersOKConnectedAsOpsRuntime(t *testing.T) {
	db := testdb.New(t)
	dbpool.ApplyOps(db.Ops)
	handler, _ := routes(Deps{DB: db.Ops})

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/health", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body %s", w.Code, http.StatusOK, w.Body)
	}
	if got := strings.TrimSpace(w.Body.String()); got != `{"status":"ok"}` {
		t.Fatalf("body = %s, want {\"status\":\"ok\"}", got)
	}
}

func TestHealth_AnswersUnavailableWhenTheDatabaseIsUnreachable(t *testing.T) {
	db, err := sql.Open("pgx", "postgres://nobody@127.0.0.1:1/none?connect_timeout=1")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	handler, _ := routes(Deps{DB: db})

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/health", nil))

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestHealth_RefusesAnotherMethod(t *testing.T) {
	handler, _ := routes(Deps{})

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/health", nil))

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}
