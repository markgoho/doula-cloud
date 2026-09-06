package clientauth_test

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/testdb"
)

func newAddressChangeWorker(sender mail.Sender) clientauth.AddressChangeWorker {
	return clientauth.NewAddressChangeWorker(outbox.Mailer{Sender: sender, Now: time.Now, AppBaseURL: testAppBaseURL, From: testSenderAddr, ReplyTo: testReplyTo})
}

func seedAddressChangeRow(t *testing.T, db *testdb.DB, identityUID, toAddress string, attemptCount int, nextAttemptAt time.Time) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO portal_address_change_outbox (identity_uid, to_address, token, attempt_count, next_attempt_at)
		 VALUES ($1, $2, 'confirm-token', $3, $4) RETURNING id`,
		identityUID, toAddress, attemptCount, nextAttemptAt,
	).Scan(&id); err != nil {
		t.Fatalf("seed address change row: %v", err)
	}
	return id
}

func addressChangeRowState(t *testing.T, db *testdb.DB, id string) (status string, attemptCount int, token sql.NullString) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status, attempt_count, token FROM portal_address_change_outbox WHERE id = $1`, id,
	).Scan(&status, &attemptCount, &token); err != nil {
		t.Fatalf("query address change row: %v", err)
	}
	return status, attemptCount, token
}

// TestAddressChangeWorker_ProcessPending_MailsTheRowsOwnAddress is the
// difference from MagicLinkWorker worth a test of its own: the recipient
// is the address on the row, never the one portal_accounts holds -- which
// here is still the old one.
func TestAddressChangeWorker_ProcessPending_MailsTheRowsOwnAddress(t *testing.T) {
	db := testdb.New(t)
	const identifier = "portal_change-send"
	testdb.SeedPortalAccount(t, db, identifier, "still-the-old@example.com")
	rowID := seedAddressChangeRow(t, db, identifier, "the-new-one@example.com", 0, time.Now().Add(-time.Minute))

	sender := &mail.FakeSender{}
	runMagicLinkTx(t, db, newAddressChangeWorker(sender).ProcessPending)

	status, _, token := addressChangeRowState(t, db, rowID)
	if status != statusSent {
		t.Fatalf("status = %q, want sent", status)
	}
	if token.Valid {
		t.Fatal("token still set once sent, want NULL")
	}
	sent := sender.Sent()
	if len(sent) != 1 || sent[0].To != "the-new-one@example.com" {
		t.Fatalf("sent = %+v, want one message to the-new-one@example.com", sent)
	}
	// ADR-0009: content-free. The mail carries the link and nothing that
	// would tell an unintended reader whose account this is.
	if !strings.Contains(sent[0].Text, testAppBaseURL+"/portal/confirm-sign-in-address?token=confirm-token") {
		t.Fatalf("mail body = %q, want the confirmation link", sent[0].Text)
	}
	if strings.Contains(sent[0].Text, "still-the-old@example.com") {
		t.Fatal("the confirmation mail names the old address")
	}
}
