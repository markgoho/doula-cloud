package payments_test

import (
	"encoding/json/v2"
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/overdue"
	"doula-cloud/api/internal/payments"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// #2828: the day an Invoice falls due may be named at raising, as a date
// or as a week of pregnancy, and the Practice's payment terms decide it
// when neither is. The Practice's zone in these tests is America/New_York.

const (
	dueOnField = "dueOn"
	weekField  = "gestationalWeek"
	nov20      = "2026-11-20"
	railStripe = "stripe"
)

func practiceZone(t *testing.T) *time.Location {
	t.Helper()
	zone, err := time.LoadLocation(testdb.SeededPracticeZoneName)
	if err != nil {
		t.Fatalf("load %s: %v", testdb.SeededPracticeZoneName, err)
	}
	return zone
}

// tenAMOnNov10 is 10:00 on 2026-11-10 in the Practice's zone.
func tenAMOnNov10(t *testing.T) time.Time {
	return time.Date(2026, time.November, 10, 10, 0, 0, 0, practiceZone(t))
}

func (e *env) raiseDue(engagementID string, req payments.CreateInvoiceRequest) payments.InvoiceView {
	e.T.Helper()
	return reqtest.Expect[payments.InvoiceView](e.T, http.MethodPost, e.PracticeURL(invoicesPath(engagementID)), http.StatusCreated,
		e.SignedIn, reqtest.JSON(req))
}

func TestPostInvoiceHandler_ADateNamesTheDueDayOnBothRails(t *testing.T) {
	zone := practiceZone(t)
	rails := []struct {
		name string
		opt  envOption
	}{
		{"by hand", withBillingMode(payments.BillingModeByHand)},
		{railStripe, connected()},
	}
	for _, rail := range rails {
		t.Run(rail.name, func(t *testing.T) {
			e := newEnv(t, rail.opt, atInstant(tenAMOnNov10(t)))
			engagementID, _ := e.signedContract()

			view := e.raiseDue(engagementID, payments.CreateInvoiceRequest{DueOn: new(nov20)})

			stored := invoiceDueAtOf(t, e.DB, view.ID)
			if view.DueOn != nov20 || seededPracticeDay(t, stored) != nov20 {
				t.Fatalf("dueOn = %q, stored day = %q; want 2026-11-20 on both", view.DueOn, seededPracticeDay(t, stored))
			}
			if rail.name == railStripe {
				if sent := e.Stripe.CreateInvoiceCalls[0].DueAt; sent.Unix() != stored.Unix() {
					t.Fatalf("sent to Stripe = %v, stored = %v; want one instant", sent, stored)
				}
			}
			listed := e.listInvoices(engagementID).Items[0]
			if listed.DueOn != nov20 {
				t.Fatalf("listed dueOn = %q, want 2026-11-20", listed.DueOn)
			}
			if got := overdue.Days("open", false, stored, time.Date(2026, time.November, 20, 23, 59, 59, 0, zone), zone); got != 0 {
				t.Fatalf("days overdue at the end of the due day = %d, want 0", got)
			}
			if got := overdue.Days("open", false, stored, time.Date(2026, time.November, 21, 0, 0, 0, 0, zone), zone); got != 1 {
				t.Fatalf("days overdue from the start of the next day = %d, want 1", got)
			}
		})
	}
}

func TestPostInvoiceHandler_AWeekOfPregnancyResolvesFromTheDueDate(t *testing.T) {
	cases := []struct {
		week int
		want string
	}{
		{37, "2026-11-29"},
		{36, "2026-11-22"},
		{40, "2026-12-20"},
		{42, "2027-01-03"},
	}
	for _, tc := range cases {
		e := newEnv(t, withBillingMode(payments.BillingModeByHand), atInstant(tenAMOnNov10(t)))
		engagementID, _ := e.signedContract(fixture.Kind("birth"), fixture.DueDate("2026-12-20"))

		view := e.raiseDue(engagementID, payments.CreateInvoiceRequest{GestationalWeek: new(tc.week)})

		if view.DueOn != tc.want || seededPracticeDay(t, invoiceDueAtOf(t, e.DB, view.ID)) != tc.want {
			t.Fatalf("week %d: dueOn = %q, want %s", tc.week, view.DueOn, tc.want)
		}
	}
}

func TestPostInvoiceHandler_ADueDayAcrossAClockChangeStaysThatDay(t *testing.T) {
	for _, day := range []string{"2026-11-01", "2026-11-02", "2027-03-14", "2027-03-15"} {
		e := newEnv(t, withBillingMode(payments.BillingModeByHand), atInstant(tenAMOnNov10(t).AddDate(0, 0, -20)))
		engagementID, _ := e.signedContract()

		view := e.raiseDue(engagementID, payments.CreateInvoiceRequest{DueOn: new(day)})

		if got := seededPracticeDay(t, invoiceDueAtOf(t, e.DB, view.ID)); got != day {
			t.Fatalf("due day = %q, want %q", got, day)
		}
	}
}

func TestPostInvoiceHandler_RaisedOnItsOwnDueDayAfterNoonFallsDueLaterThatDay(t *testing.T) {
	now := time.Date(2026, time.November, 10, 18, 0, 0, 0, practiceZone(t))
	e := newEnv(t, connected(), atInstant(now))
	engagementID, _ := e.signedContract()

	view := e.raiseDue(engagementID, payments.CreateInvoiceRequest{DueOn: new("2026-11-10")})

	stored := invoiceDueAtOf(t, e.DB, view.ID)
	if !stored.After(now) || seededPracticeDay(t, stored) != "2026-11-10" {
		t.Fatalf("due_at = %v, want a later instant on 2026-11-10", stored)
	}
	if sent := e.Stripe.CreateInvoiceCalls[0].DueAt; sent.Unix() != stored.Unix() {
		t.Fatalf("sent to Stripe = %v, stored = %v", sent, stored)
	}
}

func TestPostInvoiceHandler_NamingNoDueDayLeavesItToThePaymentTerms(t *testing.T) {
	e := newEnv(t, withBillingMode(payments.BillingModeByHand), atInstant(tenAMOnNov10(t)))
	engagementID, _ := e.signedContract()
	e.setTerms(`{"netDays":14}`)

	view := e.raiseDue(engagementID, payments.CreateInvoiceRequest{})

	if want := payments.DueAfterTerms(view.CreatedAt, 14); !invoiceDueAtOf(t, e.DB, view.ID).Equal(want) {
		t.Fatalf("due_at = %v, want %v", invoiceDueAtOf(t, e.DB, view.ID), want)
	}
	if view.DueOn != "2026-11-24" {
		t.Fatalf("dueOn = %q, want 2026-11-24", view.DueOn)
	}
}

func TestPostInvoiceHandler_RefusesADueDayItCannotUse(t *testing.T) {
	cases := []struct {
		name      string
		opts      []fixture.EngagementOption
		req       payments.CreateInvoiceRequest
		wantField string
	}{
		{"a day before today", nil, payments.CreateInvoiceRequest{DueOn: new("2026-11-09")}, dueOnField},
		{"a date that is not a date", nil, payments.CreateInvoiceRequest{DueOn: new("next week")}, dueOnField},
		{"a date and a week together", []fixture.EngagementOption{fixture.DueDate("2026-12-20")}, payments.CreateInvoiceRequest{DueOn: new(nov20), GestationalWeek: new(37)}, dueOnField},
		{"a week below the range", []fixture.EngagementOption{fixture.DueDate("2026-12-20")}, payments.CreateInvoiceRequest{GestationalWeek: new(19)}, weekField},
		{"a week above the range", []fixture.EngagementOption{fixture.DueDate("2026-12-20")}, payments.CreateInvoiceRequest{GestationalWeek: new(43)}, weekField},
		{"a week on a postpartum Engagement", []fixture.EngagementOption{fixture.Kind("postpartum"), fixture.DueDate("2026-12-20")}, payments.CreateInvoiceRequest{GestationalWeek: new(37)}, weekField},
		{"a week with no due date", []fixture.EngagementOption{fixture.Kind("birth")}, payments.CreateInvoiceRequest{GestationalWeek: new(37)}, weekField},
		{"a week that falls before today", []fixture.EngagementOption{fixture.Kind("birth"), fixture.DueDate(nov20)}, payments.CreateInvoiceRequest{GestationalWeek: new(37)}, dueOnField},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, connected(), atInstant(tenAMOnNov10(t)))
			engagementID, _ := e.signedContract(tc.opts...)

			refusal := e.postRefused(invoicesPath(engagementID), http.StatusBadRequest, reqtest.JSON(tc.req))

			if refusal.Details[tc.wantField] == "" {
				t.Fatalf("details = %v, want an entry for %q", refusal.Details, tc.wantField)
			}
			e.requireNothingRaised()
		})
	}
}

func TestPostInvoiceHandler_ActivityRecordsTheDueDayBesideTheAmount(t *testing.T) {
	cases := []struct {
		name string
		req  payments.CreateInvoiceRequest
		want string
	}{
		{"a chosen day", payments.CreateInvoiceRequest{DueOn: new(nov20)}, nov20},
		{"the terms' day", payments.CreateInvoiceRequest{}, "2026-12-10"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, withBillingMode(payments.BillingModeByHand), atInstant(tenAMOnNov10(t)))
			engagementID, _ := e.signedContract()

			e.raiseDue(engagementID, tc.req)

			diff := testdb.Scalar[[]byte](t, e.DB.Admin, `SELECT diff FROM activity WHERE subject_id = $1 AND action = 'invoice_raised'`, engagementID)
			var decoded struct {
				AmountCents int64  `json:"amountCents"`
				DueOn       string `json:"dueOn"`
			}
			if err := json.Unmarshal(diff, &decoded); err != nil {
				t.Fatalf("decode activity diff: %v", err)
			}
			if decoded.DueOn != tc.want || decoded.AmountCents != 15000 {
				t.Fatalf("diff = %+v, want dueOn %s beside amountCents 15000", decoded, tc.want)
			}
		})
	}
}
