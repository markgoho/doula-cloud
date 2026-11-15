package staffnotice_test

import (
	"strings"
	"testing"

	"doula-cloud/api/internal/staffnotice"
)

const requestDeclined = "declined"

type voidFixture struct {
	putOnFixture
	requestID string
}

func newVoidFixture(t *testing.T) voidFixture {
	t.Helper()
	f := newPutOnFixture(t)
	var contractID string
	if err := f.db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO contracts (engagement_id, status, prose, amount_cents, sent_at, signed_at)
		 VALUES ($1, 'signed', 'Agreement.', 15000, now(), now()) RETURNING id`,
		f.engagementID,
	).Scan(&contractID); err != nil {
		t.Fatalf("seed contract: %v", err)
	}
	var requestID string
	if err := f.db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO contract_void_requests (contract_id, engagement_id, practice_id, requested_by, reason)
		 VALUES ($1, $2, $3, $4, 'the plan changed') RETURNING id`,
		contractID, f.engagementID, f.practiceID, f.employeeID,
	).Scan(&requestID); err != nil {
		t.Fatalf("seed void request: %v", err)
	}
	return voidFixture{putOnFixture: f, requestID: requestID}
}

func (f voidFixture) notice(recipient string, kind staffnotice.Kind) staffnotice.Notice {
	return staffnotice.Notice{
		PracticeID:       f.practiceID,
		RecipientStaffID: recipient,
		Kind:             kind,
		SubjectID:        f.requestID,
		TriggeredBy:      f.ownerID,
	}
}

func (f voidFixture) decide(t *testing.T, status string) {
	t.Helper()
	if _, err := f.db.Admin.ExecContext(t.Context(),
		`UPDATE contract_void_requests SET status = $2::contract_void_request_status, decided_by = $3, decided_at = now(),
		        decline_reason = CASE WHEN $2 = 'declined' THEN 'not yet' END WHERE id = $1`,
		f.requestID, status, f.ownerID,
	); err != nil {
		t.Fatalf("decide: %v", err)
	}
}

func (f voidFixture) readStatus(t *testing.T) string {
	t.Helper()
	var status string
	if err := f.db.Admin.QueryRowContext(t.Context(),
		`SELECT status::text FROM staff_notice_outbox WHERE subject_id = $1`, f.requestID,
	).Scan(&status); err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	return status
}

func TestWorker_MailsEachOwnerWhenADoulaAsksForAVoid(t *testing.T) {
	f := newVoidFixture(t)
	f.queue(t, f.notice(f.ownerID, staffnotice.KindVoidRequested))

	msg := requireMails(t, f.run(t, putOnNow), 1)[0]
	if msg.To != "uid-owner@example.com" || msg.Subject != "A void request is waiting on DoulaCloud" {
		t.Fatalf("to = %q subject = %q", msg.To, msg.Subject)
	}
	link := appBaseURL + "/practices/" + f.practiceID + "/engagements/" + f.engagementID + "#void-request-" + f.requestID
	if !strings.Contains(msg.Text, "\n"+link+"\n") {
		t.Fatalf("text = %q, want the link %q on its own line", msg.Text, link)
	}
	for _, named := range []string{adaezeFirstName, adaezeFamilyName, practiceNamed, "plan changed", employeeUID} {
		if strings.Contains(msg.Subject+msg.Text, named) {
			t.Fatalf("mail names %q: %q", named, msg.Text)
		}
	}
}

func TestWorker_MailsTheDoulaTheAnswerToHerRequest(t *testing.T) {
	cases := map[string]struct {
		kind    staffnotice.Kind
		status  string
		subject string
		wording string
	}{
		"declined": {staffnotice.KindVoidDeclined, requestDeclined, "A void request was declined on DoulaCloud", "was declined"},
		"voided":   {staffnotice.KindVoidGranted, "voided", "A contract you asked to void was voided on DoulaCloud", "was voided"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newVoidFixture(t)
			f.decide(t, tc.status)
			f.queue(t, f.notice(f.employeeID, tc.kind))

			msg := requireMails(t, f.run(t, putOnNow), 1)[0]
			if msg.To != employeeUID+"@example.com" || msg.Subject != tc.subject {
				t.Fatalf("to = %q subject = %q", msg.To, msg.Subject)
			}
			link := appBaseURL + "/practices/" + f.practiceID + "/engagements/" + f.engagementID + "#void-request-" + f.requestID
			if !strings.Contains(msg.Text, tc.wording) || !strings.Contains(msg.Text, "\n"+link+"\n") {
				t.Fatalf("text = %q, want %q and the link", msg.Text, tc.wording)
			}
			if strings.Contains(msg.Text, "not yet") || strings.Contains(msg.Text, adaezeFirstName) {
				t.Fatalf("mail carries the reason or the Client: %q", msg.Text)
			}
		})
	}
}

func TestWorker_MailsNothingForAVoidRequestThatIsNoLongerWhatItSaid(t *testing.T) {
	cases := map[string]struct {
		kind   staffnotice.Kind
		change func(t *testing.T, f voidFixture)
	}{
		"asked, but already decided": {
			kind:   staffnotice.KindVoidRequested,
			change: func(t *testing.T, f voidFixture) { f.decide(t, "voided") },
		},
		"declined, but the request stands open": {
			kind:   staffnotice.KindVoidDeclined,
			change: func(*testing.T, voidFixture) {},
		},
		"voided, but it was declined": {
			kind:   staffnotice.KindVoidGranted,
			change: func(t *testing.T, f voidFixture) { f.decide(t, requestDeclined) },
		},
		"the request is gone": {
			kind: staffnotice.KindVoidRequested,
			change: func(t *testing.T, f voidFixture) {
				if _, err := f.db.Admin.ExecContext(t.Context(),
					`DELETE FROM contract_void_requests WHERE id = $1`, f.requestID); err != nil {
					t.Fatalf("delete request: %v", err)
				}
			},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newVoidFixture(t)
			f.queue(t, f.notice(f.ownerID, tc.kind))
			tc.change(t, f)

			requireMails(t, f.run(t, putOnNow), 0)
			if status := f.readStatus(t); status != statusSkipped {
				t.Fatalf("status = %s, want skipped", status)
			}
		})
	}
}

// A contractor is told how her request went only while she holds a granted
// Attachment on the Engagement; an employee is told whatever became of it.
func TestWorker_AContractorIsToldOnlyWhileSheIsOnTheEngagement(t *testing.T) {
	cases := map[string]struct {
		attached bool
		want     int
	}{
		"still on the Engagement": {attached: true, want: 1},
		"taken off":               {attached: false, want: 0},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newVoidFixture(t)
			if _, err := f.db.Admin.ExecContext(t.Context(),
				`UPDATE practice_memberships SET employment_type = 'contractor' WHERE staff_id = $1`, f.employeeID); err != nil {
				t.Fatalf("make contractor: %v", err)
			}
			if !tc.attached {
				f.endAttachment(t)
			}
			f.decide(t, requestDeclined)
			f.queue(t, f.notice(f.employeeID, staffnotice.KindVoidDeclined))

			requireMails(t, f.run(t, putOnNow), tc.want)
		})
	}
}
