package contracts_test

import (
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/clientstanding"
	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// remindedNov20 is a Friday afternoon in the Practice's zone (UTC unless a
// test says otherwise), well after the instant a seeded Contract is sent.
var remindedNov20 = time.Date(2026, 11, 20, 15, 0, 0, 0, time.UTC)

func (e *env) remind() string { return e.contractURL("/remind") }

// reminders is the Engagement's contract_reminded entries, each
// "<actor kind> <actor staff id>".
func (e *env) reminders() []string {
	e.T.Helper()
	rows, err := e.DB.Admin.QueryContext(e.T.Context(),
		`SELECT actor_kind::text || ' ' || COALESCE(actor_staff_id::text, '') FROM activity
		  WHERE subject_id = $1 AND action = $2 ORDER BY created_at, id`,
		e.Engagement.ID, string(activity.ActionContractReminded))
	if err != nil {
		e.T.Fatalf("read reminders: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			e.T.Fatalf("scan reminder: %v", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		e.T.Fatalf("iterate reminders: %v", err)
	}
	return out
}

// remindable is an env whose Sent Contract was sent before today and whose
// Client can sign in.
func remindable(t *testing.T, opts ...envOption) *env {
	t.Helper()
	e := newEnv(t, append([]envOption{withPortalLogin(), atInstant(remindedNov20)}, opts...)...)
	e.seedContract(statusSent)
	return e
}

// #2951: a reminder emails her, pushes to her, stamps the Contract and
// records who sent it, leaving the Contract Sent.
func TestPostRemindContractHandler_RemindsTheClient(t *testing.T) {
	e := remindable(t)
	testdb.SeedPushSubscription(t, e.DB, "client", e.Engagement.Client.ID, "https://push.example.com/client-recipient")

	out := reqtest.Expect[contracts.ContractResponse](t, http.MethodPost, e.remind(), http.StatusOK, e.SignedIn)

	if out.Status != statusSent {
		t.Fatalf("status = %q, want sent", out.Status)
	}
	if out.Reminded == nil || !out.Reminded.At.Equal(remindedNov20) || out.Reminded.By == "" {
		t.Fatalf("reminded = %+v, want %v and the Staff member's name", out.Reminded, remindedNov20)
	}
	if got, want := e.pendingNotices(), "reminder "+e.Staff.ID; got != want {
		t.Fatalf("queued = %q, want %q", got, want)
	}
	if got, want := e.reminders(), []string{"staff " + e.Staff.ID}; len(got) != 1 || got[0] != want[0] {
		t.Fatalf("reminders = %v, want %v", got, want)
	}
	if calls := e.pusher.Calls(); len(calls) != 1 {
		t.Fatalf("pushes = %d, want 1", len(calls))
	}
	if got := e.contract().Reminded; got == nil || !got.At.Equal(remindedNov20) {
		t.Fatalf("GET reminded = %+v, want the reminder", got)
	}
}

func TestPostRemindContractHandler_RefusesEverythingButASentContract(t *testing.T) {
	for _, status := range []string{statusDraft, statusSigned, statusVoided} {
		t.Run(status, func(t *testing.T) {
			e := newEnv(t, withPortalLogin(), atInstant(remindedNov20))
			e.seedContract(status)

			reqtest.ExpectStatus(t, http.MethodPost, e.remind(), http.StatusConflict, e.SignedIn)

			if got := e.pendingNotices(); got != "" {
				t.Fatalf("queued = %q, want nothing", got)
			}
		})
	}
}

func TestPostRemindContractHandler_NoContract(t *testing.T) {
	e := newEnv(t)

	reqtest.ExpectStatus(t, http.MethodPost, e.remind(), http.StatusNotFound, e.SignedIn)
}

func TestPostRemindContractHandler_RefusesAnEngagementItCannotReach(t *testing.T) {
	e := newEnv(t)
	e.refusesEngagementsItCannotReach(http.MethodPost, "/remind")
}

func TestPostRemindContractHandler_ContractorWithoutAttachmentNotFound(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Contractor()))

	reqtest.ExpectStatus(t, http.MethodPost, e.remind(), http.StatusNotFound, e.SignedIn)
}

func TestPostRemindContractHandler_AnyoneWhoReachesTheEngagementMay(t *testing.T) {
	for _, tc := range []struct {
		name  string
		staff []fixture.StaffOption
	}{
		{caseOwner, []fixture.StaffOption{fixture.Owner()}},
		{caseAdmin, []fixture.StaffOption{fixture.Admin()}},
		{caseEmployedDoula, []fixture.StaffOption{fixture.Doula()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := remindable(t, signedInAs(tc.staff...))

			reqtest.ExpectStatus(t, http.MethodPost, e.remind(), http.StatusOK, e.SignedIn)
		})
	}
}

func TestPostRemindContractHandler_AContractorOnAGrantedAttachmentMay(t *testing.T) {
	e := remindable(t, signedInAs(fixture.Contractor()))
	testdb.SeedGrantedAttachment(e.T, e.DB, e.Engagement.ID, e.Staff.ID)

	reqtest.ExpectStatus(t, http.MethodPost, e.remind(), http.StatusOK, e.SignedIn)
}

