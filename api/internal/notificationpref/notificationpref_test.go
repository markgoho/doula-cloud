package notificationpref_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/notificationpref"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// TestGetHandler_NeverDecidedReportsDisabled proves #303 AC1's gate: a
// Client who has never visited the notification settings screen reads
// enabled=false, so the client-side register helper never fires the
// browser's permission prompt before she has seen the explanation.
func TestGetHandler_NeverDecidedReportsDisabled(t *testing.T) {
	e := newEnv(t)

	if e.enabled(e.Engagement.ID) {
		t.Fatalf("Enabled = true, want false (never decided)")
	}
}

// TestSetHandler_TurnsOnRecordsChoiceAndActivity proves #303 AC3/AC7:
// turning push on durably persists the choice and records who did it, as
// her own act.
func TestSetHandler_TurnsOnRecordsChoiceAndActivity(t *testing.T) {
	e := newEnv(t)

	if !e.set(e.Engagement.ID, true) {
		t.Fatalf("Enabled = false, want true")
	}

	if muted, found := readPreferenceRow(t, e.DB, e.Engagement.Client.PortalUID, e.Engagement.ID); !found || muted {
		t.Fatalf("notification_preferences row = (muted=%v, found=%v), want (false, true)", muted, found)
	}

	action, actorKind := latestActivityAction(t, e.DB, e.Engagement.ID)
	if action != "push_notifications_enabled" || actorKind != "client" {
		t.Fatalf("latest activity = (%q, %q), want (push_notifications_enabled, client)", action, actorKind)
	}
}

// TestSetHandler_TurnsOffAfterOnRecordsChoiceAndActivity proves the round
// trip #303 AC3 asks for: on, then off, both durable and both recorded.
func TestSetHandler_TurnsOffAfterOnRecordsChoiceAndActivity(t *testing.T) {
	e := newEnv(t)

	e.set(e.Engagement.ID, true)
	if e.set(e.Engagement.ID, false) {
		t.Fatalf("Enabled = true, want false")
	}

	if muted, found := readPreferenceRow(t, e.DB, e.Engagement.Client.PortalUID, e.Engagement.ID); !found || !muted {
		t.Fatalf("notification_preferences row = (muted=%v, found=%v), want (true, true)", muted, found)
	}
	if e.enabled(e.Engagement.ID) {
		t.Fatalf("GET after turning off: Enabled = true, want false")
	}

	action, actorKind := latestActivityAction(t, e.DB, e.Engagement.ID)
	if action != "push_notifications_disabled" || actorKind != "client" {
		t.Fatalf("latest activity = (%q, %q), want (push_notifications_disabled, client)", action, actorKind)
	}
}

// TestSetHandler_InvalidJSONBody proves malformed input is refused rather
// than silently defaulted.
func TestSetHandler_InvalidJSONBody(t *testing.T) {
	e := newEnv(t)

	reqtest.ExpectStatus(t, http.MethodPut, e.preferenceURL(e.Engagement.ID), http.StatusBadRequest, e.AsClient, reqtest.RawJSON([]byte("not json")))
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
	e := newEnv(t)
	attacker := e.AnotherEngagement(fixture.NewClient(fixture.WithPortalLogin())).Client

	reqtest.ExpectStatus(t, http.MethodPut, e.preferenceURL(e.Engagement.ID), http.StatusForbidden, e.As(attacker),
		reqtest.JSON(notificationpref.SetRequest{Enabled: false}))

	if _, found := readPreferenceRow(t, e.DB, e.Engagement.Client.PortalUID, e.Engagement.ID); found {
		t.Fatalf("notification_preferences row for Client A's Engagement was written by Client B's request")
	}
}

// TestSetHandler_MutingOneEngagementLeavesSiblingEngagementUnaffected
// proves #303's own AC: the preference carries an Engagement reference, so
// muting one of a Client's Engagements never silences another. Two
// Engagements on one Client at one Practice, rather than the AC's own
// two-Practice example: this handler is scoped to one Engagement at a
// time either way, so the same mechanism proves both shapes. The genuine
// two-Practice case -- reachable now that #309 lifted
// client_portal_users.identity_uid's UNIQUE constraint -- is
// TestPushSubscriptionsForMessageRecipient_CrossPracticePortalAccount
// below.
func TestSetHandler_MutingOneEngagementLeavesSiblingEngagementUnaffected(t *testing.T) {
	e := newEnv(t)
	muted := e.Engagement.ID
	other := e.AnotherEngagement(fixture.ForClient(e.Engagement.Client)).ID

	// Both start on, an explicit choice on each -- otherwise "still off
	// after muting one" would be indistinguishable from "never decided".
	e.set(muted, true)
	e.set(other, true)
	e.set(muted, false)

	if !e.enabled(other) {
		t.Fatalf("sibling Engagement's Enabled = false, want true (explicitly on, unaffected by the other Engagement's mute)")
	}
}

// TestPushSubscriptionsForMessageRecipient_CrossPracticePortalAccount is
// #309's own hand-off from this file's sibling-Engagement test above: with
// client_portal_users.identity_uid no longer UNIQUE (#309), one Portal
// Account can reach a Client at two different Practices (ADR-0015), and
// this proves the genuine two-Practice case that test could only stand in
// for -- muting one Practice's Engagement never silences the Portal
// Account's push subscription at the other. No schema change needed:
// push_subscriptions_for_message_recipient (00067) already scopes its
// mute check to one engagement_id.
func TestPushSubscriptionsForMessageRecipient_CrossPracticePortalAccount(t *testing.T) {
	e := newEnv(t)
	engagementA := e.Engagement
	otherPractice := fixture.Practice(t, e.DB)
	engagementB := fixture.Engagement(t, e.DB, otherPractice)
	// The same Portal Account reaching a second Practice: a second
	// client_portal_users row, not a second Portal Account.
	testdb.AttachPortalUser(t, e.DB, engagementA.Client.PortalUID, engagementB.Client.ID)

	const seed = `INSERT INTO push_subscriptions (owner_type, owner_id, endpoint, p256dh_key, auth_key) VALUES ('client', $1, $2, 'p256dh-key', 'auth-key')`
	e.Exec(seed, engagementA.Client.ID, "https://push.example.com/a")
	e.Exec(seed, engagementB.Client.ID, "https://push.example.com/b")

	e.set(engagementA.ID, false)

	var endpointB string
	if err := e.DB.App.QueryRowContext(t.Context(),
		`SELECT endpoint FROM push_subscriptions_for_message_recipient($1, 'client')`, engagementB.ID,
	).Scan(&endpointB); err != nil {
		t.Fatalf("query recipient subscriptions for Practice B's Engagement: %v", err)
	}
	if endpointB != "https://push.example.com/b" {
		t.Fatalf("endpoint = %q, want Practice B's own subscription, unaffected by muting Practice A's Engagement", endpointB)
	}

	var countA int
	if err := e.DB.App.QueryRowContext(t.Context(),
		`SELECT count(*) FROM push_subscriptions_for_message_recipient($1, 'client')`, engagementA.ID,
	).Scan(&countA); err != nil {
		t.Fatalf("query recipient subscriptions for Practice A's Engagement: %v", err)
	}
	if countA != 0 {
		t.Fatalf("Practice A's Engagement returned %d subscriptions, want 0 (muted)", countA)
	}
}
