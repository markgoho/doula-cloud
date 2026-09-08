package practicerate_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/practicerate"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

const doulaRole = "doula"

// ownerRole is named once so golangci-lint's goconst check doesn't see
// repeated "owner" literals across this package's test surface, mirroring
// contracts/template_test.go's own const.
const ownerRole = "owner"
const adminRole = "admin"

// employeeType and contractorType are the two employment_type values
// this package's tests seed a Membership with -- named once so
// golangci-lint's goconst check doesn't see the repeated literal across
// this package's table-driven role tests.
const employeeType = "employee"
const contractorType = "contractor"

// seedRate seeds a practice_rates row directly (bypassing the handlers
// under test), always against the "birth" kind -- the only kind this
// package's tests seed directly; a test proving something about
// "postpartum" sets it through PutRateHandler instead.
func seedRate(t *testing.T, db *testdb.DB, practiceID string, amountCents int64) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_rates (practice_id, kind, amount_cents) VALUES ($1, 'birth'::engagement_kind, $2)`,
		practiceID, amountCents,
	); err != nil {
		t.Fatalf("seed rate: %v", err)
	}
}

func newRateServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	practicerate.Mount(g, ir)
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

func getRates(t *testing.T, srv *httptest.Server, session, practiceID string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/api/practices/"+practiceID+"/rates", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func putRateRaw(t *testing.T, srv *httptest.Server, session, practiceID, kind string, body []byte) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut, srv.URL+"/api/practices/"+practiceID+"/rates/"+kind, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func putRate(t *testing.T, srv *httptest.Server, session, practiceID, kind string, amountCents int64) *http.Response {
	t.Helper()
	payload, err := json.Marshal(practicerate.RatePutRequest{AmountCents: amountCents})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	return putRateRaw(t, srv, session, practiceID, kind, payload)
}

// TestGetRatesHandler_NoRateSetIsValid proves an unset rate card reads as
// both kinds present with a null amountCents, not a 404 or an error --
// #966's AC that a Practice with no rate set is a valid state.
func TestGetRatesHandler_NoRateSetIsValid(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-no-rate"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, employeeType)

	srv, session := newRateServer(t, db, uid)
	defer srv.Close()

	resp := getRates(t, srv, session, practiceID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out practicerate.RatesResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(out.Rates) != 2 {
		t.Fatalf("rates = %v, want both kinds present", out.Rates)
	}
	for _, rate := range out.Rates {
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
		name           string
		roles          []string
		employmentType string
	}{
		{"employee doula", []string{doulaRole}, employeeType},
		{"contractor doula", []string{doulaRole}, contractorType},
		{"employee admin", []string{adminRole}, employeeType},
		{"contractor admin", []string{adminRole}, contractorType},
		{"employee owner", []string{ownerRole}, employeeType},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testdb.New(t)
			uid := "get-any-role-" + tc.name
			practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, tc.roles, tc.employmentType)
			seedRate(t, db, practiceID, 15000)

			srv, session := newRateServer(t, db, uid)
			defer srv.Close()

			resp := getRates(t, srv, session, practiceID)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
			}
			var out practicerate.RatesResponse
			if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			var found bool
			for _, rate := range out.Rates {
				if rate.Kind == "birth" {
					found = true
					if rate.AmountCents == nil || *rate.AmountCents != 15000 {
						t.Fatalf("birth rate = %v, want 15000", rate.AmountCents)
					}
				}
			}
			if !found {
				t.Fatalf("rates = %v, want a birth entry", out.Rates)
			}
		})
	}
}

// TestPutRateHandler_NonOwnerAdminForbidden table-drives #966's AC that
// the API refuses a Doula, employee or contractor.
func TestPutRateHandler_NonOwnerAdminForbidden(t *testing.T) {
	cases := []struct {
		name           string
		employmentType string
	}{
		{"employee doula", employeeType},
		{"contractor doula", contractorType},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testdb.New(t)
			uid := "put-forbidden-" + tc.name
			practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, tc.employmentType)

			srv, session := newRateServer(t, db, uid)
			defer srv.Close()

			resp := putRate(t, srv, session, practiceID, "birth", 15000)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusForbidden {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
			}
		})
	}
}

// TestPutRateHandler_AdminAllowed proves an Admin, not only an Owner, may
// set a rate -- #966's AC names both seats.
func TestPutRateHandler_AdminAllowed(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-admin-allowed"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{adminRole}, employeeType)

	srv, session := newRateServer(t, db, uid)
	defer srv.Close()

	resp := putRate(t, srv, session, practiceID, "birth", 15000)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

// TestPutRateHandler_InvalidKind proves a kind outside birth/postpartum
// is refused before any write.
func TestPutRateHandler_InvalidKind(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-invalid-kind"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)

	srv, session := newRateServer(t, db, uid)
	defer srv.Close()

	resp := putRate(t, srv, session, practiceID, "insurance", 15000)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPutRateHandler_InvalidBody proves malformed JSON is refused.
func TestPutRateHandler_InvalidBody(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-invalid-body"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)

	srv, session := newRateServer(t, db, uid)
	defer srv.Close()

	resp := putRateRaw(t, srv, session, practiceID, "birth", []byte("not json"))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPutRateHandler_NonPositiveAmountRejected table-drives zero and
// negative amounts, both invalid.
func TestPutRateHandler_NonPositiveAmountRejected(t *testing.T) {
	cases := []struct {
		name        string
		amountCents int64
	}{
		{"zero", 0},
		{"negative", -100},
	}

	db := testdb.New(t)
	const uid = "put-non-positive"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)
	srv, session := newRateServer(t, db, uid)
	defer srv.Close()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := putRate(t, srv, session, practiceID, "birth", tc.amountCents)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
			}
		})
	}
}

// TestPutRateHandler_Success proves a set round-trips through GET, and
// that a second PUT with a new amount replaces it -- both kinds set
// independently.
func TestPutRateHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-success"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)

	srv, session := newRateServer(t, db, uid)
	defer srv.Close()

	putResp := putRate(t, srv, session, practiceID, "birth", 15000)
	defer putResp.Body.Close()
	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("PUT status = %d, want %d", putResp.StatusCode, http.StatusOK)
	}
	var putOut practicerate.Rate
	if err := json.NewDecoder(putResp.Body).Decode(&putOut); err != nil {
		t.Fatalf("decode PUT response: %v", err)
	}
	if putOut.AmountCents == nil || *putOut.AmountCents != 15000 {
		t.Fatalf("PUT amountCents = %v, want 15000", putOut.AmountCents)
	}

	changeResp := putRate(t, srv, session, practiceID, "birth", 20000)
	defer changeResp.Body.Close()
	if changeResp.StatusCode != http.StatusOK {
		t.Fatalf("second PUT status = %d, want %d", changeResp.StatusCode, http.StatusOK)
	}

	getResp := getRates(t, srv, session, practiceID)
	defer getResp.Body.Close()
	var getOut practicerate.RatesResponse
	if err := json.NewDecoder(getResp.Body).Decode(&getOut); err != nil {
		t.Fatalf("decode GET response: %v", err)
	}
	for _, rate := range getOut.Rates {
		switch rate.Kind {
		case "birth":
			if rate.AmountCents == nil || *rate.AmountCents != 20000 {
				t.Fatalf("birth rate after replace = %v, want 20000", rate.AmountCents)
			}
		case "postpartum":
			if rate.AmountCents != nil {
				t.Fatalf("postpartum rate = %v, want nil (never set)", *rate.AmountCents)
			}
		}
	}
}

// TestPutRateHandler_RepeatedIdenticalValueRecordsNoNewActivity proves
// the idempotent-PUT shape: a retry with the same amount writes no
// second activity row, mirroring
// staffauth.PutMFARequiredHandler_test's own repeat case.
func TestPutRateHandler_RepeatedIdenticalValueRecordsNoNewActivity(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-repeat"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)

	srv, session := newRateServer(t, db, uid)
	defer srv.Close()

	first := putRate(t, srv, session, practiceID, "birth", 15000)
	defer first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first PUT status = %d, want %d", first.StatusCode, http.StatusOK)
	}

	second := putRate(t, srv, session, practiceID, "birth", 15000)
	defer second.Body.Close()
	if second.StatusCode != http.StatusOK {
		t.Fatalf("second PUT status = %d, want %d", second.StatusCode, http.StatusOK)
	}

	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM activity WHERE subject_kind = 'practice' AND subject_id = $1 AND action = 'practice_rate_changed'`,
		practiceID,
	).Scan(&count); err != nil {
		t.Fatalf("count activity rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("activity rows = %d, want 1 (no duplicate on an unchanged retry)", count)
	}
}

// TestPutRateHandler_RecordsActivity proves setting a rate is recorded
// in the activity ledger with the Staff member who did it -- #966's AC.
func TestPutRateHandler_RecordsActivity(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-records-activity"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)

	srv, session := newRateServer(t, db, uid)
	defer srv.Close()

	resp := putRate(t, srv, session, practiceID, "postpartum", 12000)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var action, actorStaffID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT action, actor_staff_id FROM activity WHERE subject_kind = 'practice' AND subject_id = $1 AND action = 'practice_rate_changed'`,
		practiceID,
	).Scan(&action, &actorStaffID); err != nil {
		t.Fatalf("query activity: %v", err)
	}
	if action != "practice_rate_changed" {
		t.Fatalf("action = %q, want practice_rate_changed", action)
	}
	if actorStaffID != staffID {
		t.Fatalf("actor_staff_id = %q, want %q", actorStaffID, staffID)
	}
}
