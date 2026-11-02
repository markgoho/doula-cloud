package staffnotice_test

import (
	"strings"
	"testing"

	"doula-cloud/api/internal/staffnotice"
)

const (
	adaezeFirstName  = "Adaeze"
	adaezeFamilyName = "Nwosu"
	practiceNamed    = "Test Practice"
	ownerNamed       = "uid-owner"
)

func (f putOnFixture) takenOffNotice() staffnotice.Notice {
	n := f.putOnNotice()
	n.Kind = staffnotice.KindTakenOffEngagement
	return n
}

func (f putOnFixture) endAttachment(t *testing.T) {
	t.Helper()
	if _, err := f.db.Admin.ExecContext(t.Context(),
		`UPDATE engagement_attachments SET ended_at = now(), ended_by = $2 WHERE engagement_id = $1`,
		f.engagementID, f.ownerID); err != nil {
		t.Fatalf("end attachment: %v", err)
	}
}

func TestWorker_MailsHerTheEngagementSomeoneTookHerOff(t *testing.T) {
	f := newPutOnFixture(t)
	f.endAttachment(t)
	f.queue(t, f.takenOffNotice())

	sent := f.run(t, putOnNow).Sent()
	if len(sent) != 1 {
		t.Fatalf("sent %d messages, want 1", len(sent))
	}
	msg := sent[0]
	if msg.To != employeeUID+"@example.com" || msg.Subject != "You've been taken off a client on DoulaCloud" {
		t.Fatalf("to = %q subject = %q", msg.To, msg.Subject)
	}
	link := appBaseURL + "/practices/" + f.practiceID + "/engagements/" + f.engagementID
	if !strings.Contains(msg.Text, "\n"+link+"\n") || !strings.Contains(msg.Text, "opens that client's page") {
		t.Fatalf("text = %q, want the link and the employee's wording", msg.Text)
	}
	if strings.Contains(msg.Text, "Your offers") {
		t.Fatalf("an employee's mail speaks of Your offers: %q", msg.Text)
	}
	for _, named := range []string{adaezeFirstName, adaezeFamilyName, practiceNamed, ownerNamed} {
		if strings.Contains(msg.Subject+msg.Text, named) {
			t.Fatalf("mail names %q: %q", named, msg.Text)
		}
	}
}

func TestWorker_AContractorTakenOffIsToldWhereHerFeeIs(t *testing.T) {
	f := newPutOnFixture(t)
	if _, err := f.db.Admin.ExecContext(t.Context(),
		`UPDATE practice_memberships SET employment_type = 'contractor' WHERE staff_id = $1`, f.employeeID); err != nil {
		t.Fatalf("make contractor: %v", err)
	}
	f.endAttachment(t)
	f.queue(t, f.takenOffNotice())

	sent := f.run(t, putOnNow).Sent()
	if len(sent) != 1 {
		t.Fatalf("sent %d messages, want 1", len(sent))
	}
	text := sent[0].Text
	if !strings.Contains(text, "This link shows the day your work ended:") ||
		!strings.Contains(text, "Your fee, the terms and what you were paid stay on Your offers.") ||
		strings.Contains(text, "client's page") {
		t.Fatalf("text = %q, want the contractor's wording", text)
	}
}

func TestWorker_MailsNothingOnceSheIsBackOnItOrGone(t *testing.T) {
	cases := map[string]func(f putOnFixture) (string, []any){
		"she is on it again": func(_ putOnFixture) (string, []any) {
			return `SELECT 1`, nil
		},
		"she has left the Practice": func(f putOnFixture) (string, []any) {
			return `DELETE FROM practice_memberships WHERE staff_id = $1`, []any{f.employeeID}
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			f := newPutOnFixture(t)
			f.queue(t, f.takenOffNotice())
			query, args := change(f)
			if _, err := f.db.Admin.ExecContext(t.Context(), query, args...); err != nil {
				t.Fatalf("change: %v", err)
			}
			if sent := f.run(t, putOnNow).Sent(); len(sent) != 0 {
				t.Fatalf("sent %d messages, want none", len(sent))
			}
		})
	}
}
