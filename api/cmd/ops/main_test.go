package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/dbpool"
	"doula-cloud/api/internal/stackswitch"
)

const testTenant = "operators"

func fakeFactory(_ context.Context, _, _ string) (Verifier, error) { return fakeVerifier{}, nil }

func envOf(env map[string]string) func(string) string {
	return func(k string) string { return env[k] }
}

func TestNewServer_ListensOnPORTWithThePoolCappedAtTheOpsShare(t *testing.T) {
	env := map[string]string{"PORT": "20080", tenantVar: testTenant, databaseURLVar: "postgres://nobody@127.0.0.1:1/none?connect_timeout=1"}
	server, db, err := newServer(t.Context(), envOf(env), fakeFactory)
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
	server, db, err := newServer(t.Context(), envOf(map[string]string{tenantVar: testTenant}), fakeFactory)
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if server.Addr != ":8080" {
		t.Errorf("Addr = %q, want :8080", server.Addr)
	}
}

func TestNewServer_RefusesTheEmulatorHostOnCloudRun(t *testing.T) {
	env := map[string]string{tenantVar: testTenant, "K_SERVICE": "doula-ops", "FIREBASE_AUTH_EMULATOR_HOST": "127.0.0.1:9099"}

	_, _, err := newServer(t.Context(), envOf(env), fakeFactory)

	if !errors.Is(err, errEmulatorOnCloudRun) {
		t.Fatalf("err = %v, want %v", err, errEmulatorOnCloudRun)
	}
}

func TestNewServer_RefusesNoTenant(t *testing.T) {
	_, _, err := newServer(t.Context(), envOf(map[string]string{}), fakeFactory)

	if err == nil || !strings.Contains(err.Error(), tenantVar) {
		t.Fatalf("err = %v, want one naming OPS_TENANT_ID", err)
	}
}

func TestNewServer_HandsTheVerifierTheProjectAndTenant(t *testing.T) {
	var gotProject, gotTenant string
	refusal := errors.New("no credentials")
	factory := func(_ context.Context, project, tenant string) (Verifier, error) {
		gotProject, gotTenant = project, tenant
		return nil, refusal
	}

	_, _, err := newServer(t.Context(), envOf(map[string]string{tenantVar: " " + testTenant + " ", "GCP_PROJECT_ID": "doula-cloud"}), factory)

	if !errors.Is(err, refusal) {
		t.Fatalf("err = %v, want the factory's", err)
	}
	if gotProject != "doula-cloud" || gotTenant != "operators" {
		t.Errorf("factory got %q %q, want doula-cloud operators", gotProject, gotTenant)
	}
}

func TestNewServer_RefusesTheSimClockOutsideTheStack(t *testing.T) {
	env := map[string]string{tenantVar: testTenant, stackswitch.ClockVar: "1"}

	_, _, err := newServer(t.Context(), envOf(env), fakeFactory)

	if !errors.Is(err, stackswitch.ErrNotTheStack) {
		t.Fatalf("err = %v, want %v", err, stackswitch.ErrNotTheStack)
	}
}

func TestNewServer_AcceptsTheEmulatorFactorOnlyWithTheEmulatorHost(t *testing.T) {
	const unreachable = "postgres://nobody@127.0.0.1:1/none?connect_timeout=1"
	for _, tc := range []struct {
		env  map[string]string
		want int
	}{
		{map[string]string{tenantVar: testTenant, databaseURLVar: unreachable, "FIREBASE_AUTH_EMULATOR_HOST": "127.0.0.1:9099"}, http.StatusInternalServerError},
		{map[string]string{tenantVar: testTenant, databaseURLVar: unreachable}, http.StatusForbidden},
	} {
		factory := func(context.Context, string, string) (Verifier, error) { return tokens, nil }
		server, db, err := newServer(t.Context(), envOf(tc.env), factory)
		if err != nil {
			t.Fatalf("newServer: %v", err)
		}
		t.Cleanup(func() { _ = db.Close() })

		r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/session", nil)
		r.Header.Set("Authorization", "Bearer phone-token")
		w := httptest.NewRecorder()
		server.Handler.ServeHTTP(w, r)

		// With the emulator the factor passes and the unreachable
		// database is what answers; without it the factor is refused
		// before the database is asked.
		if w.Code != tc.want {
			t.Errorf("env %v: status = %d, want %d", tc.env, w.Code, tc.want)
		}
	}
}

func TestStripeClient_IsTheFakeOnlyOnTheStackWithNoKey(t *testing.T) {
	if _, ok := stripeClient("", true).(*billing.FakeStripeClient); !ok {
		t.Error("the emulator with no key: want the fake")
	}
	for _, c := range []struct {
		key      string
		emulator bool
	}{{"sk_test_x", true}, {"sk_test_x", false}, {"", false}} {
		if _, ok := stripeClient(c.key, c.emulator).(*billing.StripeAPIClient); !ok {
			t.Errorf("key %q, emulator %v: want the Stripe API client", c.key, c.emulator)
		}
	}
}

func TestOrigins_ReadsACommaSeparatedList(t *testing.T) {
	if got := origins(" http://a , ,http://b"); !slices.Equal(got, []string{"http://a", "http://b"}) {
		t.Errorf("origins = %v", got)
	}
	if got := origins(""); got != nil {
		t.Errorf("origins of unset = %v, want none", got)
	}
}

func TestIdentityOf_ReadsTheSecondFactorProvider(t *testing.T) {
	got := identityOf("uid", map[string]any{
		"email":    opEmail,
		"name":     opName,
		"firebase": map[string]any{"tenant": "operators", "sign_in_second_factor": factorTOTP},
	})
	want := Identity{UID: "uid", Email: opEmail, Name: opName, SecondFactor: "totp"}
	if got != want {
		t.Errorf("identityOf = %+v, want %+v", got, want)
	}
	if got := identityOf("uid", map[string]any{}); got != (Identity{UID: "uid"}) {
		t.Errorf("identityOf with no claims = %+v", got)
	}
}
