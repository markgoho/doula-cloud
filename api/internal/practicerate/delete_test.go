package practicerate_test

import (
	"encoding/json/v2"
	"net/http"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/practicerate"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// TestDeleteRateHandler_Refusals proves #2080's AC that removing a rate
// sits behind the same role guard as setting one, and that a kind outside
// birth/postpartum is refused before any write.
func TestDeleteRateHandler_Refusals(t *testing.T) {
	cases := []struct {
		name string
		opts []fixture.StaffOption
		kind string
		want int
	}{
		{employeeDoulaName, employeeDoula, fixture.KindBirth, http.StatusForbidden},
		{contractorDoulaName, contractorDoula, fixture.KindBirth, http.StatusForbidden},
		{"invalid kind", []fixture.StaffOption{fixture.Owner()}, "insurance", http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(tc.opts...))
			e.seedRate(240000)

			e.deleteRate(tc.kind, tc.want)
		})
	}
}

// TestDeleteRateHandler_AdminRemovesRateAndRecordsIt proves #2080's
// first two ACs: an Admin removes a rate, the rate card reads "no rate"
// for that kind again, and the removal is recorded with who did it and
// the amount before.
func TestDeleteRateHandler_AdminRemovesRateAndRecordsIt(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Admin()))
	e.seedRate(240000)

	removed := reqtest.Expect[practicerate.Rate](t, http.MethodDelete, e.rateURL(fixture.KindBirth), http.StatusOK, e.SignedIn)
	if removed.Kind != fixture.KindBirth || removed.AmountCents != nil {
		t.Fatalf("response = %+v, want birth with a null amountCents", removed)
	}
	for _, rate := range e.rates().Rates {
		if rate.AmountCents != nil {
			t.Fatalf("rate %q amountCents = %v, want nil after removal", rate.Kind, *rate.AmountCents)
		}
	}

	var diff struct {
		Kind              string `json:"kind"`
		AmountCentsBefore *int64 `json:"amountCentsBefore"`
		AmountCentsAfter  *int64 `json:"amountCentsAfter"`
	}
	if actor := testdb.Scalar[string](t, e.DB.Admin, `SELECT actor_staff_id `+rateChangedRows, e.Practice.ID); actor != e.Staff.ID {
		t.Fatalf("actor_staff_id = %q, want %q", actor, e.Staff.ID)
	}
	diffJSON := testdb.Scalar[[]byte](t, e.DB.Admin, `SELECT diff `+rateChangedRows, e.Practice.ID)
	if err := json.Unmarshal(diffJSON, &diff); err != nil {
		t.Fatalf("decode diff: %v", err)
	}
	if diff.Kind != fixture.KindBirth || diff.AmountCentsBefore == nil || *diff.AmountCentsBefore != 240000 || diff.AmountCentsAfter != nil {
		t.Fatalf("diff = %s, want birth from 240000 to null", diffJSON)
	}
}

// TestDeleteRateHandler_RepeatRecordsOnce proves a retry, and a removal
// of a rate that was never set, write and record nothing new.
func TestDeleteRateHandler_RepeatRecordsOnce(t *testing.T) {
	e := ownerEnv(t)
	e.seedRate(240000)

	for _, kind := range []string{fixture.KindBirth, fixture.KindBirth, fixture.KindPostpartum} {
		e.deleteRate(kind, http.StatusOK)
	}

	if count := e.rateChanged(); count != 1 {
		t.Fatalf("activity rows = %d, want 1 (only the removal that removed something)", count)
	}
}