func TestPostRemindContractHandler_RefusesAnErasedClient(t *testing.T) {
	e := remindable(t)
	fixture.EraseClient(t, e.DB, e.Engagement.Client.ID)

	got := apierrtest.Expect(t, http.MethodPost, e.remind(), http.StatusConflict, e.SignedIn)

	if _, want := clientstanding.ActSendContract.Check(clientstanding.Erased); got.Message != want {
		t.Errorf("message = %q, want %q", got.Message, want)
	}
}

// A Client she cannot reach is refused in words that say why, and nothing
// is written.
func TestPostRemindContractHandler_RefusesAClientTheReminderCannotReach(t *testing.T) {
	for _, tc := range []struct {
		name, change, want string
	}{
		{"never invited", ``, msgRemindNoInviteWords},
		{"refused", `UPDATE client_portal_users SET state = 'refused', access_refused_at = now() WHERE client_id = $1`, "This Client signs in through another record, so a reminder from this one has nowhere to go."},
		{"expired", `UPDATE client_portal_users SET invite_token_expires_at = now() - interval '1 day' WHERE client_id = $1`, "This Client's portal invite has expired, so a reminder has nowhere to go. Send a new portal invite first."},
		{"withdrawn", `UPDATE client_portal_users SET state = 'revoked', invite_token = NULL, invite_token_expires_at = NULL, revoked_at = now(), revoke_reason = 'email_changed' WHERE client_id = $1`, "This Client's portal invite was withdrawn, so a reminder has nowhere to go. Send a new portal invite first."},
		{"bounced", `UPDATE portal_invite_outbox SET status = 'bounced', sent_at = now() WHERE client_portal_user_id IN (SELECT id FROM client_portal_users WHERE client_id = $1)`, "This Client's portal invite did not arrive, so a reminder has nowhere to go. Send a new portal invite first."},
		{"unlinked", `UPDATE client_portal_users SET state = 'unlinked', invite_token = NULL, invite_token_expires_at = NULL, unlinked_at = now() WHERE client_id = $1`, "This Client no longer has portal access, so a reminder has nowhere to go."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, atInstant(remindedNov20))
			e.seedContract(statusSent)
			if tc.change != "" {
				testdb.SeedPendingPortalInvite(t, e.DB, e.Engagement.Client.ID)
				e.Exec(tc.change, e.Engagement.Client.ID)
			}

			out := apierrtest.Expect(t, http.MethodPost, e.remind(), http.StatusConflict, e.SignedIn)

			if out.Code != apierr.CodeFailedPrecondition || out.Message != tc.want {
				t.Fatalf("refusal = %+v, want FAILED_PRECONDITION %q", out, tc.want)
			}
			if got := e.pendingNotices(); got != "" || len(e.reminders()) != 0 {
				t.Fatalf("queued = %q, reminders = %v, want nothing written", got, e.reminders())
			}
		})
	}
}

const msgRemindNoInviteWords = "This Client has not been invited to the portal yet, so a reminder has nowhere to go."

// #2951: a Client is not emailed twice in a day, and the refusal says so
// and when to come back.
func TestPostRemindContractHandler_RefusesASecondReminderTheSameDay(t *testing.T) {
	e := remindable(t)
	reqtest.ExpectStatus(t, http.MethodPost, e.remind(), http.StatusOK, e.SignedIn)

	e.MoveClockTo(remindedNov20.Add(5 * time.Hour))
	out := apierrtest.Expect(t, http.MethodPost, e.remind(), http.StatusConflict, e.As(e.Staff))

	if out.Code != apierr.CodeFailedPrecondition || out.Message != "This Client was already emailed about a Contract today. You can send a reminder tomorrow." {
		t.Fatalf("refusal = %+v", out)
	}
	if got := len(e.reminders()); got != 1 {
		t.Fatalf("reminders = %d, want the one", got)
	}
}

func TestPostRemindContractHandler_AllowsTheNextDay(t *testing.T) {
	e := remindable(t)
	reqtest.ExpectStatus(t, http.MethodPost, e.remind(), http.StatusOK, e.SignedIn)

	e.MoveClockTo(remindedNov20.Add(24 * time.Hour))
	reqtest.ExpectStatus(t, http.MethodPost, e.remind(), http.StatusOK, e.As(e.Staff))

	if got := len(e.reminders()); got != 2 {
		t.Fatalf("reminders = %d, want 2", got)
	}
}

