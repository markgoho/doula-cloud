package message_test

import (
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/message"
	"doula-cloud/api/internal/testdb"
)

// TestClientLatest reads the Message "Your care" quotes (#2007), on a
// Client-portal transaction: nothing before anybody writes, then the
// newest Message with the sender's name the thread gives her.
func TestClientLatest(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db, fixture.PracticeName("Riverside Doulas"))
	engagement := fixture.Engagement(t, db, p)
	doula := fixture.Staff(t, db, p, fixture.StaffName("Priya Raman"))

	read := func(t *testing.T) (message.Message, bool) {
		t.Helper()
		tx := db.BeginAs(t, testdb.Client(engagement.Client.ID))
		defer func() { _ = tx.Rollback() }()
		latest, found, err := message.ClientLatest(t.Context(), tx, engagement.ID)
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
		engagement.ID, engagement.Client.ID, doula.ID); err != nil {
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
