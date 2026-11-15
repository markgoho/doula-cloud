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

const (
	nextDay            = "2026-11-21"
	kindReminder       = "reminder"
	noticePending      = "pending"
	noticeDeadLettered = "dead_lettered"
)

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

// seedNotice writes a contract_notice_outbox row for the Contract of
// engagementID, as the send or a reminder would have left it by now.
func (e *env) seedNotice(engagementID, kind, status string, at time.Time) {
	e.T.Helper()
	e.Exec(`INSERT INTO contract_notice_outbox (practice_id, contract_id, kind, triggered_by_staff_id, status, next_attempt_at, created_at, sent_at, dead_letter_reason)
	        SELECT $1, ct.id, $3::contract_notice_kind, $2, $4::contract_notice_outbox_status, $5, $5,
	               CASE WHEN $4 = 'sent' THEN $5::timestamptz END,
	               CASE WHEN $4 = 'dead_lettered' THEN 'own_data'::outbox_dead_letter_reason END
	          FROM contracts ct WHERE ct.engagement_id = $6`,
		e.Practice.ID, e.Staff.ID, kind, status, at, engagementID)
}

// seedSkippedNotice is seedNotice for a row the worker skipped.
func (e *env) seedSkippedNotice(engagementID, kind string, at time.Time) {
	e.T.Helper()
	e.Exec(`INSERT INTO contract_notice_outbox (practice_id, contract_id, kind, triggered_by_staff_id, status, next_attempt_at, created_at, skip_reason, skipped_at)
	        SELECT $1, ct.id, $3::contract_notice_kind, $2, 'skipped', $4, $4, 'already_resolved', $4
	          FROM contracts ct WHERE ct.engagement_id = $5`,
		e.Practice.ID, e.Staff.ID, kind, at, engagementID)
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
	if out.Reminded == nil || !out.Reminded.At.Equal(remindedNov20) || out.Reminded.By != e.Staff.Name || out.Reminded.Delivery != "queued" {
		t.Fatalf("reminded = %+v, want %v by %s, queued", out.Reminded, remindedNov20, e.Staff.Name)
	}
	if out.CanRemind || out.CanRemindOn != nextDay {
		t.Fatalf("canRemind = %v on %q, want false until 2026-11-21", out.CanRemind, out.CanRemindOn)
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

	e.Exec(`UPDATE contract_notice_outbox SET status = 'sent', sent_at = created_at`)
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
	e.seedNotice(e.Engagement.ID, statusSent, statusSent, remindedNov20.Add(-time.Hour))

	reqtest.ExpectStatus(t, http.MethodPost, e.remind(), http.StatusConflict, e.SignedIn)
}

// She is the one reminded at most once a day, whichever of her Contracts
// it was about.
func TestPostRemindContractHandler_CountsEveryContractOfTheSameClient(t *testing.T) {
	e := remindable(t)
	other := e.AnotherEngagement(fixture.ForClient(e.Engagement.Client))
	seedContract(t, e.DB, other.ID, statusSent, mergeFieldProse)
	e.seedNotice(other.ID, kindReminder, statusSent, remindedNov20.Add(-time.Hour))

	reqtest.ExpectStatus(t, http.MethodPost, e.remind(), http.StatusConflict, e.SignedIn)
}

func TestPostRemindContractHandler_AnotherClientIsNotCounted(t *testing.T) {
	e := remindable(t)
	other := e.AnotherEngagement()
	seedContract(t, e.DB, other.ID, statusSent, mergeFieldProse)
	e.seedNotice(other.ID, kindReminder, statusSent, remindedNov20.Add(-time.Hour))

	reqtest.ExpectStatus(t, http.MethodPost, e.remind(), http.StatusOK, e.SignedIn)
}

// #2978: mail she never got holds nothing. A mail still waiting to go, or
// one that went, holds the day; one that dead-lettered or was skipped frees
// it, for a reminder and for the mail at send alike.
func TestPostRemindContractHandler_OnlyMailQueuedOrSentHoldsTheDay(t *testing.T) {
	for _, tc := range []struct {
		kind, status string
		held         bool
	}{
		{kindReminder, noticePending, true},
		{kindReminder, statusSent, true},
		{statusSent, noticePending, true},
		{statusSent, statusSent, true},
		{kindReminder, noticeDeadLettered, false},
		{statusSent, noticeDeadLettered, false},
		{kindReminder, "skipped", false},
	} {
		t.Run(tc.kind+" "+tc.status, func(t *testing.T) {
			e := remindable(t)
			if tc.status == "skipped" {
				e.seedSkippedNotice(e.Engagement.ID, tc.kind, remindedNov20.Add(-time.Hour))
			} else {
				e.seedNotice(e.Engagement.ID, tc.kind, tc.status, remindedNov20.Add(-time.Hour))
			}

			want := http.StatusOK
			if tc.held {
				want = http.StatusConflict
			}
			reqtest.ExpectStatus(t, http.MethodPost, e.remind(), want, e.SignedIn)
		})
	}
}

// #2978: the list, the Engagement and the endpoint read one rule. For each
// way a Client can have been emailed today, what the list and the
// Engagement offer is what pressing Remind then does.
func TestRemindIsOfferedExactlyWhereItWillBeSent(t *testing.T) {
	for _, tc := range []struct {
		name         string
		kind, status string
		offered      bool
	}{
		{"never emailed today", "", "", true},
		{"mail at send is waiting", statusSent, noticePending, false},
		{"mail at send went out", statusSent, statusSent, false},
		{"mail at send dead-lettered", statusSent, noticeDeadLettered, true},
		{"reminder went out", kindReminder, statusSent, false},
		{"reminder dead-lettered", kindReminder, noticeDeadLettered, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := remindable(t, signedInAs(fixture.Owner()))
			if tc.kind != "" {
				e.seedNotice(e.Engagement.ID, tc.kind, tc.status, remindedNov20.Add(-time.Hour))
			}

			item := e.list("?view=sent").Items[0]
			page := e.contract()
			if item.CanRemind != tc.offered || page.CanRemind != tc.offered {
				t.Fatalf("list canRemind = %v, Engagement canRemind = %v, want %v", item.CanRemind, page.CanRemind, tc.offered)
			}
			wantOn := ""
			if !tc.offered {
				wantOn = nextDay
			}
			if item.CanRemindOn != wantOn || page.CanRemindOn != wantOn {
				t.Fatalf("list canRemindOn = %q, Engagement canRemindOn = %q, want %q", item.CanRemindOn, page.CanRemindOn, wantOn)
			}
			want := http.StatusConflict
			if tc.offered {
				want = http.StatusOK
			}
			reqtest.ExpectStatus(t, http.MethodPost, e.remind(), want, e.SignedIn)
		})
	}
}

