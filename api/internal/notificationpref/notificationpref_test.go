package notificationpref_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/notificationpref"
	"doula-cloud/api/internal/testdb"
)

// newPortalServer mounts the same notification-preference routes main.go
// wires up for the Client-portal population, behind clientauth.Middleware.
func newPortalServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("GET /portal/engagements/{engagementId}/notification-preference",
		clientauth.Middleware(db.App)(notificationpref.GetHandler()))
	mux.Handle("PUT /portal/engagements/{engagementId}/notification-preference",
		clientauth.Middleware(db.App)(notificationpref.SetHandler()))
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

func authedRequest(t *testing.T, session, method, url string, body []byte) *http.Response {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(t.Context(), method, url, reader)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func decodePreference(t *testing.T, resp *http.Response) notificationpref.PreferenceResponse {
	t.Helper()
	defer resp.Body.Close()
	var out notificationpref.PreferenceResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode preference response: %v", err)
	}
	return out
}

// TestGetHandler_NeverDecidedReportsDisabled proves #303 AC1's gate: a
// Client who has never visited the notification settings screen reads
// enabled=false, so the client-side register helper never fires the
// browser's permission prompt before she has seen the explanation.
func TestGetHandler_NeverDecidedReportsDisabled(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-never-decided"
	practiceID := testdb.SeedPractice(t, db, "Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID)
	seedPortalUser(t, db, identityUID, clientID)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := authedRequest(t, session, http.MethodGet, srv.URL+"/portal/engagements/"+engagementID+"/notification-preference", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := decodePreference(t, resp); got.Enabled {
		t.Fatalf("Enabled = %v, want false (never decided)", got.Enabled)
	}
}

// TestSetHandler_TurnsOnRecordsChoiceAndActivity proves #303 AC3/AC7:
// turning push on durably persists the choice and records who did it, as
// her own act.
func TestSetHandler_TurnsOnRecordsChoiceAndActivity(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-turns-on"
	practiceID := testdb.SeedPractice(t, db, "Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID)
	seedPortalUser(t, db, identityUID, clientID)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	body, _ := json.Marshal(notificationpref.SetRequest{Enabled: true})
	resp := authedRequest(t, session, http.MethodPut, srv.URL+"/portal/engagements/"+engagementID+"/notification-preference", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := decodePreference(t, resp); !got.Enabled {
		t.Fatalf("Enabled = %v, want true", got.Enabled)
	}

	if muted, found := readPreferenceRow(t, db, identityUID, engagementID); !found || muted {
		t.Fatalf("notification_preferences row = (muted=%v, found=%v), want (false, true)", muted, found)
	}

	action, actorKind := latestActivityAction(t, db, engagementID)
	if action != "push_notifications_enabled" || actorKind != "client" {
		t.Fatalf("latest activity = (%q, %q), want (push_notifications_enabled, client)", action, actorKind)
	}
}

// TestSetHandler_TurnsOffAfterOnRecordsChoiceAndActivity proves the round
// trip #303 AC3 asks for: on, then off, both durable and both recorded.
func TestSetHandler_TurnsOffAfterOnRecordsChoiceAndActivity(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-turns-off"
	practiceID := testdb.SeedPractice(t, db, "Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID)
	seedPortalUser(t, db, identityUID, clientID)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	onBody, _ := json.Marshal(notificationpref.SetRequest{Enabled: true})
	onResp := authedRequest(t, session, http.MethodPut, srv.URL+"/portal/engagements/"+engagementID+"/notification-preference", onBody)
	_ = onResp.Body.Close()

	offBody, _ := json.Marshal(notificationpref.SetRequest{Enabled: false})
	offResp := authedRequest(t, session, http.MethodPut, srv.URL+"/portal/engagements/"+engagementID+"/notification-preference", offBody)
	if offResp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", offResp.StatusCode, http.StatusOK)
	}
	if got := decodePreference(t, offResp); got.Enabled {
		t.Fatalf("Enabled = %v, want false", got.Enabled)
	}

	if muted, found := readPreferenceRow(t, db, identityUID, engagementID); !found || !muted {
		t.Fatalf("notification_preferences row = (muted=%v, found=%v), want (true, true)", muted, found)
	}

	getResp := authedRequest(t, session, http.MethodGet, srv.URL+"/portal/engagements/"+engagementID+"/notification-preference", nil)
	defer getResp.Body.Close()
	if got := decodePreference(t, getResp); got.Enabled {
		t.Fatalf("GET after turning off: Enabled = %v, want false", got.Enabled)
	}

	action, actorKind := latestActivityAction(t, db, engagementID)
	if action != "push_notifications_disabled" || actorKind != "client" {
		t.Fatalf("latest activity = (%q, %q), want (push_notifications_disabled, client)", action, actorKind)
	}
}

// TestSetHandler_InvalidJSONBody proves malformed input is refused rather
// than silently defaulted.
func TestSetHandler_InvalidJSONBody(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-bad-json"
	practiceID := testdb.SeedPractice(t, db, "Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID)
	seedPortalUser(t, db, identityUID, clientID)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := authedRequest(t, session, http.MethodPut, srv.URL+"/portal/engagements/"+engagementID+"/notification-preference", []byte("not json"))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestSetHandler_CannotWriteAnotherClientsEngagementPreference proves the
// enforcement story behind this route's exemption from
// staffauth.AttachingWrite in ../../write_gate_guardrail_test.go: it is a
// clientauth.Middleware-scoped Client write, not a Staff write on the
// Engagement, so the boundary that has to hold is "a Client can never
// write another Client's preference" rather than ADR-0008's attachment
// gate. clientauth.Middleware refuses at the URL before SetHandler ever
// runs -- setClientAndCheckEngagement (clientauth/middleware.go) checks
// engagement ownership against app.current_client_id -- so this is really
// a clientauth.Middleware test reached through this route; the RLS policy
// on notification_preferences (00067) is the layer underneath if that
// middleware check were ever bypassed.
func TestSetHandler_CannotWriteAnotherClientsEngagementPreference(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Practice")
	const identityA = "client-a-owns-the-engagement"
	clientA, engagementA := seedClientEngagement(t, db, practiceID)
	seedPortalUser(t, db, identityA, clientA)

	const identityB = "client-b-attacker"
	clientB, _ := seedClientEngagement(t, db, practiceID)
	seedPortalUser(t, db, identityB, clientB)

	srvB, sessionB := newPortalServer(t, db, identityB)
	defer srvB.Close()

	body, _ := json.Marshal(notificationpref.SetRequest{Enabled: false})
	resp := authedRequest(t, sessionB, http.MethodPut, srvB.URL+"/portal/engagements/"+engagementA+"/notification-preference", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d (Client B does not own Engagement A)", resp.StatusCode, http.StatusForbidden)
	}

	if _, found := readPreferenceRow(t, db, identityA, engagementA); found {
		t.Fatalf("notification_preferences row for Client A's Engagement was written by Client B's request")
	}
}

// TestSetHandler_MutingOneEngagementLeavesSiblingEngagementUnaffected
// proves #303's own AC: the preference carries an Engagement reference, so
// muting one of a Client's Engagements never silences another. The AC's
// own example is two Engagements at two different Practices, which is not
// reachable through this handler alone -- client_portal_users.identity_uid
// stays UNIQUE (00006_client_portal_users.sql), so one identity_uid cannot
// yet resolve to a second Client record at a second Practice
// (#309: "A person who already has a portal account cannot accept a
// second invite"). This proves the same Engagement-scoping mechanism the
// two-Practice case would exercise, using two Engagements on one Client at
// one Practice instead.
func TestSetHandler_MutingOneEngagementLeavesSiblingEngagementUnaffected(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-two-engagements"
	practiceID := testdb.SeedPractice(t, db, "Practice")
	clientID, mutedEngagementID := seedClientEngagement(t, db, practiceID)
	otherEngagementID := seedEngagement(t, db, practiceID, clientID)
	seedPortalUser(t, db, identityUID, clientID)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	// Both start on, an explicit choice on each -- otherwise "still off
	// after muting one" would be indistinguishable from "never decided".
	onBody, _ := json.Marshal(notificationpref.SetRequest{Enabled: true})
	for _, id := range []string{mutedEngagementID, otherEngagementID} {
		resp := authedRequest(t, session, http.MethodPut, srv.URL+"/portal/engagements/"+id+"/notification-preference", onBody)
		_ = resp.Body.Close()
	}

	offBody, _ := json.Marshal(notificationpref.SetRequest{Enabled: false})
	offResp := authedRequest(t, session, http.MethodPut, srv.URL+"/portal/engagements/"+mutedEngagementID+"/notification-preference", offBody)
	_ = offResp.Body.Close()

	otherResp := authedRequest(t, session, http.MethodGet, srv.URL+"/portal/engagements/"+otherEngagementID+"/notification-preference", nil)
	defer otherResp.Body.Close()
	if got := decodePreference(t, otherResp); !got.Enabled {
		t.Fatalf("sibling Engagement's Enabled = %v, want true (explicitly on, unaffected by the other Engagement's mute)", got.Enabled)
	}
}
