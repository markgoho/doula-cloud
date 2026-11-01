package legalnotice_test

import (
	"os"
	"strings"
	"testing"

	"doula-cloud/api/internal/legal"
	"doula-cloud/api/internal/legalnotice"
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/mailsuppress"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/testdb"
)

// A suppressed address (ADR-0029) is recorded as not delivered, on its
// own row, and the other Owner of the same Practice is still told: "was
// she told?" has a true answer for each.
func TestWorker_SuppressedAddressIsRecordedAsNotDelivered(t *testing.T) {
	db := testdb.New(t)
	harbor := testdb.SeedPractice(t, db, "Harbor Doulas")
	ana := testdb.SeedStaffAtPractice(t, db, harbor, "ana", owner, employee)
	bea := testdb.SeedStaffAtPractice(t, db, harbor, "bea", owner, employee)
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO email_suppressions (address, cause) VALUES ('bea@example.com', 'complaint')`); err != nil {
		t.Fatalf("seed suppression: %v", err)
	}

	publishOK(t, newPublishServer(t, db), "terms", "2026-12-01")
	inner := &mail.FakeSender{}
	runWorker(t, db, mailsuppress.Sender{Inner: inner, DB: db.App}, fixtureDocs)

	if got := inner.Sent(); len(got) != 1 || got[0].To != anaAddress {
		t.Fatalf("sent = %+v, want one email, to Ana", got)
	}
	if status, _ := statusOf(t, db, harbor, ana); status != statusSent {
		t.Fatalf("Ana's row is %s, want sent", status)
	}
	status, lastError := statusOf(t, db, harbor, bea)
	if status != statusDead || !strings.Contains(lastError, "suppressed") {
		t.Fatalf("Bea's row is %s (%q), want dead_lettered with the suppression as the reason", status, lastError)
	}
	if got := reasonOf(t, db, harbor, bea); got != string(outbox.OwnData) {
		t.Fatalf("Bea's dead_letter_reason = %q, want own_data", got)
	}
}

// An Owner who deletes her login after the email was queued is not
// mailed at the sentinel address, and her row says why.
func TestWorker_DeletedLoginIsNotMailed(t *testing.T) {
	db := testdb.New(t)
	harbor := testdb.SeedPractice(t, db, "Harbor Doulas")
	ana := testdb.SeedStaffAtPractice(t, db, harbor, "ana", owner, employee)

	publishOK(t, newPublishServer(t, db), "terms", "2026-12-01")
	testdb.RedactDeletedLogin(t, db, ana)
	sender := &mail.FakeSender{}
	runWorker(t, db, sender, fixtureDocs)

	if n := len(sender.Sent()); n != 0 {
		t.Fatalf("sent %d emails, want none", n)
	}
	status, lastError := statusOf(t, db, harbor, ana)
	if status != statusDead || !strings.Contains(lastError, "deleted") {
		t.Fatalf("row is %s (%q), want dead_lettered because the login was deleted", status, lastError)
	}
	if got := reasonOf(t, db, harbor, ana); got != string(outbox.OwnData) {
		t.Fatalf("dead_letter_reason = %q, want own_data", got)
	}
}

// A row whose version the worker's history does not hold cannot be
// written, and is dead-lettered rather than mailed with an invented line.
func TestWorker_VersionMissingFromTheHistoryIsNotMailed(t *testing.T) {
	db := testdb.New(t)
	harbor := testdb.SeedPractice(t, db, "Harbor Doulas")
	ana := testdb.SeedStaffAtPractice(t, db, harbor, "ana", owner, employee)

	publishOK(t, newPublishServer(t, db), "terms", "2026-12-01")
	sender := &mail.FakeSender{}
	runWorker(t, db, sender, []legal.Document{fixturePrivacy})

	if n := len(sender.Sent()); n != 0 {
		t.Fatalf("sent %d emails, want none", n)
	}
	if status, _ := statusOf(t, db, harbor, ana); status != statusDead {
		t.Fatalf("row is %s, want dead_lettered", status)
	}
	if got := reasonOf(t, db, harbor, ana); got != string(outbox.OwnData) {
		t.Fatalf("dead_letter_reason = %q, want own_data", got)
	}
}

// TestCopy_MatchesTheCopyDocument holds docs/copy/legal-change-email.md
// and the email the worker sends together: the subject and the body,
// with each placeholder in the document standing where the worker puts
// a value.
func TestCopy_MatchesTheCopyDocument(t *testing.T) {
	raw, err := os.ReadFile("../../../docs/copy/legal-change-email.md")
	if err != nil {
		t.Fatalf("read copy document: %v", err)
	}
	doc := string(raw)

	values := legalnotice.Values{
		Document: "{Document}", Date: "{Date}", Practice: "{Practice}", Change: "{Change}", Reason: "{Reason}",
		DocumentURL: "{DocumentURL}", SettingsURL: "{SettingsURL}", DeleteURL: "{DeleteURL}",
	}
	subject, text := legalnotice.Compose(values)

	if !strings.Contains(doc, "**Subject:** "+subject+"\n") {
		t.Errorf("the copy document's subject is not %q", subject)
	}

	_, block, found := strings.Cut(doc, "```text\n")
	want, _, closed := strings.Cut(block, "```\n")
	if !found || !closed {
		t.Fatal("the copy document has no ```text block holding the body")
	}
	if text != want {
		t.Errorf("email body and copy document differ.\nemail:\n%s\ncopy document:\n%s", text, want)
	}

	// A version that does not raise the price of a Credit has no reason,
	// and its email leaves the line out, blank line and all.
	const reasonLine = "Why the price rises: {Reason}\n\n"
	values.Reason = ""
	if _, plain := legalnotice.Compose(values); plain != strings.Replace(want, reasonLine, "", 1) || !strings.Contains(want, reasonLine) {
		t.Errorf("without a reason, the email is not the copy document with the reason line left out:\n%s", plain)
	}
}
