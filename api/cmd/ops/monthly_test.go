package main

import (
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/testdb"
)

const (
	nameField = "name"
	september = "2026-09"
)

var newYork = mustNewYork()

func mustNewYork() *time.Location {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		panic(err)
	}
	return loc
}

func nyTime(month time.Month, day int) time.Time {
	return time.Date(2026, month, day, 9, 0, 0, 0, newYork)
}

type monthlyStack struct {
	*opsStack
	practiceID string
}

// newMonthlyStack seeds one Practice signed up in July 2026 that buys in
// August and September and starts Engagements in both, with a signup
// bonus and a founding grant that count as neither.
func newMonthlyStack(t *testing.T) *monthlyStack {
	t.Helper()
	s := &monthlyStack{opsStack: newOpsStack(t, false)}
	s.practiceID = testdb.SeedPractice(t, s.db, "Rooted Birth Collective")
	s.signedUp(s.practiceID, nyTime(time.July, 20))
	s.ledger(s.practiceID, "signup_bonus", 5, nyTime(time.July, 20))
	actID := testdb.SeedOperatorAct(t, s.db, "Mark Goho")
	s.exec(`INSERT INTO credit_ledger (practice_id, origin, quantity, operator_act_id, created_at) VALUES ($1, 'founding_grant', 6, $2, $3)`,
		s.practiceID, actID, nyTime(time.September, 2))
	s.purchase(s.practiceID, 5, "pi_aug", nyTime(time.August, 3))
	s.purchase(s.practiceID, 10, "pi_sep", nyTime(time.September, 10))
	s.consume(s.practiceID, "Hannah Sorensen", nyTime(time.August, 20))
	s.consume(s.practiceID, "Camille Boyd", nyTime(time.September, 4))
	s.consume(s.practiceID, "Nadia Haddad", nyTime(time.September, 28))
	return s
}

func (s *opsStack) signedUp(practiceID string, at time.Time) {
	s.exec(`UPDATE practices SET created_at = $2 WHERE id = $1`, practiceID, at)
}

func (s *opsStack) ledger(practiceID, origin string, quantity int, at time.Time) {
	s.exec(`INSERT INTO credit_ledger (practice_id, origin, quantity, created_at) VALUES ($1, $2, $3, $4)`, practiceID, origin, quantity, at)
}

func (s *opsStack) purchase(practiceID string, quantity int, intent string, at time.Time) {
	s.exec(`INSERT INTO credit_ledger (practice_id, origin, quantity, unit_price_cents, tax_cents, stripe_payment_intent_id, created_at)
	        VALUES ($1, 'purchase', $2, 2000, 0, $3, $4)`, practiceID, quantity, intent, at)
}

func (s *opsStack) consume(practiceID, clientName string, at time.Time) {
	email := strings.ToLower(strings.Fields(clientName)[0]) + "@example.com"
	_, engagementID := testdb.SeedNamedEngagement(s.t, s.db, practiceID, clientName, email)
	s.exec(`INSERT INTO credit_ledger (practice_id, origin, quantity, consumed_engagement_id, consumed_at, drawn_lot_id, created_at)
	        SELECT $1, 'consumption', -1, $2, $3, id, $3 FROM credit_ledger
	         WHERE practice_id = $1 AND origin IN ('signup_bonus', 'purchase') ORDER BY created_at LIMIT 1`,
		practiceID, engagementID, at)
}

type monthlyRead struct {
	Month     string                       `json:"month"`
	Months    []string                     `json:"months"`
	Practices []map[string]json.RawMessage `json:"practices"`
	Totals    map[string]int               `json:"totals"`
}

func (s *opsStack) monthly(query, cookie string) *httptest.ResponseRecorder {
	s.t.Helper()
	return s.list("/api/credits/monthly"+query, cookie)
}

func decodeMonthly(t *testing.T, w *httptest.ResponseRecorder) monthlyRead {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", w.Code, w.Body)
	}
	var read monthlyRead
	if err := json.Unmarshal(w.Body.Bytes(), &read); err != nil {
		t.Fatalf("decode %s: %v", w.Body, err)
	}
	return read
}

func TestMonthlyCredits_CountsTheMonthAskedForPerPracticeWithTotals(t *testing.T) {
	s := newMonthlyStack(t)
	other := testdb.SeedPractice(t, s.db, "Atlas Doulas")
	s.signedUp(other, nyTime(time.August, 1))
	s.purchase(other, 3, "pi_other", nyTime(time.September, 5))

	read := decodeMonthly(t, s.monthly("?month="+september, s.signedInCookie()))

	if read.Month != september {
		t.Errorf("month = %s, want 2026-09", read.Month)
	}
	got, _ := json.Marshal(read.Practices)
	want := compactJSON(t, `[
		{"id": "`+other+`", "name": "Atlas Doulas", "creditsBought": 3, "creditsSpent": 0, "boughtBefore": false},
		{"id": "`+s.practiceID+`", "name": "Rooted Birth Collective", "creditsBought": 10, "creditsSpent": 2, "boughtBefore": true}
	]`)
	if compactJSON(t, string(got)) != want {
		t.Errorf("practices =\n%s\nwant\n%s", got, want)
	}
	if want := map[string]int{"creditsBought": 13, "creditsSpent": 2, "practicesBoughtAgain": 1}; !maps.Equal(read.Totals, want) {
		t.Errorf("totals = %v, want %v", read.Totals, want)
	}
}

