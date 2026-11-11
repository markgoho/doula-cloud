package payments_test

import (
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/testdb"
)

// These tests drive #2836 through the Client-portal Invoice reads: an
// unpaid Invoice carries its due day in the Practice's zone, and once a
// birth outcome is recorded only an Invoice raised after that carries it.

var (
	dueFarOut   = time.Date(2027, 3, 11, 2, 0, 0, 0, time.UTC)
	dayMar10    = "2027-03-10"
	recordedAt  = time.Date(2026, 12, 1, 15, 0, 0, 0, time.UTC)
	beforeAt    = recordedAt.Add(-48 * time.Hour)
	afterAt     = recordedAt.Add(48 * time.Hour)
	lateOrLater = regexp.MustCompile(`(?i)overdue|\blate\b|daysOverdue`)
)

func seedOutcomeRow(t *testing.T, db *testdb.DB, me clientMoney, before, after *string, at time.Time) {
	t.Helper()
	diff, err := json.Marshal(map[string]any{"birthOutcomeBefore": before, "birthOutcomeAfter": after})
	if err != nil {
		t.Fatalf("marshal diff: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO activity (practice_id, subject_kind, subject_id, action, diff, actor_kind, created_at)
		 VALUES ($1, 'engagement', $2, 'birth_outcome_recorded', $3, 'system', $4)`,
		me.practiceID, me.engagementID, string(diff), at); err != nil {
		t.Fatalf("seed outcome activity: %v", err)
	}
}

func setOutcome(t *testing.T, db *testdb.DB, me clientMoney, outcome string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE engagements SET birth_outcome = $1::birth_outcome, pregnancy_ended_on = CASE WHEN $1 = 'unknown' THEN NULL ELSE '2026-11-30'::date END WHERE id = $2`,
		outcome, me.engagementID); err != nil {
		t.Fatalf("set outcome: %v", err)
	}
}

// dueOnOf reads the Invoice from the detail and the list and requires the
// two to agree, returning the one answer.
func dueOnOf(t *testing.T, srv *httptest.Server, session string, me clientMoney, invoiceID string) *string {
	t.Helper()
	status, body := getPortal(t, srv, session, clientInvoicesPath(me.engagementID, "/"+invoiceID))
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", status, body)
	}
	if lateOrLater.MatchString(body) {
		t.Fatalf("body names lateness: %s", body)
	}
	var detail payments.ClientInvoiceView
	if err := json.Unmarshal([]byte(body), &detail); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var fromList *payments.ClientInvoiceView
	for _, it := range readClientInvoices(t, srv, session, me.engagementID, "").Items {
		if it.ID == invoiceID {
			fromList = &it
		}
	}
	if fromList == nil {
		t.Fatalf("invoice %s missing from list", invoiceID)
	}
	if (detail.DueOn == nil) != (fromList.DueOn == nil) || (detail.DueOn != nil && *detail.DueOn != *fromList.DueOn) {
		t.Fatalf("detail dueOn %v differs from list dueOn %v", detail.DueOn, fromList.DueOn)
	}
	return detail.DueOn
}

func wantDueOn(t *testing.T, got *string, want string) {
	t.Helper()
	if got == nil || *got != want {
		t.Fatalf("dueOn = %v, want %s", got, want)
	}
}

func wantNoDueOn(t *testing.T, got *string) {
	t.Helper()
	if got != nil {
		t.Fatalf("dueOn = %q, want none", *got)
	}
}

func TestClientInvoice_UnpaidShowsDueDayInThePracticesZone(t *testing.T) {
	db := testdb.New(t)
	const uid = "client-due-day"
	me := seedClientMoney(t, db, uid, "Hannah Client", "hannah@example.com")
	raised := time.Date(2027, 2, 1, 15, 0, 0, 0, time.UTC)
	open := seedInvoiceDue(t, db, me.practiceID, me.contractID, "in_due_open", invoiceStatusOpen, 150000, raised, dueFarOut)

	srv := newClientMoneyServer(t, db)
	wantDueOn(t, dueOnOf(t, srv, portalSession(t, db, uid), me, open), dayMar10)
}

