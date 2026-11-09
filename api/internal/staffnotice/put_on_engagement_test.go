package staffnotice_test

import (
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/staffnotice"
	"doula-cloud/api/internal/testdb"
)

type putOnFixture struct {
	fixture
	employeeID   string
	engagementID string
}

const (
	employeeUID = "uid-employee"
	statusSent  = "sent"
)

func newPutOnFixture(t *testing.T) putOnFixture {
	t.Helper()
	f := newFixture(t)
	employeeID := testdb.SeedStaffAtPractice(t, f.db, f.practiceID, employeeUID, []string{"doula"}, "employee")
	_, engagementID := testdb.SeedNamedEngagement(t, f.db, f.practiceID, "Adaeze Nwosu", "adaeze@example.com")
	if _, err := f.db.Admin.ExecContext(t.Context(),
		`INSERT INTO engagement_attachments (engagement_id, staff_id, origin, attached_by)
		 VALUES ($1, $2, 'granted', $3)`,
		engagementID, employeeID, f.ownerID,
	); err != nil {
		t.Fatalf("seed attachment: %v", err)
	}
	return putOnFixture{fixture: f, employeeID: employeeID, engagementID: engagementID}
}

func (f putOnFixture) putOnNotice() staffnotice.Notice {
	return staffnotice.Notice{
		PracticeID:       f.practiceID,
		RecipientStaffID: f.employeeID,
		Kind:             staffnotice.KindPutOnEngagement,
		SubjectID:        f.engagementID,
		TriggeredBy:      f.ownerID,
	}
}

func (f putOnFixture) readPutOn(t *testing.T) (string, *string) {
	t.Helper()
	var status string
	var address *string
	if err := f.db.Admin.QueryRowContext(t.Context(),
		`SELECT status::text, sent_to_address FROM staff_notice_outbox WHERE subject_id = $1`,
		f.engagementID,
	).Scan(&status, &address); err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	return status, address
}

var putOnNow = time.Date(2030, time.March, 1, 12, 0, 0, 0, time.UTC)

func TestWorker_MailsHerTheEngagementSomeoneElsePutHerOn(t *testing.T) {
	f := newPutOnFixture(t)
	f.queue(t, f.putOnNotice())

	sent := requireMails(t, f.run(t, putOnNow), 1)
	msg := sent[0]
	if msg.To != employeeUID+"@example.com" {
		t.Fatalf("to = %q", msg.To)
	}
	if msg.Subject != "You've been put on a client on DoulaCloud" {
		t.Fatalf("subject = %q", msg.Subject)
	}
	link := appBaseURL + "/practices/" + f.practiceID + "/engagements/" + f.engagementID
	if !strings.Contains(msg.Text, "\n"+link+"\n") {
		t.Fatalf("text = %q, want the link %q on its own line", msg.Text, link)
	}
	for _, named := range []string{"Adaeze", "Nwosu", "Test Practice", "uid-owner"} {
		if strings.Contains(msg.Subject+msg.Text, named) {
			t.Fatalf("mail names %q: %q / %q", named, msg.Subject, msg.Text)
		}
	}
	if status, address := f.readPutOn(t); status != statusSent || address == nil || *address != employeeUID+"@example.com" {
		t.Fatalf("row = (%s, %v), want sent to her", status, address)
	}
}

func (f putOnFixture) leavePractice(t *testing.T) {
	t.Helper()
	if _, err := f.db.Admin.ExecContext(t.Context(),
		`DELETE FROM practice_memberships WHERE staff_id = $1`, f.employeeID); err != nil {
		t.Fatalf("remove membership: %v", err)
	}
}

func TestWorker_MailsNothingOnceSheIsNoLongerOnIt(t *testing.T) {
	cases := map[string]struct {
		notice func(putOnFixture) staffnotice.Notice
		change func(putOnFixture, *testing.T)
	}{
		"put on, and her Attachment ended":     {putOnFixture.putOnNotice, putOnFixture.endAttachment},
		"put on, and she left the Practice":    {putOnFixture.putOnNotice, putOnFixture.leavePractice},
		"taken off, and she is on it again":    {putOnFixture.takenOffNotice, func(putOnFixture, *testing.T) {}},
		"taken off, and she left the Practice": {putOnFixture.takenOffNotice, putOnFixture.leavePractice},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newPutOnFixture(t)
			f.queue(t, tc.notice(f))
			tc.change(f, t)

			requireMails(t, f.run(t, putOnNow), 0)
			if status, address := f.readPutOn(t); status != statusSent || address != nil {
				t.Fatalf("row = (%s, %v), want sent with no address", status, address)
			}
		})
	}
}
