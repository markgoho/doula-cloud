package pushsub_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/pushsub"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// env is what a test in this package starts from: a Practice, an employee
// Doula at the screen, and this package's whole surface mounted through
// pushsub.Mount, the same call main.go makes. A test that is about a Client
// builds it with fixture.WithClient(fixture.WithPortalLogin()).
type env struct {
	*fixture.Scene
}

func newEnv(t *testing.T, opts ...fixture.SceneOption) *env {
	t.Helper()
	return &env{Scene: fixture.NewScene(t, pushsub.Mount, opts...)}
}

func newClientEnv(t *testing.T) *env {
	t.Helper()
	return newEnv(t, fixture.WithClient(fixture.WithPortalLogin()))
}

// subscriptions is one of the two places a device registers: the Staff
// route under a Practice, or the Client route under an Engagement.
type subscriptions struct {
	name string
	url  string
	as   reqtest.Option
}

func (e *env) staffSubscriptions() subscriptions {
	return subscriptions{name: "staff", url: e.PracticeURL("/push-subscriptions"), as: e.SignedIn}
}

func (e *env) clientSubscriptions() subscriptions {
	return subscriptions{
		name: "client",
		url:  e.URL("/api/portal/engagements/" + e.Engagement.ID + "/push-subscriptions"),
		as:   e.AsClient,
	}
}

// bothSubscriptions is the table every test that holds for both populations
// ranges over.
func (e *env) bothSubscriptions() []subscriptions {
	return []subscriptions{e.staffSubscriptions(), e.clientSubscriptions()}
}

func (s subscriptions) register(t *testing.T, endpoint string) {
	t.Helper()
	reqtest.ExpectStatus(t, http.MethodPost, s.url, http.StatusNoContent, s.as, reqtest.JSON(subscribeRequest(endpoint)))
}

func (s subscriptions) unregister(t *testing.T, endpointQuery string, want int) {
	t.Helper()
	reqtest.ExpectStatus(t, http.MethodDelete, s.url+endpointQuery, want, s.as)
}

func subscribeRequest(endpoint string) pushsub.SubscribeRequest {
	req := pushsub.SubscribeRequest{Endpoint: endpoint}
	req.Keys.P256dh = "p256dh-key"
	req.Keys.Auth = "auth-key"
	return req
}

// countSubscriptions counts push_subscriptions rows for endpoint, via the
// superuser Admin connection (bypassing RLS -- used only to assert on
// ground truth, not to observe RLS in effect).
func countSubscriptions(t *testing.T, db *testdb.DB, endpoint string) int {
	t.Helper()
	var count int
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM push_subscriptions WHERE endpoint = $1`, endpoint).Scan(&count); err != nil {
		t.Fatalf("count push_subscriptions for %q: %v", endpoint, err)
	}
	return count
}
