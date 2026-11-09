package oncall_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/oncall"
	"doula-cloud/api/internal/testdb"
)

const testAppBaseURL = "https://app.example.test"

// noticeEnv is a live birth whose window holds the next few hours, with
// one uncovered gap saved through the API. Its dates are relative to now,
// because the worker's recheck compares a gap's end with the clock.
type noticeEnv struct {
	*env
	Admin  fixture.StaffRecord
	Backup fixture.StaffRecord
	GapID  string
}

func newNoticeEnv(t *testing.T) noticeEnv {
	t.Helper()
	due := time.Now().AddDate(0, 0, 10).Format("2006-01-02")
	e := noticeEnv{env: newEnv(t, practiceNamed("Willow Birth Collective"), dueOn(due))}
	e.Admin = fixture.Staff(t, e.DB, e.Practice, fixture.Admin())
	e.Backup = e.backup()

	starts := time.Now().Add(time.Hour).Truncate(time.Second)
	ends := starts.Add(4 * time.Hour)
	e.GapID = e.recordGap(e.SignedIn, oncall.GapRequest{StaffID: e.Doula.ID, StartsAt: &starts, EndsAt: &ends}).ID
	return e
}

// runWorker runs one pass of the worker, sending through sender.
func (e noticeEnv) runWorker(sender *mail.FakeSender) {
	e.T.Helper()
	e.runWorkerWith(oncall.GapNoticeWorker{Sender: sender, Now: time.Now, AppBaseURL: testAppBaseURL, From: "from@example.test", ReplyTo: "reply@example.test"})
}

func (e noticeEnv) noticeStatus() string {
	e.T.Helper()
	return testdb.Scalar[string](e.T, e.DB.Admin, `SELECT status FROM coverage_gap_outbox WHERE gap_id = $1`, e.GapID)
}

func TestGapNoticeWorker_MailsEveryOwnerAndAdminAndNamesNothing(t *testing.T) {
	e := newNoticeEnv(t)
	sender := &mail.FakeSender{}

	e.runWorker(sender)

	var to []string
	for _, msg := range sender.Sent() {
		to = append(to, msg.To)
		for _, identifying := range []string{"Willow", "Ada", "Whitfield", "Maya", backupName} {
			if strings.Contains(msg.Subject+msg.Text+msg.From, identifying) {
				t.Errorf("the notice carries %q; a Platform Notification is content-free", identifying)
			}
		}
		if !strings.Contains(msg.Text, testAppBaseURL+"/practices/"+e.Practice.ID+"/on-call") {
			t.Errorf("the notice does not link to the roster: %s", msg.Text)
		}
	}
	slices.Sort(to)
	want := []string{emailOf(e.Admin), emailOf(e.Staff)}
	slices.Sort(want)
	if !slices.Equal(to, want) {
		t.Fatalf("mailed %v, want every Owner and Admin %v and no Doula", to, want)
	}
	if got := e.noticeStatus(); got != statusSent {
		t.Fatalf("status = %q, want sent", got)
	}
	if n := e.count(`SELECT count(*) FROM coverage_gap_outbox WHERE gap_id = $1 AND cardinality(notified_staff_ids) = 2`, e.GapID); n != 1 {
		t.Fatal("the row does not record the two people it addressed")
	}
}

func TestGapNoticeWorker_SendsNothingForAGapNoLongerOpen(t *testing.T) {
	tests := []struct {
		name  string
		fixup string
	}{
		{"covered", `UPDATE engagement_coverage_gaps SET covering_staff_id = (SELECT staff_id FROM engagement_attachments WHERE engagement_id = engagement_coverage_gaps.engagement_id AND staff_id <> engagement_coverage_gaps.staff_id LIMIT 1)`},
		{"cleared", `UPDATE engagement_coverage_gaps SET cleared_at = now(), cleared_by = staff_id`},
		{"ended", `UPDATE engagement_coverage_gaps SET starts_at = now() - interval '5 hours', ends_at = now() - interval '1 hour'`},
	}
	for _, tt := range tests {
		t.Run("a gap that is "+tt.name, func(t *testing.T) {
			e := newNoticeEnv(t)
			e.Exec(tt.fixup)
			sender := &mail.FakeSender{}

			e.runWorker(sender)

			if len(sender.Sent()) != 0 {
				t.Fatalf("mailed %d messages for a gap that was %s", len(sender.Sent()), tt.name)
			}
			if got := e.noticeStatus(); got != statusSent {
				t.Fatalf("status = %q, want sent with nothing mailed", got)
			}
		})
	}
}
