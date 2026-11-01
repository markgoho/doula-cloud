package staffnotice_test

import (
	"strings"
	"testing"

	"doula-cloud/api/internal/staffnotice"
)

type visitFixture struct {
	putOnFixture
	visitID string
}

// The Visit is the employee's, moved to her off the contractor Doula.
func newVisitFixture(t *testing.T) visitFixture {
	t.Helper()
	f := newPutOnFixture(t)
	var visitID string
	if err := f.db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO visits (engagement_id, staff_id) VALUES ($1, $2) RETURNING id`,
		f.engagementID, f.employeeID,
	).Scan(&visitID); err != nil {
		t.Fatalf("seed visit: %v", err)
	}
	return visitFixture{putOnFixture: f, visitID: visitID}
}

func (f visitFixture) notice(recipient string, kind staffnotice.Kind) staffnotice.Notice {
	return staffnotice.Notice{
		PracticeID:       f.practiceID,
		RecipientStaffID: recipient,
		Kind:             kind,
		SubjectID:        f.visitID,
		TriggeredBy:      f.ownerID,
	}
}

func (f visitFixture) moveTo(t *testing.T, staffID string) {
	t.Helper()
	if _, err := f.db.Admin.ExecContext(t.Context(), `UPDATE visits SET staff_id = $2 WHERE id = $1`, f.visitID, staffID); err != nil {
		t.Fatalf("move visit: %v", err)
	}
}

// #2436: the Doula a Visit moved onto, and the one it moved off, each get
// an email that links to the Engagement and names no Client, colleague
// or Practice.
func TestWorker_MailsBothEndsOfAMovedVisit(t *testing.T) {
	f := newVisitFixture(t)
	f.queue(t, f.notice(f.employeeID, staffnotice.KindVisitReassignedTo))
	f.queue(t, f.notice(f.doulaID, staffnotice.KindVisitReassignedFrom))

	sent := f.run(t, putOnNow).Sent()
	if len(sent) != 2 {
		t.Fatalf("sent %d messages, want 2", len(sent))
	}
	link := appBaseURL + "/practices/" + f.practiceID + "/engagements/" + f.engagementID
	want := map[string]string{
		employeeUID + "@example.com": "You've been put on a visit on DoulaCloud",
		doulaAddress:                 "You've been taken off a visit on DoulaCloud",
	}
	for _, msg := range sent {
		if msg.Subject != want[msg.To] {
			t.Fatalf("mail to %s has subject %q, want %q", msg.To, msg.Subject, want[msg.To])
		}
		if !strings.Contains(msg.Text, "\n"+link+"\n") {
			t.Fatalf("text = %q, want the link %q on its own line", msg.Text, link)
		}
		for _, named := range []string{"Adaeze", "Nwosu", "Test Practice", "uid-owner", employeeUID, doulaUID} {
			if strings.Contains(msg.Subject+msg.Text, named) {
				t.Fatalf("mail names %q: %q / %q", named, msg.Subject, msg.Text)
			}
		}
	}
}

// Checked again at send time: a Visit moved on again, or deleted, or a
// recipient who has left the Practice, is marked sent with nothing
// mailed.
func TestWorker_MailsNothingOnceTheMoveIsNoLongerTrue(t *testing.T) {
	cases := map[string]struct {
		recipient func(f visitFixture) string
		kind      staffnotice.Kind
		change    func(t *testing.T, f visitFixture)
	}{
		"the Visit moved off her again": {
			recipient: func(f visitFixture) string { return f.employeeID },
			kind:      staffnotice.KindVisitReassignedTo,
			change:    func(t *testing.T, f visitFixture) { f.moveTo(t, f.doulaID) },
		},
		"the Visit moved back onto her": {
			recipient: func(f visitFixture) string { return f.doulaID },
			kind:      staffnotice.KindVisitReassignedFrom,
			change:    func(t *testing.T, f visitFixture) { f.moveTo(t, f.doulaID) },
		},
		"the Visit is gone": {
			recipient: func(f visitFixture) string { return f.employeeID },
			kind:      staffnotice.KindVisitReassignedTo,
			change: func(t *testing.T, f visitFixture) {
				if _, err := f.db.Admin.ExecContext(t.Context(), `DELETE FROM visits WHERE id = $1`, f.visitID); err != nil {
					t.Fatalf("delete visit: %v", err)
				}
			},
		},
		"the recipient is no longer at the Practice": {
			recipient: func(f visitFixture) string { return f.employeeID },
			kind:      staffnotice.KindVisitReassignedTo,
			change: func(t *testing.T, f visitFixture) {
				if _, err := f.db.Admin.ExecContext(t.Context(), `DELETE FROM practice_memberships WHERE staff_id = $1`, f.employeeID); err != nil {
					t.Fatalf("remove membership: %v", err)
				}
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newVisitFixture(t)
			f.queue(t, f.notice(tc.recipient(f), tc.kind))
			tc.change(t, f)

			if sent := f.run(t, putOnNow).Sent(); len(sent) != 0 {
				t.Fatalf("sent %d messages, want none", len(sent))
			}
			var status string
			if err := f.db.Admin.QueryRowContext(t.Context(),
				`SELECT status::text FROM staff_notice_outbox WHERE subject_id = $1`, f.visitID,
			).Scan(&status); err != nil {
				t.Fatalf("read outbox: %v", err)
			}
			if status != statusSent {
				t.Fatalf("status = %s, want sent", status)
			}
		})
	}
}