func TestClientInvoice_TwoUnpaidShowTheirOwnDays(t *testing.T) {
	db := testdb.New(t)
	const uid = "client-two-due"
	me := seedClientMoney(t, db, uid, "Hannah Client", "hannah@example.com")
	raised := time.Date(2027, 2, 1, 15, 0, 0, 0, time.UTC)
	retainer := seedInvoiceDue(t, db, me.practiceID, me.contractID, "in_ret", invoiceStatusOpen, 150000, raised, time.Date(2027, 2, 1, 17, 0, 0, 0, time.UTC))
	balance := seedInvoiceDue(t, db, me.practiceID, me.contractID, "in_bal", invoiceStatusOpen, 300000, raised.Add(time.Minute), time.Date(2027, 5, 10, 16, 0, 0, 0, time.UTC))

	srv := newClientMoneyServer(t, db)
	session := portalSession(t, db, uid)
	wantDueOn(t, dueOnOf(t, srv, session, me, retainer), "2027-02-01")
	wantDueOn(t, dueOnOf(t, srv, session, me, balance), "2027-05-10")
	if got := readClientInvoices(t, srv, session, me.engagementID, "").TotalToPayCents; got != 450000 {
		t.Fatalf("totalToPayCents = %d, want 450000", got)
	}
}

func TestClientInvoice_NotOpenShowsNoDueDay(t *testing.T) {
	db := testdb.New(t)
	const uid = "client-settled-due"
	me := seedClientMoney(t, db, uid, "Hannah Client", "hannah@example.com")
	raised := time.Date(2027, 2, 1, 15, 0, 0, 0, time.UTC)
	srv := newClientMoneyServer(t, db)
	session := portalSession(t, db, uid)
	for i, status := range []string{invoiceStatusPaid, invoiceStatusVoid, invoiceStatusUncollectible} {
		id := seedInvoiceDue(t, db, me.practiceID, me.contractID, "in_settled_"+status, status, 5000, raised.Add(time.Duration(i)*time.Minute), dueFarOut)
		wantNoDueOn(t, dueOnOf(t, srv, session, me, id))
	}
}

func TestClientInvoice_DueDayStaysOnceItHasPassed(t *testing.T) {
	db := testdb.New(t)
	const uid = "client-past-due"
	me := seedClientMoney(t, db, uid, "Hannah Client", "hannah@example.com")
	raised := time.Date(2020, 1, 1, 15, 0, 0, 0, time.UTC)
	id := seedInvoiceDue(t, db, me.practiceID, me.contractID, "in_past", invoiceStatusOpen, 5000, raised, time.Date(2020, 1, 31, 17, 0, 0, 0, time.UTC))

	srv := newClientMoneyServer(t, db)
	wantDueOn(t, dueOnOf(t, srv, portalSession(t, db, uid), me, id), "2020-01-31")
}

func TestClientInvoice_AfterAnOutcomeOnlyALaterInvoiceShowsItsDay(t *testing.T) {
	for _, outcome := range []string{"loss", "live_birth", "unknown"} {
		t.Run(outcome, func(t *testing.T) {
			db := testdb.New(t)
			const uid = "client-outcome-due"
			me := seedClientMoney(t, db, uid, "Nadia Client", "nadia@example.com")
			setOutcome(t, db, me, outcome)
			seedOutcomeRow(t, db, me, nil, new(outcome), recordedAt)
			earlier := seedInvoiceDue(t, db, me.practiceID, me.contractID, "in_before", invoiceStatusOpen, 300000, beforeAt, dueFarOut)
			later := seedInvoiceDue(t, db, me.practiceID, me.contractID, "in_after", invoiceStatusOpen, 20000, afterAt, dueFarOut)

			srv := newClientMoneyServer(t, db)
			session := portalSession(t, db, uid)
			status, body := getPortal(t, srv, session, clientInvoicesPath(me.engagementID, "/"+earlier))
			if status != http.StatusOK {
				t.Fatalf("status = %d; body %s", status, body)
			}
			if regexp.MustCompile(`dueOn`).MatchString(body) || regexp.MustCompile(outcome+`|birth`).MatchString(body) {
				t.Fatalf("response for the earlier Invoice carries a day or the outcome: %s", body)
			}
			wantNoDueOn(t, dueOnOf(t, srv, session, me, earlier))
			wantDueOn(t, dueOnOf(t, srv, session, me, later), dayMar10)
		})
	}
}

