package billing_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"testing"
	"time"

	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/internalauth"
	"doula-cloud/api/internal/testdb"
)

var monthlyNow = time.Date(2026, time.May, 15, 12, 0, 0, 0, time.UTC)

func newMonthlyCreditsServer(t *testing.T, db *testdb.DB) *httptest.Server {
	t.Helper()
	h := clock.Middleware(func() time.Time { return monthlyNow })(
		billing.MonthlyCreditsHandler(db.App, internalauth.FromSecret(internalTestSecret)))
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func at(month time.Month, day int) time.Time {
	return time.Date(2026, month, day, 12, 0, 0, 0, time.UTC)
}

func setPracticeCreated(t *testing.T, db *testdb.DB, practiceID string, created time.Time) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practices SET created_at = $2 WHERE id = $1`, practiceID, created); err != nil {
		t.Fatalf("set practice created_at: %v", err)
	}
}

func seedGrantAt(t *testing.T, db *testdb.DB, practiceID, origin string, quantity int, grantedBy any, when time.Time) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO credit_ledger (practice_id, origin, quantity, granted_by, created_at)
		 VALUES ($1, $2, $3, $4, $5)`, practiceID, origin, quantity, grantedBy, when); err != nil {
		t.Fatalf("seed %s: %v", origin, err)
	}
}

func seedConsumptionAt(t *testing.T, db *testdb.DB, practiceID, engagementID, lotID string, when time.Time) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO credit_ledger (practice_id, origin, quantity, consumed_engagement_id, consumed_at, drawn_lot_id, created_at)
		 VALUES ($1, 'consumption', -1, $2, $3, $4, $3)`, practiceID, engagementID, when, lotID); err != nil {
		t.Fatalf("seed consumption: %v", err)
	}
}

func getMonthly(t *testing.T, srv *httptest.Server, query, secret string) (int, []byte) {
	t.Helper()
	return internalRequest(t, srv, http.MethodGet, "/api/internal/billing/monthly-credits"+query, secret, "")
}

// TestMonthlyCreditsHandler_CountsPurchasesAndConsumptionsMonthByMonth
// seeds a Practice with a signup bonus, a founding grant, two purchases
// in different months and three consumptions, and reads her rows month
// by month: the grants count as neither bought nor spent.
func TestMonthlyCreditsHandler_CountsPurchasesAndConsumptionsMonthByMonth(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Monthly Read")
	setPracticeCreated(t, db, practiceID, at(time.January, 10))
	engagements := seedEngagements(t, db, practiceID, 3)

	seedGrantAt(t, db, practiceID, "signup_bonus", 3, nil, at(time.January, 10))
	seedGrantAt(t, db, practiceID, "founding_grant", 6, "Founder", at(time.January, 12))
	firstLot := seedPurchase(t, db, practiceID, 5, 2000, 0, "pi_monthly_feb", at(time.February, 3))
	seedConsumptionAt(t, db, practiceID, engagements[0], firstLot, at(time.February, 20))
	seedConsumptionAt(t, db, practiceID, engagements[1], firstLot, at(time.April, 2))
	seedConsumptionAt(t, db, practiceID, engagements[2], firstLot, at(time.April, 28))
	seedPurchase(t, db, practiceID, 10, 2000, 0, "pi_monthly_apr", at(time.April, 30))

	srv := newMonthlyCreditsServer(t, db)
	status, body := getMonthly(t, srv, "?month=2026-05", internalTestSecret)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", status, body)
	}
	var rows []billing.PracticeMonth
	if err := json.Unmarshal(body, &rows); err != nil {
		t.Fatalf("decode: %v", err)
	}

	type counts struct {
		month         string
		bought, spent int
	}
	want := []counts{
		{"2026-01", 0, 0},
		{"2026-02", 5, 1},
		{"2026-03", 0, 0},
		{"2026-04", 10, 2},
		{"2026-05", 0, 0},
	}
	if len(rows) != len(want) {
		t.Fatalf("rows = %+v, want %d months", rows, len(want))
	}
	for i, w := range want {
		got := rows[i]
		if got.Month != w.month || got.CreditsBought != w.bought || got.CreditsSpent != w.spent {
			t.Errorf("row %d = %s bought %d spent %d, want %s bought %d spent %d",
				i, got.Month, got.CreditsBought, got.CreditsSpent, w.month, w.bought, w.spent)
		}
		if got.PracticeID != practiceID || got.PracticeName != "Monthly Read" || got.PracticeCreatedMonth != "2026-01" {
			t.Errorf("row %d names %+v, want the seeded Practice created 2026-01", i, got)
		}
		if got.FirstPurchaseMonth == nil || *got.FirstPurchaseMonth != "2026-02" {
			t.Errorf("row %d firstPurchaseMonth = %v, want 2026-02", i, got.FirstPurchaseMonth)
		}
	}
}

// TestMonthlyCreditsHandler_StopsAtTheMonthAskedAndTheMonthDeleted
// proves the month range: a Practice created after the month asked has
// no rows, one never bought from reads a null first purchase, and a
// deleted Practice stops at the month she was deleted.
func TestMonthlyCreditsHandler_StopsAtTheMonthAskedAndTheMonthDeleted(t *testing.T) {
	db := testdb.New(t)
	early := testdb.SeedPractice(t, db, "Early")
	setPracticeCreated(t, db, early, at(time.February, 1))
	gone := testdb.SeedPractice(t, db, "Gone")
	setPracticeCreated(t, db, gone, at(time.February, 2))
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practices SET deleted_at = $2 WHERE id = $1`, gone, at(time.February, 20)); err != nil {
		t.Fatalf("delete practice: %v", err)
	}
	late := testdb.SeedPractice(t, db, "Late")
	setPracticeCreated(t, db, late, at(time.April, 1))

	srv := newMonthlyCreditsServer(t, db)
	status, body := getMonthly(t, srv, "?month=2026-03", internalTestSecret)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", status, body)
	}
	var rows []billing.PracticeMonth
	if err := json.Unmarshal(body, &rows); err != nil {
		t.Fatalf("decode: %v", err)
	}
	got := []string{}
	for _, r := range rows {
		got = append(got, r.PracticeName+" "+r.Month)
		if r.FirstPurchaseMonth != nil {
			t.Errorf("%s %s firstPurchaseMonth = %s, want null", r.PracticeName, r.Month, *r.FirstPurchaseMonth)
		}
	}
	want := []string{"Early 2026-02", "Early 2026-03", "Gone 2026-02"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
}

