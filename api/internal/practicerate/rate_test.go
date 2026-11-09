package practicerate_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// TestGetRatesHandler_NoRateSetIsValid proves an unset rate card reads as
// both kinds present with a null amountCents, not a 404 or an error --
// #966's AC that a Practice with no rate set is a valid state.
func TestGetRatesHandler_NoRateSetIsValid(t *testing.T) {
	e := newEnv(t)

	rates := e.rates().Rates
	if len(rates) != 2 {
		t.Fatalf("rates = %v, want both kinds present", rates)
	}
	for _, rate := range rates {
		if rate.AmountCents != nil {
			t.Fatalf("rate %q amountCents = %v, want nil", rate.Kind, *rate.AmountCents)
		}
	}
}

// TestGetRatesHandler_AnyRoleReads table-drives #966's AC that every
// Staff member with practice access reads the rate card, contractors
// included.
func TestGetRatesHandler_AnyRoleReads(t *testing.T) {
	cases := []struct {
		name string
		opts []fixture.StaffOption
	}{
		{"employee doula", []fixture.StaffOption{fixture.Doula()}},
		{"contractor doula", []fixture.StaffOption{fixture.Doula(), fixture.Contractor()}},
		{"employee admin", []fixture.StaffOption{fixture.Admin()}},
		{"contractor admin", []fixture.StaffOption{fixture.Admin(), fixture.Contractor()}},
		{"employee owner", []fixture.StaffOption{fixture.Owner()}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(tc.opts...))
			e.seedRate(fixture.KindBirth, 15000)

			if got := e.amountOf(fixture.KindBirth); got == nil || *got != 15000 {
				t.Fatalf("birth rate = %v, want 15000", got)
			}
		})
	}
}

// TestPutRateHandler_Refusals table-drives who may set a rate and what
// the API refuses before any write: a Doula, employee or contractor
// (#966's AC), a kind outside birth/postpartum, malformed JSON, and a
// zero or negative amount. An Admin, not only an Owner, may set one.
func TestPutRateHandler_Refusals(t *testing.T) {
	amount := func(cents int64) reqtest.Option { return reqtest.JSON(map[string]int64{"amountCents": cents}) }
	cases := []struct {
		name string
		opts []fixture.StaffOption
		kind string
		body reqtest.Option
		want int
	}{
		{"employee doula", []fixture.StaffOption{fixture.Doula()}, fixture.KindBirth, amount(15000), http.StatusForbidden},
		{"contractor doula", []fixture.StaffOption{fixture.Doula(), fixture.Contractor()}, fixture.KindBirth, amount(15000), http.StatusForbidden},
		{"admin", []fixture.StaffOption{fixture.Admin()}, fixture.KindBirth, amount(15000), http.StatusOK},
		{"invalid kind", []fixture.StaffOption{fixture.Owner()}, "insurance", amount(15000), http.StatusBadRequest},
		{"invalid body", []fixture.StaffOption{fixture.Owner()}, fixture.KindBirth, reqtest.RawJSON([]byte("not json")), http.StatusBadRequest},
		{"zero amount", []fixture.StaffOption{fixture.Owner()}, fixture.KindBirth, amount(0), http.StatusBadRequest},
		{"negative amount", []fixture.StaffOption{fixture.Owner()}, fixture.KindBirth, amount(-100), http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(tc.opts...))

			reqtest.ExpectStatus(t, http.MethodPut, e.rateURL(tc.kind), tc.want, e.SignedIn, tc.body)
		})
	}
}

// TestPutRateHandler_Success proves a set round-trips through GET, and
// that a second PUT with a new amount replaces it -- both kinds set
// independently.
func TestPutRateHandler_Success(t *testing.T) {
	e := ownerEnv(t)

	e.putRate(fixture.KindBirth, 15000, http.StatusOK)
	if got := e.amountOf(fixture.KindBirth); got == nil || *got != 15000 {
		t.Fatalf("birth rate after set = %v, want 15000", got)
	}

	e.putRate(fixture.KindBirth, 20000, http.StatusOK)
	if got := e.amountOf(fixture.KindBirth); got == nil || *got != 20000 {
		t.Fatalf("birth rate after replace = %v, want 20000", got)
	}
	if got := e.amountOf(fixture.KindPostpartum); got != nil {
		t.Fatalf("postpartum rate = %v, want nil (never set)", *got)
	}
}

// TestPutRateHandler_RepeatedIdenticalValueRecordsNoNewActivity proves
// the idempotent-PUT shape: a retry with the same amount writes no
// second activity row, mirroring
// staffauth.PutMFARequiredHandler_test's own repeat case.
func TestPutRateHandler_RepeatedIdenticalValueRecordsNoNewActivity(t *testing.T) {
	e := ownerEnv(t)

	e.putRate(fixture.KindBirth, 15000, http.StatusOK)
	e.putRate(fixture.KindBirth, 15000, http.StatusOK)

	if count := e.rateChanged(); count != 1 {
		t.Fatalf("activity rows = %d, want 1 (no duplicate on an unchanged retry)", count)
	}
}

// TestPutRateHandler_RecordsActivity proves setting a rate is recorded
// in the activity ledger with the Staff member who did it -- #966's AC.
func TestPutRateHandler_RecordsActivity(t *testing.T) {
	e := ownerEnv(t)

	e.putRate(fixture.KindPostpartum, 12000, http.StatusOK)

	if actor := testdb.Scalar[string](t, e.DB.Admin, `SELECT actor_staff_id `+rateChangedRows, e.Practice.ID); actor != e.Staff.ID {
		t.Fatalf("actor_staff_id = %q, want %q", actor, e.Staff.ID)
	}
}
