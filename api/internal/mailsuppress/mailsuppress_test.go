package mailsuppress_test

import (
	"errors"
	"os"
	"testing"

	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/mailsuppress"
	"doula-cloud/api/internal/testdb"
)

// TestMain terminates the shared Postgres container testdb.New starts for
// this test process -- see testdb.Main's doc comment.
func TestMain(m *testing.M) {
	os.Exit(testdb.Main(m))
}

const testAddress = "someone@example.test"

func testMessage(to string) mail.Message {
	return mail.Message{To: to, From: "notifications@example.test", Subject: "s", Text: "t"}
}

func TestActive_UnknownAddressIsNotSuppressed(t *testing.T) {
	db := testdb.New(t)

	got, err := mailsuppress.Active(t.Context(), db.App, testAddress)
	if err != nil {
		t.Fatalf("Active: %v", err)
	}
	if got {
		t.Fatal("an address nobody ever complained about reads as suppressed")
	}
}

func TestRecordThenActive(t *testing.T) {
	db := testdb.New(t)

	if err := mailsuppress.Record(t.Context(), db.App, testAddress, mailsuppress.CauseComplaint, "evt-1"); err != nil {
		t.Fatalf("Record: %v", err)
	}
	got, err := mailsuppress.Active(t.Context(), db.App, testAddress)
	if err != nil {
		t.Fatalf("Active: %v", err)
	}
	if !got {
		t.Fatal("a recorded complaint does not read as suppressed")
	}
}

// Mailgun reports the recipient as the sender wrote it, so a Client whose
// address is stored with capitals must still be recognised.
func TestSuppressionIsCaseAndSpaceInsensitive(t *testing.T) {
	db := testdb.New(t)

	if err := mailsuppress.Record(t.Context(), db.App, "  Someone@Example.Test ", mailsuppress.CauseBounce, "evt-1"); err != nil {
		t.Fatalf("Record: %v", err)
	}
	got, err := mailsuppress.Active(t.Context(), db.App, "SOMEONE@EXAMPLE.TEST")
	if err != nil {
		t.Fatalf("Active: %v", err)
	}
	if !got {
		t.Fatal("a differently-cased spelling of a suppressed address reads as sendable")
	}
}

// A second event for an address already suppressed is not an error --
// Mailgun sends one per failed attempt, and each replaces the last.
func TestRecordTwiceReplacesTheCause(t *testing.T) {
	db := testdb.New(t)

	if err := mailsuppress.Record(t.Context(), db.App, testAddress, mailsuppress.CauseBounce, "evt-1"); err != nil {
		t.Fatalf("first Record: %v", err)
	}
	if err := mailsuppress.Record(t.Context(), db.App, testAddress, mailsuppress.CauseComplaint, "evt-2"); err != nil {
		t.Fatalf("second Record: %v", err)
	}

	var cause, eventID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT cause, mailgun_event_id FROM email_suppressions WHERE address = $1`, testAddress,
	).Scan(&cause, &eventID); err != nil {
		t.Fatalf("read suppression: %v", err)
	}
	if cause != mailsuppress.CauseComplaint {
		t.Fatalf("cause = %q, want %q (the newer event wins)", cause, mailsuppress.CauseComplaint)
	}
	if eventID != "evt-2" {
		t.Fatalf("mailgun_event_id = %q, want evt-2", eventID)
	}
}

// A Staff member clearing a bounce must actually make the address
// sendable again, and a later bounce must re-arm it.
func TestClearedSuppressionIsInactiveUntilRecordedAgain(t *testing.T) {
	db := testdb.New(t)

	if err := mailsuppress.Record(t.Context(), db.App, testAddress, mailsuppress.CauseBounce, "evt-1"); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE email_suppressions SET cleared_at = now() WHERE address = $1`, testAddress,
	); err != nil {
		t.Fatalf("clear suppression: %v", err)
	}

	got, err := mailsuppress.Active(t.Context(), db.App, testAddress)
	if err != nil {
		t.Fatalf("Active after clear: %v", err)
	}
	if got {
		t.Fatal("a cleared suppression still refuses the address")
	}

	if err := mailsuppress.Record(t.Context(), db.App, testAddress, mailsuppress.CauseComplaint, "evt-2"); err != nil {
		t.Fatalf("re-Record: %v", err)
	}
	got, err = mailsuppress.Active(t.Context(), db.App, testAddress)
	if err != nil {
		t.Fatalf("Active after re-record: %v", err)
	}
	if !got {
		t.Fatal("a new complaint after a clear does not re-suppress the address")
	}
}

func TestSender_DeliversAnUnsuppressedAddress(t *testing.T) {
	db := testdb.New(t)
	inner := &mail.FakeSender{}
	s := mailsuppress.Sender{Inner: inner, DB: db.App}

	if err := s.Send(t.Context(), testMessage(testAddress)); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(inner.Sent()) != 1 {
		t.Fatalf("delivered %d messages, want 1", len(inner.Sent()))
	}
}

func TestSender_RefusesASuppressedAddressWithoutCallingMailgun(t *testing.T) {
	db := testdb.New(t)
	if err := mailsuppress.Record(t.Context(), db.App, testAddress, mailsuppress.CauseComplaint, "evt-1"); err != nil {
		t.Fatalf("Record: %v", err)
	}
	inner := &mail.FakeSender{}
	s := mailsuppress.Sender{Inner: inner, DB: db.App}

	err := s.Send(t.Context(), testMessage(testAddress))
	if !errors.Is(err, mail.ErrSuppressed) {
		t.Fatalf("Send error = %v, want mail.ErrSuppressed", err)
	}
	if len(inner.Sent()) != 0 {
		t.Fatalf("handed %d messages to Mailgun, want 0", len(inner.Sent()))
	}
}

// A real Mailgun failure keeps its own identity, so ADR-0010's backoff
// still applies to it rather than the suppression dead-letter.
func TestSender_PassesThroughAnOrdinarySendFailure(t *testing.T) {
	db := testdb.New(t)
	inner := &mail.FakeSender{Err: errBoom}
	s := mailsuppress.Sender{Inner: inner, DB: db.App}

	err := s.Send(t.Context(), testMessage(testAddress))
	if !errors.Is(err, errBoom) {
		t.Fatalf("Send error = %v, want errBoom", err)
	}
	if errors.Is(err, mail.ErrSuppressed) {
		t.Fatal("an ordinary send failure reads as a suppression")
	}
}

var errBoom = errors.New("mailgun is down")

var _ mail.Sender = mailsuppress.Sender{}