// TestMonthlyCreditsHandler_ReturnsOnlyPracticeAndLedgerFields fixes the
// field list: nothing naming a Staff member, a Client, a login or a
// feature.
func TestMonthlyCreditsHandler_ReturnsOnlyPracticeAndLedgerFields(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Field List")
	setPracticeCreated(t, db, practiceID, at(time.May, 1))

	srv := newMonthlyCreditsServer(t, db)
	status, body := getMonthly(t, srv, "?month=2026-05", internalTestSecret)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", status, body)
	}
	var rows []map[string]any
	if err := json.Unmarshal(body, &rows); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(rows) == 0 {
		t.Fatalf("no rows, want at least the current month")
	}
	keys := []string{}
	for k := range rows[0] {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	want := []string{"creditsBought", "creditsSpent", "firstPurchaseMonth", "month", "practiceCreatedMonth", "practiceId", "practiceName"}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("fields = %v, want exactly %v", keys, want)
	}
}

// TestMonthlyCreditsHandler_RefusesWhatItShould covers the guard and the
// month parameter: no caller, no month, a malformed month, and a month
// in the future are refused; the current month is not.
func TestMonthlyCreditsHandler_RefusesWhatItShould(t *testing.T) {
	db := testdb.New(t)
	srv := newMonthlyCreditsServer(t, db)

	cases := []struct {
		name, query, secret string
		want                int
	}{
		{"no caller", "?month=2026-05", "", http.StatusUnauthorized},
		{"no month", "", internalTestSecret, http.StatusBadRequest},
		{"malformed month", "?month=2026-5", internalTestSecret, http.StatusBadRequest},
		{"future month", "?month=2026-06", internalTestSecret, http.StatusBadRequest},
		{"current month", "?month=2026-05", internalTestSecret, http.StatusOK},
	}
	for _, c := range cases {
		if status, body := getMonthly(t, srv, c.query, c.secret); status != c.want {
			t.Errorf("%s: status = %d, want %d: %s", c.name, status, c.want, body)
		}
	}
}
