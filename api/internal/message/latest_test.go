package message_test

import (
	"testing"

	"doula-cloud/api/internal/message"
	"doula-cloud/api/internal/testdb"
)

// TestClientLatest reads the Message "Your care" quotes (#2007), on a
// Client-portal transaction: nothing before anybody writes, then the
// newest Message with the sender's name the thread gives her.
func TestClientLatest(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Riverside Doulas")
	clientID, engagementID := testdb.SeedEngagement(t, db, practiceID)
	doulaID := testdb.SeedNamedStaffAtPractice(t, db, practiceID, "latest-doula", "Priya Raman", []string{"doula"}, "employee")

	read := func(t *testing.T) (message.Message, bool) {
		t.Helper()
		tx, err := db.App.BeginTx(t.Context(), nil)
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		defer func() { _ = tx.Rollback() }()
		if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_client_id', $1, true)`, clientID); err != nil {
			t.Fatalf("set_config: %v", err)
		}
		latest, found, err := message.ClientLatest(t.Context(), tx, engagementID)
		if err != nil {
			t.Fatalf("latest: %v", err)
		}
		return latest, found
	}

	if _, found := read(t); found {
		t.Fatalf("found a Message on an empty thread")
	}

	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO messages (engagement_id, sender_type, sender_id, body, created_at)
		 VALUES ($1, 'client', $2, 'An older note.', now() - interval '2 days'),
		        ($1, 'staff', $3, 'Thursday works. See you at 2.', now() - interval '1 hour')`,
		engagementID, clientID, doulaID); err != nil {
		t.Fatalf("seed messages: %v", err)
	}
	latest, found := read(t)
	if !found || latest.Body != "Thursday works. See you at 2." {
		t.Fatalf("latest = %+v, found = %v, want the newest Message", latest, found)
	}
	if latest.SenderName != "Priya Raman" {
		t.Fatalf("senderName = %q, want %q", latest.SenderName, "Priya Raman")
	}
}