// "A day" is the Practice's, not UTC's: 04:00 UTC on the 21st is 11 pm on
// the 20th in New York's November, 06:00 UTC is already the 21st there, and
// 23:00 UTC is still that 21st.
func TestPostRemindContractHandler_ADayIsThePracticesDay(t *testing.T) {
	e := remindable(t, atInstant(time.Date(2026, 11, 21, 4, 0, 0, 0, time.UTC)))
	e.Exec(`UPDATE practices SET timezone = 'America/New_York' WHERE id = $1`, e.Practice.ID)
	reqtest.ExpectStatus(t, http.MethodPost, e.remind(), http.StatusOK, e.SignedIn)

	e.MoveClockTo(time.Date(2026, 11, 21, 6, 0, 0, 0, time.UTC))
	reqtest.ExpectStatus(t, http.MethodPost, e.remind(), http.StatusOK, e.As(e.Staff))

	e.MoveClockTo(time.Date(2026, 11, 21, 23, 0, 0, 0, time.UTC))
	reqtest.ExpectStatus(t, http.MethodPost, e.remind(), http.StatusConflict, e.As(e.Staff))
}

func TestPostRemindContractHandler_ABadStoredZoneIsAnErrorNotUTC(t *testing.T) {
	e := remindable(t)
	e.Exec(`UPDATE practices SET timezone = 'Nowhere/Bad' WHERE id = $1`, e.Practice.ID)

	reqtest.ExpectStatus(t, http.MethodPost, e.remind(), http.StatusInternalServerError, e.SignedIn)

	if got := len(e.reminders()); got != 0 {
		t.Fatalf("reminders = %d, want none", got)
	}
}

func TestPostRemindContractHandler_RefusesOnTheDayTheContractWasSent(t *testing.T) {
	e := remindable(t)
	e.Exec(`UPDATE contracts SET sent_at = $2 WHERE engagement_id = $1`, e.Engagement.ID, remindedNov20.Add(-time.Hour))

	reqtest.ExpectStatus(t, http.MethodPost, e.remind(), http.StatusConflict, e.SignedIn)
}

// She is the one reminded at most once a day, whichever of her Contracts
// it was about.
func TestPostRemindContractHandler_CountsEveryContractOfTheSameClient(t *testing.T) {
	e := remindable(t)
	other := e.AnotherEngagement(fixture.ForClient(e.Engagement.Client))
	seedContract(t, e.DB, other.ID, statusSent, mergeFieldProse)
	e.Exec(`UPDATE contracts SET reminded_at = $2 WHERE engagement_id = $1`, other.ID, remindedNov20.Add(-time.Hour))

	reqtest.ExpectStatus(t, http.MethodPost, e.remind(), http.StatusConflict, e.SignedIn)
}

func TestPostRemindContractHandler_AnotherClientIsNotCounted(t *testing.T) {
	e := remindable(t)
	other := e.AnotherEngagement()
	seedContract(t, e.DB, other.ID, statusSent, mergeFieldProse)
	e.Exec(`UPDATE contracts SET reminded_at = $2 WHERE engagement_id = $1`, other.ID, remindedNov20.Add(-time.Hour))

	reqtest.ExpectStatus(t, http.MethodPost, e.remind(), http.StatusOK, e.SignedIn)
}

// A repeat of the same request is the first one's answer, not a second email.
func TestPostRemindContractHandler_ReplaysTheFirstAnswer(t *testing.T) {
	e := remindable(t)
	opts := []reqtest.Option{e.SignedIn, reqtest.Header("Idempotency-Key", "11111111-1111-4111-8111-111111111111")}

	reqtest.ExpectStatus(t, http.MethodPost, e.remind(), http.StatusOK, opts...)
	reqtest.ExpectStatus(t, http.MethodPost, e.remind(), http.StatusOK, opts...)

	if got := len(e.reminders()); got != 1 {
		t.Fatalf("reminders = %d, want 1 for a replay", got)
	}
}

func TestListContracts_SaysWhenAndWhetherASentContractCanBeReminded(t *testing.T) {
	e := remindable(t, signedInAs(fixture.Owner()))
	e.Exec(`UPDATE practices SET timezone = 'America/New_York' WHERE id = $1`, e.Practice.ID)
	e.Exec(`UPDATE contracts SET sent_at = $2 WHERE engagement_id = $1`, e.Engagement.ID, remindedNov20.Add(-72*time.Hour))

	before := e.list("?view=sent").Items[0]
	if !before.CanRemind || before.RemindedOn != "" {
		t.Fatalf("before: canRemind = %v remindedOn = %q, want true and none", before.CanRemind, before.RemindedOn)
	}

	reqtest.ExpectStatus(t, http.MethodPost, e.remind(), http.StatusOK, e.SignedIn)

	after := e.list("?view=sent").Items[0]
	if after.CanRemind || after.RemindedOn != "2026-11-20" {
		t.Fatalf("after: canRemind = %v remindedOn = %q, want false and 2026-11-20", after.CanRemind, after.RemindedOn)
	}
}

func TestListContracts_ASentTodayContractCannotBeRemindedYet(t *testing.T) {
	e := remindable(t, signedInAs(fixture.Owner()))
	e.Exec(`UPDATE contracts SET sent_at = $2 WHERE engagement_id = $1`, e.Engagement.ID, remindedNov20.Add(-time.Hour))

	if item := e.list("?view=sent").Items[0]; item.CanRemind {
		t.Fatalf("canRemind = true on the day it was sent")
	}
}
