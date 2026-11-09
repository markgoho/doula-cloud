package practicerate_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/practicerate"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// env is what a test in this package starts from: a Practice with the
// rate routes mounted and one Staff member at the screen, an employee
// Doula unless a test says otherwise.
type env struct {
	*fixture.Scene
}

// newEnv mounts this package's routes through practicerate.Mount, the same
// call main.go makes on the real GatedRouter and idempotency.Router.
func newEnv(t *testing.T, opts ...fixture.SceneOption) *env {
	t.Helper()
	return &env{fixture.NewScene(t, fixture.Routes(practicerate.Mount), opts...)}
}

// The two Doulas no rate write is open to.
const (
	employeeDoulaName   = "employee doula"
	contractorDoulaName = "contractor doula"
)

var (
	employeeDoula   = []fixture.StaffOption{fixture.Doula()}
	contractorDoula = []fixture.StaffOption{fixture.Doula(), fixture.Contractor()}
)

func signedInAs(opts ...fixture.StaffOption) fixture.SceneOption {
	return fixture.SignedInAs(opts...)
}

// ownerEnv is an env whose Staff member is an Owner.
func ownerEnv(t *testing.T) *env {
	t.Helper()
	return newEnv(t, signedInAs(fixture.Owner()))
}

func (e *env) ratesURL() string { return e.PracticeURL("/rates") }

func (e *env) rateURL(kind string) string { return e.ratesURL() + "/" + kind }

// rates reads the rate card as the Staff member at the screen.
func (e *env) rates() practicerate.RatesResponse {
	e.T.Helper()
	return reqtest.Expect[practicerate.RatesResponse](e.T, http.MethodGet, e.ratesURL(), http.StatusOK, e.SignedIn)
}

// seedRate seeds a birth rate without going through the handlers.
func (e *env) seedRate(amountCents int64) {
	e.T.Helper()
	testdb.SeedPracticeRate(e.T, e.DB, e.Practice.ID, fixture.KindBirth, amountCents)
}

// putRate sets kind's rate and requires it to be accepted.
func (e *env) putRate(kind string, amountCents int64) {
	e.T.Helper()
	reqtest.ExpectStatus(e.T, http.MethodPut, e.rateURL(kind), http.StatusOK, e.SignedIn, reqtest.JSON(practicerate.RatePutRequest{AmountCents: amountCents}))
}

// deleteRate removes kind's rate and requires want.
func (e *env) deleteRate(kind string, want int) {
	e.T.Helper()
	reqtest.ExpectStatus(e.T, http.MethodDelete, e.rateURL(kind), want, e.SignedIn)
}

// amountOf is kind's amount on the rate card, nil when none is set.
func (e *env) amountOf(kind string) *int64 {
	e.T.Helper()
	rates := e.rates().Rates
	for _, rate := range rates {
		if rate.Kind == kind {
			return rate.AmountCents
		}
	}
	e.T.Fatalf("rates = %v, want a %s entry", rates, kind)
	return nil
}

const rateChangedRows = `FROM activity WHERE subject_kind = 'practice' AND subject_id = $1 AND action = 'practice_rate_changed'`

// rateChanged counts the Practice's practice_rate_changed rows.
func (e *env) rateChanged() int {
	e.T.Helper()
	return testdb.Scalar[int](e.T, e.DB.Admin, `SELECT count(*) `+rateChangedRows, e.Practice.ID)
}
