package push_test

import (
	"database/sql"
	"errors"
	"os"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/push"
	"doula-cloud/api/internal/testdb"
)

func TestMain(m *testing.M) {
	os.Exit(testdb.Main(m))
}

type payload struct {
	EngagementID string `json:"engagementId"`
}

// clientWithSubscription seeds an Engagement whose Client has one push
// subscription, and opens the transaction a notification is sent inside.
func clientWithSubscription(t *testing.T) (db *testdb.DB, engagement fixture.EngagementRecord, tx *sql.Tx) {
	t.Helper()
	db = testdb.New(t)
	p := fixture.Practice(t, db, fixture.PracticeName("Practice"))
	engagement = fixture.Engagement(t, db, p, fixture.NewClient(fixture.ClientName("Client"), fixture.ClientEmail("client@example.com")))
	testdb.SeedPushSubscription(t, db, "client", engagement.Client.ID, "https://push.example.com/client")
	return db, engagement, db.BeginAdmin(t)
}

func TestNotifyMessageRecipient_SendsToTheRecipientsSubscriptionsOnly(t *testing.T) {
	db, engagement, tx := clientWithSubscription(t)
	staff := fixture.Staff(t, db, fixture.PracticeRecord{ID: engagement.PracticeID})
	testdb.SeedPushSubscription(t, db, "staff", staff.ID, "https://push.example.com/staff")

	pusher := push.NewFakePusher()
	push.NotifyMessageRecipient(t.Context(), tx, pusher, "test", engagement.ID, "client", payload{EngagementID: engagement.ID})

	calls := pusher.Calls()
	if len(calls) != 1 || calls[0].Subscription.Endpoint != "https://push.example.com/client" {
		t.Fatalf("calls = %+v, want one send to the client's subscription", calls)
	}
	if want := `{"engagementId":"` + engagement.ID + `"}`; string(calls[0].Payload) != want {
		t.Fatalf("payload = %s, want %s", calls[0].Payload, want)
	}
}

func TestNotifyMessageRecipient_ASendFailureIsSwallowed(t *testing.T) {
	_, engagement, tx := clientWithSubscription(t)

	pusher := push.NewFakePusher()
	pusher.Err = errors.New("push service down")
	push.NotifyMessageRecipient(t.Context(), tx, pusher, "test", engagement.ID, "client", payload{EngagementID: engagement.ID})

	if got := len(pusher.Calls()); got != 1 {
		t.Fatalf("send attempts = %d, want 1", got)
	}
}
