package pushsub_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/pushsub"
	"doula-cloud/api/internal/reqtest"
)

const endpointQuery = "?endpoint="

func TestRegister_StoresTheDevice(t *testing.T) {
	e := newClientEnv(t)
	for _, s := range e.bothSubscriptions() {
		t.Run(s.name, func(t *testing.T) {
			endpoint := "https://push.example.com/" + s.name + "-device"
			s.register(t, endpoint)
			if got := countSubscriptions(t, e.DB, endpoint); got != 1 {
				t.Fatalf("push_subscriptions rows for endpoint = %d, want 1", got)
			}
		})
	}
}

// TestRegisterHandler_ReregisterSameEndpointUpserts proves re-registering
// the same device (same endpoint, e.g. after the browser rotated its
// keys) updates the existing row in place rather than erroring on the
// endpoint UNIQUE constraint.
func TestRegisterHandler_ReregisterSameEndpointUpserts(t *testing.T) {
	e := newEnv(t)
	s := e.staffSubscriptions()

	const endpoint = "https://push.example.com/staff-rotating-keys"
	s.register(t, endpoint)
	s.register(t, endpoint)
	if got := countSubscriptions(t, e.DB, endpoint); got != 1 {
		t.Fatalf("push_subscriptions rows for endpoint = %d, want 1 (upserted, not duplicated)", got)
	}
}

func TestRegister_RefusesWhatItCannotRead(t *testing.T) {
	e := newClientEnv(t)
	for _, s := range e.bothSubscriptions() {
		t.Run(s.name+" body that is not JSON", func(t *testing.T) {
			reqtest.ExpectStatus(t, http.MethodPost, s.url, http.StatusBadRequest, s.as, reqtest.RawJSON([]byte("not json")))
		})
	}
	t.Run("staff body with no fields", func(t *testing.T) {
		reqtest.ExpectStatus(t, http.MethodPost, e.staffSubscriptions().url, http.StatusBadRequest, e.SignedIn, reqtest.JSON(pushsub.SubscribeRequest{}))
	})
}

func TestUnregister_RemovesTheDevice(t *testing.T) {
	e := newClientEnv(t)
	for _, s := range e.bothSubscriptions() {
		t.Run(s.name, func(t *testing.T) {
			endpoint := "https://push.example.com/" + s.name + "-leaving"
			s.register(t, endpoint)
			s.unregister(t, endpointQuery+endpoint, http.StatusNoContent)
			if got := countSubscriptions(t, e.DB, endpoint); got != 0 {
				t.Fatalf("push_subscriptions rows for endpoint = %d, want 0 after unregister", got)
			}
		})
	}
}

func TestUnregister_MissingEndpointRejected(t *testing.T) {
	e := newClientEnv(t)
	for _, s := range e.bothSubscriptions() {
		t.Run(s.name, func(t *testing.T) {
			s.unregister(t, "", http.StatusBadRequest)
		})
	}
}

// TestUnregisterHandler_CannotDeleteAnotherStaffMembersSubscription proves
// push_subscriptions' self-identity-only RLS (00008_messaging.sql)
// protects the unregister path too: Staff B's unregister request naming
// Staff A's endpoint deletes nothing.
func TestUnregisterHandler_CannotDeleteAnotherStaffMembersSubscription(t *testing.T) {
	e := newEnv(t)
	owner := e.staffSubscriptions()
	attacker := owner
	attacker.as = e.As(e.Colleague(fixture.Doula()))

	const endpoint = "https://push.example.com/staff-a-device"
	owner.register(t, endpoint)

	attacker.unregister(t, endpointQuery+endpoint, http.StatusNoContent) // a no-op delete is still a success response
	if got := countSubscriptions(t, e.DB, endpoint); got != 1 {
		t.Fatalf("push_subscriptions rows for endpoint = %d, want 1 (Staff A's row must survive)", got)
	}
}
