package legalnotice_test

import (
	"os"
	"sort"
	"strings"
	"testing"

	"doula-cloud/api/internal/legal"
	"doula-cloud/api/internal/legalnotice"
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/mailsuppress"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/testdb"
)

// Each Owner of each Practice a material version queued an email for is
// sent one naming the version, its own page and the Practice it is about.
func TestWorker_MailsEachOwnerOfEachPracticeTheVersion(t *testing.T) {
	db := testdb.New(t)
	harbor := testdb.SeedPractice(t, db, "Harbor Doulas")
	ana := testdb.SeedStaffAtPractice(t, db, harbor, "ana", owner, employee)
	testdb.SeedStaffAtPractice(t, db, harbor, "bea", []string{"owner", "admin"}, employee)
	willow := testdb.SeedPractice(t, db, "Willow Birth")
	addOwner(t, db, willow, ana)

	if queued := testdb.SeedLegalPublication(t, db, "terms", "2026-12-01", true, publishedAt); queued != 3 {
		t.Fatalf("queued = %d, want 3", queued)
	}
	sender := &mail.FakeSender{}
	runWorker(t, db, sender, fixtureDocs)

	sent := sender.Sent()
	var to []string
	var harborText string
	for _, m := range sent {
		to = append(to, m.To)
		if m.Subject != "DoulaCloud: a new version of the Terms of Service starts on December 1, 2026" {
			t.Errorf("subject = %q", m.Subject)
		}
		if !strings.Contains(m.Text, "What changes: The price of a Credit changes to $40.") {
			t.Errorf("text does not carry the version's one line:\n%s", m.Text)
		}
		if !strings.Contains(m.Text, "\nhttps://doula.cloud/terms/2026-12-01\n") {
			t.Errorf("text does not link the new version's own page:\n%s", m.Text)
		}
		if strings.Contains(m.Text, "your practice, Harbor Doulas.") {
			harborText = m.Text
		}
	}
	sort.Strings(to)
	if want := []string{anaAddress, anaAddress, "bea@example.com"}; strings.Join(to, ",") != strings.Join(want, ",") {
		t.Fatalf("sent to %v, want %v", to, want)
	}
	if !strings.Contains(harborText, testAppBase+"/practices/"+harbor+"/settings/delete") {
		t.Fatalf("Harbor's email does not link Harbor's deletion screen:\n%s", harborText)
	}
	for _, r := range outboxRows(t, db) {
		if r.status != statusSent {
			t.Errorf("row for %s at %s is %s, want sent", r.staffID, r.practiceID, r.status)
		}
	}
}

// A price rise's email gives the reason, as the Terms promise.
func TestWorker_PriceRiseEmailGivesTheReason(t *testing.T) {
	db := testdb.New(t)
	harbor := testdb.SeedPractice(t, db, "Harbor Doulas")
	testdb.SeedStaffAtPractice(t, db, harbor, "ana", owner, employee)

	testdb.SeedLegalPublication(t, db, "terms", "2026-12-31", true, publishedAt)
	sender := &mail.FakeSender{}
	runWorker(t, db, sender, fixtureDocs)

	sent := sender.Sent()
	if len(sent) != 1 {
		t.Fatalf("sent %d emails, want 1", len(sent))
	}
	if !strings.Contains(sent[0].Text, "What changes: A Credit costs $50.\n\nWhy the price rises: Text messages cost more to send.\n\n") {
		t.Fatalf("text does not give the reason for the rise:\n%s", sent[0].Text)
	}
}

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

	testdb.SeedLegalPublication(t, db, "terms", "2026-12-01", true, publishedAt)
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

	testdb.SeedLegalPublication(t, db, "terms", "2026-12-01", true, publishedAt)
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

	testdb.SeedLegalPublication(t, db, "terms", "2026-12-01", true, publishedAt)
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
