package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/dbpool"
)

func TestNewServer_ListensOnPORTWithThePoolCappedAtTheOpsShare(t *testing.T) {
	env := map[string]string{"PORT": "20080", "DATABASE_URL": "postgres://nobody@127.0.0.1:1/none?connect_timeout=1"}
	server, db, err := newServer(func(k string) string { return env[k] })
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if server.Addr != ":20080" {
		t.Errorf("Addr = %q, want :20080", server.Addr)
	}
	if got := db.Stats().MaxOpenConnections; got != dbpool.OpsPerInstance {
		t.Errorf("MaxOpenConnections = %d, want %d", got, dbpool.OpsPerInstance)
	}

	w := httptest.NewRecorder()
	server.Handler.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/health", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("health against an unreachable database = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestNewServer_DefaultsToCloudRunsPort(t *testing.T) {
	server, db, err := newServer(func(string) string { return "" })
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if server.Addr != ":8080" {
		t.Errorf("Addr = %q, want :8080", server.Addr)
	}
}