func TestClientInvoice_PostpartumEngagementShowsTheDay(t *testing.T) {
	db := testdb.New(t)
	const uid = "client-postpartum-due"
	practiceID := testdb.SeedPractice(t, db, "Practice "+uid)
	clientID, engagementID := testdb.SeedEngagementWithKind(t, db, practiceID, "Camille Client", "camille@example.com", "postpartum")
	testdb.SeedPortalUser(t, db, testdb.PortalUID(uid), clientID)
	me := clientMoney{practiceID, clientID, engagementID, seedSignedContract(t, db, engagementID)}
	id := seedInvoiceDue(t, db, practiceID, me.contractID, "in_pp", invoiceStatusOpen, 50000, beforeAt, dueFarOut)

	srv := newClientMoneyServer(t, db)
	wantDueOn(t, dueOnOf(t, srv, portalSession(t, db, uid), me, id), dayMar10)
}

func TestClientInvoice_AnOutcomeWithNoRecordingTimeWithholdsEveryDay(t *testing.T) {
	db := testdb.New(t)
	const uid = "client-unrecorded"
	me := seedClientMoney(t, db, uid, "Nadia Client", "nadia@example.com")
	setOutcome(t, db, me, "loss")
	id := seedInvoiceDue(t, db, me.practiceID, me.contractID, "in_unrecorded", invoiceStatusOpen, 5000, afterAt, dueFarOut)

	srv := newClientMoneyServer(t, db)
	wantNoDueOn(t, dueOnOf(t, srv, portalSession(t, db, uid), me, id))
}

func TestClientInvoice_ACorrectionDoesNotMoveTheRecordingTime(t *testing.T) {
	db := testdb.New(t)
	const uid = "client-corrected"
	me := seedClientMoney(t, db, uid, "Nadia Client", "nadia@example.com")
	setOutcome(t, db, me, "unknown")
	seedOutcomeRow(t, db, me, nil, new("loss"), recordedAt)
	seedOutcomeRow(t, db, me, new("loss"), new("unknown"), afterAt.Add(24*time.Hour))
	between := seedInvoiceDue(t, db, me.practiceID, me.contractID, "in_between", invoiceStatusOpen, 5000, afterAt, dueFarOut)

	srv := newClientMoneyServer(t, db)
	wantDueOn(t, dueOnOf(t, srv, portalSession(t, db, uid), me, between), dayMar10)
}

func TestClientInvoice_AnOutcomeClearedAndEnteredAgainRestartsTheGate(t *testing.T) {
	db := testdb.New(t)
	const uid = "client-rerecorded"
	me := seedClientMoney(t, db, uid, "Nadia Client", "nadia@example.com")
	setOutcome(t, db, me, "loss")
	seedOutcomeRow(t, db, me, nil, new("loss"), recordedAt)
	seedOutcomeRow(t, db, me, new("loss"), nil, afterAt)
	seedOutcomeRow(t, db, me, nil, new("loss"), afterAt.Add(24*time.Hour))
	between := seedInvoiceDue(t, db, me.practiceID, me.contractID, "in_between2", invoiceStatusOpen, 5000, afterAt, dueFarOut)

	srv := newClientMoneyServer(t, db)
	wantNoDueOn(t, dueOnOf(t, srv, portalSession(t, db, uid), me, between))
}
