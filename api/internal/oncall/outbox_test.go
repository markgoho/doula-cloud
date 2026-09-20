package oncall_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/oncall"
	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/testdb"
)

const testAppBaseURL = "https://app.example.test"

// noticeFixture is a live birth whose window holds the next few hours,
// with one uncovered gap saved through the API. Its dates are relative
// to now, because the worker's recheck compares a gap's end with the
// clock.
type noticeFixture struct {
	practiceID, ownerUID, doulaID, backupID, engagementID, gapID string
}

func newNoticeFixture(t *testing.T, db *testdb.DB, prefix string) noticeFixture {
	t.Helper()
	f := noticeFixture{ownerUID: prefix + "-owner"}
	f.practiceID, _ = testdb.SeedStaffAtNewPractice(t, db, f.ownerUID, []string{ownerRole}, employeeType)
	exec(t, db, `UPDATE practices SET name = 'Willow Birth Collective' WHERE id = $1`, f.practiceID)
	testdb.SeedStaffAtPractice(t, db, f.practiceID, prefix+"-admin", []string{adminRole}, employeeType)
	f.doulaID = testdb.SeedNamedStaffAtPractice(t, db, f.practiceID, prefix+"-doula", "Maya Primary", []string{doulaRole}, employeeType)
	f.backupID = testdb.SeedNamedStaffAtPractice(t, db, f.practiceID, prefix+"-backup", "Bo Backup", []string{doulaRole}, employeeType)

	due := time.Now().AddDate(0, 0, 10).Format("2006-01-02")
	f.engagementID = seedBirth(t, db, f.practiceID, "Ada Whitfield", due)
	testdb.SeedGrantedAttachment(t, db, f.engagementID, f.doulaID)
	testdb.SeedGrantedAttachment(t, db, f.engagementID, f.backupID)

	starts := time.Now().Add(time.Hour).Truncate(time.Second)
	ends := starts.Add(4 * time.Hour)
	srv, session := newServer(t, db, f.ownerUID)
	defer srv.Close()
	f.gapID = decode[oncall.Gap](t, authedBody(t, session, http.MethodPost, gapsURL(srv.URL, f.practiceID, f.engagementID),
		oncall.GapRequest{StaffID: f.doulaID, StartsAt: &starts, EndsAt: &ends}), http.StatusCreated).ID
	return f
}

// runWorker sets the trusted door outbox's own handler would set, runs
// one pass and commits.
func runWorker(t *testing.T, db *testdb.DB, sender *mail.FakeSender) {
	t.Helper()
	w := oncall.GapNoticeWorker{Mailer: outbox.Mailer{Sender: sender, Now: time.Now, AppBaseURL: testAppBaseURL, From: "from@example.test", ReplyTo: "reply@example.test"}}
	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.notification_worker_trusted', 'true', true)`); err != nil {
		t.Fatalf("set trusted door: %v", err)
	}
	if err := w.ProcessPending(t.Context(), tx); err != nil {
		t.Fatalf("ProcessPending: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

func noticeStatus(t *testing.T, db *testdb.DB, gapID string) string {
	t.Helper()
	var status string
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT status FROM coverage_gap_outbox WHERE gap_id = $1`, gapID).Scan(&status); err != nil {
		t.Fatalf("read notice: %v", err)
	}
	return status
}

func TestGapNoticeWorker_MailsEveryOwnerAndAdminAndNamesNothing(t *testing.T) {
	db := testdb.New(t)
	f := newNoticeFixture(t, db, "notice-mail")
	sender := &mail.FakeSender{}
	runWorker(t, db, sender)

	var to []string
	for _, msg := range sender.Sent() {
		to = append(to, msg.To)
		for _, identifying := range []string{"Willow", "Ada", "Whitfield", "Maya", "Bo Backup"} {
			if strings.Contains(msg.Subject+msg.Text+msg.From, identifying) {
				t.Errorf("the notice carries %q; a Platform Notification is content-free", identifying)
			}
		}
		if !strings.Contains(msg.Text, testAppBaseURL+"/practices/"+f.practiceID+"/on-call") {
			t.Errorf("the notice does not link to the roster: %s", msg.Text)
		}
	}
	slices.Sort(to)
	want := []string{"notice-mail-admin@example.com", "notice-mail-owner@example.com"}
	if !slices.Equal(to, want) {
		t.Fatalf("mailed %v, want every Owner and Admin %v and no Doula", to, want)
	}
	if got := noticeStatus(t, db, f.gapID); got != "sent" {
		t.Fatalf("status = %q, want sent", got)
	}
	if n := countRows(t, db, `SELECT count(*) FROM coverage_gap_outbox WHERE gap_id = $1 AND cardinality(notified_staff_ids) = 2`, f.gapID); n != 1 {
		t.Fatal("the row does not record the two people it addressed")
	}
}

func TestGapNoticeWorker_SendsNothingForAGapNoLongerOpen(t *testing.T) {
	for name, fixup := range map[string]string{
		"covered": `UPDATE engagement_coverage_gaps SET covering_staff_id = (SELECT staff_id FROM engagement_attachments WHERE engagement_id = engagement_coverage_gaps.engagement_id AND staff_id <> engagement_coverage_gaps.staff_id LIMIT 1)`,
		"cleared": `UPDATE engagement_coverage_gaps SET cleared_at = now(), cleared_by = staff_id`,
		"ended":   `UPDATE engagement_coverage_gaps SET starts_at = now() - interval '5 hours', ends_at = now() - interval '1 hour'`,
	} {
		t.Run(name, func(t *testing.T) {
			db := testdb.New(t)
			f := newNoticeFixture(t, db, "notice-"+name)
			exec(t, db, fixup)

			sender := &mail.FakeSender{}
			runWorker(t, db, sender)
			if len(sender.Sent()) != 0 {
				t.Fatalf("mailed %d messages for a gap that was %s", len(sender.Sent()), name)
			}
			if got := noticeStatus(t, db, f.gapID); got != "sent" {
				t.Fatalf("status = %q, want sent with nothing mailed", got)
			}
		})
	}
}
