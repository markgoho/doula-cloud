package testdb

import (
	"database/sql"
	"testing"

	"doula-cloud/api/internal/mail"
)

// RequireSentOnceTo proves a worker pass mailed exactly one message, to
// the address, and returns it for the assertions on its body.
func RequireSentOnceTo(t *testing.T, sender *mail.FakeSender, to string) mail.Message {
	t.Helper()
	sent := sender.Sent()
	if len(sent) != 1 || sent[0].To != to {
		// coverage:ignore reason: the failure branch ends the calling test by design; exercising it would need a testing.T substitute this repo has no precedent for
		t.Fatalf("sent = %+v, want one message to %s", sent, to)
	}
	return sent[0]
}

// RequireSentAndTokenCleared proves an outbox row a worker pass mailed is
// marked sent and no longer holds the one-time token it carried, given the
// status and token the test read back from the row, and that its one
// message went to the address.
func RequireSentAndTokenCleared(t *testing.T, status string, token sql.NullString, sender *mail.FakeSender, to string) mail.Message {
	t.Helper()
	if status != "sent" {
		// coverage:ignore reason: the failure branch ends the calling test by design; exercising it would need a testing.T substitute this repo has no precedent for
		t.Fatalf("status = %q, want sent", status)
	}
	if token.Valid {
		// coverage:ignore reason: the failure branch ends the calling test by design; exercising it would need a testing.T substitute this repo has no precedent for
		t.Fatal("token still set once sent, want NULL")
	}
	return RequireSentOnceTo(t, sender, to)
}
