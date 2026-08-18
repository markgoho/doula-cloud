package message_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/message"
	"doula-cloud/api/internal/push"
	"doula-cloud/api/internal/testdb"
)

// pushNotificationPayload mirrors message.pushPayload's JSON shape (that
// type is unexported, so tests in this external package redeclare the
// wire shape to decode against) -- the AC this proves is "a body-free
// payload: no sender name, no message text", so only these two fields
// should ever appear.
type pushNotificationPayload struct {
	EngagementID string `json:"engagementId"`
	PracticeID   string `json:"practiceId,omitempty"`
}

// TestCreateHandler_NotifiesClientPushSubscription proves #61's core AC:
// a Staff-authored Message triggers exactly one Pusher.Send call, to the
// Client's registered subscription, with a body-free payload (just the
// Engagement id -- no sender name, no message text, no practiceId, since
// the Client-portal route needs only the Engagement id).
func TestCreateHandler_NotifiesClientPushSubscription(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-notifies-client"
	practiceID := seedPractice(t, db, "Practice")
	seedStaffAtPractice(t, db, practiceID, identityUID)
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")
	seedPushSubscription(t, db, "client", clientID, "https://push.example.com/client-recipient")

	pusher := push.NewFakePusher()
	srv := newServerWithPusher(authntest.Verifier{UID: identityUID}, db, pusher)
	defer srv.Close()

	body, _ := json.Marshal(message.CreateRequest{Body: "Hi, checking in."})
	resp := authedPost(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/messages", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	calls := pusher.Calls()
	if len(calls) != 1 {
		t.Fatalf("Pusher.Send call count = %d, want 1: %+v", len(calls), calls)
	}
	if calls[0].Subscription.Endpoint != "https://push.example.com/client-recipient" {
		t.Fatalf("notified endpoint = %q, want the Client's own subscription", calls[0].Subscription.Endpoint)
	}
	var payload pushNotificationPayload
	if err := json.Unmarshal(calls[0].Payload, &payload); err != nil {
		t.Fatalf("decode push payload: %v", err)
	}
	if payload.EngagementID != engagementID || payload.PracticeID != "" {
		t.Fatalf("push payload = %+v, want {EngagementID: %q, PracticeID: \"\"}", payload, engagementID)
	}
	if raw := string(calls[0].Payload); strings.Contains(raw, "Hi, checking in") {
		t.Fatalf("push payload leaked message content: %s", raw)
	}
}

// TestClientCreateHandler_NotifiesStaffPushSubscriptions proves the
// mirror direction: a Client-authored Message notifies every Staff
// member's registered subscription at that Practice, with the Practice
// id included (the Staff-side thread route is Practice-scoped, unlike
// the Client-portal one).
func TestClientCreateHandler_NotifiesStaffPushSubscriptions(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-notifies-staff"
	practiceID := seedPractice(t, db, "Practice")
	staffAID := seedStaffAtPracticeNamed(t, db, practiceID, "staff-a-push", "Staff A")
	staffBID := seedStaffAtPracticeNamed(t, db, practiceID, "staff-b-push", "Staff B")
	seedPushSubscription(t, db, "staff", staffAID, "https://push.example.com/staff-a")
	seedPushSubscription(t, db, "staff", staffBID, "https://push.example.com/staff-b")
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")
	seedPortalUser(t, db, identityUID, clientID)

	pusher := push.NewFakePusher()
	srv := newPortalServerWithPusher(authntest.Verifier{UID: identityUID}, db, pusher)
	defer srv.Close()

	body, _ := json.Marshal(message.CreateRequest{Body: "Question about my visit."})
	resp := authedPost(t, srv.URL+"/portal/engagements/"+engagementID+"/messages", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	calls := pusher.Calls()
	if len(calls) != 2 {
		t.Fatalf("Pusher.Send call count = %d, want 2: %+v", len(calls), calls)
	}
	notified := map[string]bool{}
	for _, c := range calls {
		notified[c.Subscription.Endpoint] = true
		var payload pushNotificationPayload
		if err := json.Unmarshal(c.Payload, &payload); err != nil {
			t.Fatalf("decode push payload: %v", err)
		}
		if payload.EngagementID != engagementID || payload.PracticeID != practiceID {
			t.Fatalf("push payload = %+v, want engagement %q at practice %q", payload, engagementID, practiceID)
		}
	}
	if !notified["https://push.example.com/staff-a"] || !notified["https://push.example.com/staff-b"] {
		t.Fatalf("notified endpoints = %v, want both Staff A and Staff B", notified)
	}
}

// TestCreateHandler_NoSubscriptionsMeansNoPushCalls proves a Message can
// still be created when the recipient has never registered a push
// subscription -- push delivery is best-effort, not a precondition.
func TestCreateHandler_NoSubscriptionsMeansNoPushCalls(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-no-subs"
	practiceID := seedPractice(t, db, "Practice")
	seedStaffAtPractice(t, db, practiceID, identityUID)
	_, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")

	pusher := push.NewFakePusher()
	srv := newServerWithPusher(authntest.Verifier{UID: identityUID}, db, pusher)
	defer srv.Close()

	body, _ := json.Marshal(message.CreateRequest{Body: "no subscribers yet"})
	resp := authedPost(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/messages", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	if calls := pusher.Calls(); len(calls) != 0 {
		t.Fatalf("Pusher.Send call count = %d, want 0: %+v", len(calls), calls)
	}
}

// TestCreateHandler_PushFailureDoesNotBlockMessageCreation proves push
// delivery failure (e.g. the recipient's subscription expired at the push
// service) is non-fatal: the Message row is already written to tx by the
// time Pusher.Send runs, so a push failure is logged, not surfaced as a
// 500.
func TestCreateHandler_PushFailureDoesNotBlockMessageCreation(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-push-fails"
	practiceID := seedPractice(t, db, "Practice")
	seedStaffAtPractice(t, db, practiceID, identityUID)
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")
	seedPushSubscription(t, db, "client", clientID, "https://push.example.com/gone")

	pusher := push.NewFakePusher()
	pusher.Err = errors.New("simulated push service failure")
	srv := newServerWithPusher(authntest.Verifier{UID: identityUID}, db, pusher)
	defer srv.Close()

	body, _ := json.Marshal(message.CreateRequest{Body: "still gets created"})
	resp := authedPost(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/messages", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	if calls := pusher.Calls(); len(calls) != 1 {
		t.Fatalf("Pusher.Send call count = %d, want 1 (still attempted): %+v", len(calls), calls)
	}
}

// TestCreateHandler_DoesNotNotifyStaffSubscriptionsForClientRecipient
// proves push_subscriptions_for_message_recipient's population filter: a
// Staff-authored Message must notify only the Client's subscription, even
// though a Staff member at the same Practice also has one registered.
func TestCreateHandler_DoesNotNotifyStaffSubscriptionsForClientRecipient(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-population-filter"
	practiceID := seedPractice(t, db, "Practice")
	staffID := seedStaffAtPractice(t, db, practiceID, identityUID)
	seedPushSubscription(t, db, "staff", staffID, "https://push.example.com/staff-own")
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com")
	seedPushSubscription(t, db, "client", clientID, "https://push.example.com/client-only")

	pusher := push.NewFakePusher()
	srv := newServerWithPusher(authntest.Verifier{UID: identityUID}, db, pusher)
	defer srv.Close()

	body, _ := json.Marshal(message.CreateRequest{Body: "checking population filter"})
	resp := authedPost(t, srv.URL+"/practices/"+practiceID+"/engagements/"+engagementID+"/messages", body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	calls := pusher.Calls()
	if len(calls) != 1 || calls[0].Subscription.Endpoint != "https://push.example.com/client-only" {
		t.Fatalf("Pusher.Send calls = %+v, want exactly one call to the Client's subscription", calls)
	}
}