func TestMonthlyCredits_DefaultsToTheLastClosedMonthAndNeverOffersTheCurrentOne(t *testing.T) {
	s := newMonthlyStack(t)
	s.now = time.Date(2026, time.November, 1, 2, 0, 0, 0, time.UTC)

	read := decodeMonthly(t, s.monthly("", s.signedInCookie()))

	if read.Month != september {
		t.Errorf("month = %s, want 2026-09: at 22:00 on October 31 in New York, October has not closed", read.Month)
	}
	if want := []string{september, "2026-08", "2026-07"}; !slices.Equal(read.Months, want) {
		t.Errorf("months = %v, want %v", read.Months, want)
	}
}

func TestMonthlyCredits_DecidesMonthsInNewYork(t *testing.T) {
	s := newMonthlyStack(t)
	s.purchase(s.practiceID, 4, "pi_late_sep", time.Date(2026, time.October, 1, 2, 0, 0, 0, time.UTC))
	cookie := s.signedInCookie()

	sep := decodeMonthly(t, s.monthly("?month="+september, cookie))
	oct := decodeMonthly(t, s.monthly("?month=2026-10", cookie))

	if got := string(sep.Practices[0]["creditsBought"]); got != "14" {
		t.Errorf("September bought = %s, want 14: 02:00 UTC on October 1 is September 30 in New York", got)
	}
	if got := string(oct.Practices[0]["creditsBought"]); got != "0" {
		t.Errorf("October bought = %s, want 0", got)
	}
}

func TestMonthlyCredits_RefusesTheCurrentMonthAFutureOneAndAMalformedOne(t *testing.T) {
	s := newMonthlyStack(t)
	cookie := s.signedInCookie()

	for _, month := range []string{"2026-11", "2027-01", "november", "2026-13"} {
		w := s.monthly("?month="+month, cookie)
		if w.Code != http.StatusBadRequest {
			t.Errorf("month %s: status = %d, want 400; body %s", month, w.Code, w.Body)
		}
		if !strings.Contains(w.Body.String(), `"month"`) {
			t.Errorf("month %s: body %s names no month field", month, w.Body)
		}
	}
}

func TestMonthlyCredits_ListsAPracticeOnlyForTheMonthsItExisted(t *testing.T) {
	s := newMonthlyStack(t)
	gone := testdb.SeedPractice(t, s.db, "Gone")
	s.signedUp(gone, nyTime(time.July, 1))
	s.exec(`UPDATE practices SET deleted_at = $2 WHERE id = $1`, gone, nyTime(time.August, 15))
	cookie := s.signedInCookie()

	names := func(month string) []string {
		out := []string{}
		for _, p := range decodeMonthly(t, s.monthly("?month="+month, cookie)).Practices {
			var name string
			_ = json.Unmarshal(p[nameField], &name)
			out = append(out, name)
		}
		return out
	}

	if got := names("2026-08"); !slices.Equal(got, []string{"Gone", "Rooted Birth Collective"}) {
		t.Errorf("August = %v, want both", got)
	}
	if got := names(september); !slices.Equal(got, []string{"Rooted Birth Collective"}) {
		t.Errorf("September = %v, want only the live Practice", got)
	}
	if got := names("2026-06"); len(got) != 0 {
		t.Errorf("June = %v, want none", got)
	}
}

func TestMonthlyCredits_WithNoPracticeOffersOnlyTheLastClosedMonth(t *testing.T) {
	s := newOpsStack(t, false)

	read := decodeMonthly(t, s.monthly("", s.signedInCookie()))

	if read.Month != "2026-10" || !slices.Equal(read.Months, []string{"2026-10"}) || len(read.Practices) != 0 {
		t.Errorf("read = %+v, want October alone and no Practice", read)
	}
}

func TestMonthlyCredits_CarriesNoPersonAndWritesNoReadRow(t *testing.T) {
	s := newMonthlyStack(t)
	cookie := s.signedInCookie()
	before := s.count(`SELECT count(*) FROM operator_acts`)

	read := decodeMonthly(t, s.monthly("?month="+september, cookie))

	keys := []string{}
	for k := range read.Practices[0] {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	if want := []string{"boughtBefore", "creditsBought", "creditsSpent", "id", nameField}; !slices.Equal(keys, want) {
		t.Errorf("fields = %v, want %v", keys, want)
	}
	if after := s.count(`SELECT count(*) FROM operator_acts`); after != before {
		t.Errorf("operator_acts rows %d -> %d, want no read row for a page that shows nobody", before, after)
	}
}

func TestMonthlyCredits_RefusedWithoutASession(t *testing.T) {
	s := newOpsStack(t, false)

	if w := s.monthly("", ""); w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}