// #2978: a Client emailed about another of her Contracts today is not
// offered Remind on this one, and is told when she can be.
func TestListContracts_AClientEmailedAboutAnotherContractIsNotOfferedRemind(t *testing.T) {
	e := remindable(t, signedInAs(fixture.Owner()))
	other := e.AnotherEngagement(fixture.ForClient(e.Engagement.Client))
	seedContract(t, e.DB, other.ID, statusSent, mergeFieldProse)
	e.seedNotice(other.ID, kindReminder, statusSent, remindedNov20.Add(-time.Hour))

	for _, item := range e.list("?view=sent").Items {
		if item.CanRemind || item.CanRemindOn != nextDay {
			t.Fatalf("%s: canRemind = %v on %q, want false until 2026-11-21", item.ContractID, item.CanRemind, item.CanRemindOn)
		}
	}
}

// #2978: the Engagement says what became of the latest reminder, and the
// list's day counts only a reminder that was sent or is on its way.
func TestEngagementContract_SaysWhatBecameOfTheLatestReminder(t *testing.T) {
	for _, tc := range []struct {
		status, delivery, listedOn string
	}{
		{noticePending, "queued", "2026-11-19"},
		{statusSent, statusSent, "2026-11-19"},
		{noticeDeadLettered, "not_delivered", ""},
	} {
		t.Run(tc.status, func(t *testing.T) {
			e := remindable(t, signedInAs(fixture.Owner()))
			e.seedNotice(e.Engagement.ID, kindReminder, tc.status, remindedNov20.Add(-24*time.Hour))

			got := e.contract().Reminded
			if got == nil || got.Delivery != tc.delivery || got.By != e.Staff.Name || !got.At.Equal(remindedNov20.Add(-24*time.Hour)) {
				t.Fatalf("reminded = %+v, want %s by %s", got, tc.delivery, e.Staff.Name)
			}
			if on := e.list("?view=sent").Items[0].RemindedOn; on != tc.listedOn {
				t.Fatalf("list remindedOn = %q, want %q", on, tc.listedOn)
			}
		})
	}
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

	before := e.list("?view=sent").Items[0]
	if !before.CanRemind || before.RemindedOn != "" {
		t.Fatalf("before: canRemind = %v remindedOn = %q, want true and none", before.CanRemind, before.RemindedOn)
	}

	reqtest.ExpectStatus(t, http.MethodPost, e.remind(), http.StatusOK, e.SignedIn)

	after := e.list("?view=sent").Items[0]
	if after.CanRemind || after.CanRemindOn != nextDay || after.RemindedOn != "2026-11-20" {
		t.Fatalf("after: canRemind = %v on %q remindedOn = %q, want false until 2026-11-21 and 2026-11-20", after.CanRemind, after.CanRemindOn, after.RemindedOn)
	}
}

func TestEngagementContract_ABadStoredZoneIsAnErrorNotAGuess(t *testing.T) {
	e := remindable(t)
	e.Exec(`UPDATE practices SET timezone = 'Nowhere/Bad' WHERE id = $1`, e.Practice.ID)

	reqtest.ExpectStatus(t, http.MethodGet, e.contractURL(""), http.StatusInternalServerError, e.SignedIn)
}

func TestListContracts_ASentTodayContractCannotBeRemindedYet(t *testing.T) {
	e := remindable(t, signedInAs(fixture.Owner()))
	e.seedNotice(e.Engagement.ID, statusSent, noticePending, remindedNov20.Add(-time.Hour))

	if item := e.list("?view=sent").Items[0]; item.CanRemind {
		t.Fatalf("canRemind = true on the day it was sent")
	}
}
