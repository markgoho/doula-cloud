package practicerate_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/practicerate"
	"doula-cloud/api/internal/testdb"
)

// birthKind is named once so golangci-lint's goconst check doesn't see
// the repeated literal across this package's tests.
const birthKind = "birth"

func deleteRate(t *testing.T, srv *httptest.Server, session, practiceID, kind string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodDelete, srv.URL+"/api/practices/"+practiceID+"/rates/"+kind, nil)
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

// countRateChanged counts practiceID's practice_rate_changed rows.
func countRateChanged(t *testing.T, db *testdb.DB, practiceID string) int {
	t.Helper()
	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM activity WHERE subject_kind = 'practice' AND subject_id = $1 AND action = 'practice_rate_changed'`,
		practiceID,
	).Scan(&count); err != nil {
		t.Fatalf("count activity rows: %v", err)
	}
	return count
}

// TestDeleteRateHandler_NonOwnerAdminForbidden proves #2080's AC that
// removing a rate sits behind the same role guard as setting one.
func TestDeleteRateHandler_NonOwnerAdminForbidden(t *testing.T) {
	for _, employmentType := range []string{employeeType, contractorType} {
		t.Run(employmentType, func(t *testing.T) {
			db := testdb.New(t)
			uid := "delete-forbidden-" + employmentType
			practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, employmentType)
			seedRate(t, db, practiceID, 240000)

			srv, session := newRateServer(t, db, uid)
			defer srv.Close()

			resp := deleteRate(t, srv, session, practiceID, birthKind)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusForbidden {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
			}
		})
	}
}

// TestDeleteRateHandler_InvalidKind proves a kind outside
// birth/postpartum is refused before any write.
func TestDeleteRateHandler_InvalidKind(t *testing.T) {
	db := testdb.New(t)
	const uid = "delete-invalid-kind"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)

	srv, session := newRateServer(t, db, uid)
	defer srv.Close()

	resp := deleteRate(t, srv, session, practiceID, "insurance")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestDeleteRateHandler_AdminRemovesRateAndRecordsIt proves #2080's
// first two ACs: an Admin removes a rate, the rate card reads "no rate"
// for that kind again, and the removal is recorded with who did it and
// the amount before.
func TestDeleteRateHandler_AdminRemovesRateAndRecordsIt(t *testing.T) {
	db := testdb.New(t)
	const uid = "delete-admin"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{adminRole}, employeeType)
	seedRate(t, db, practiceID, 240000)

	srv, session := newRateServer(t, db, uid)
	defer srv.Close()

	resp := deleteRate(t, srv, session, practiceID, birthKind)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var removed practicerate.Rate
	if err := json.NewDecoder(resp.Body).Decode(&removed); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if removed.Kind != birthKind || removed.AmountCents != nil {
		t.Fatalf("response = %+v, want birth with a null amountCents", removed)
	}

	get := getRates(t, srv, session, practiceID)
	defer get.Body.Close()
	var out practicerate.RatesResponse
	if err := json.NewDecoder(get.Body).Decode(&out); err != nil {
		t.Fatalf("decode rates: %v", err)
	}
	for _, rate := range out.Rates {
		if rate.AmountCents != nil {
			t.Fatalf("rate %q amountCents = %v, want nil after removal", rate.Kind, *rate.AmountCents)
		}
	}

	var actorStaffID string
	var diff struct {
		Kind              string `json:"kind"`
		AmountCentsBefore *int64 `json:"amountCentsBefore"`
		AmountCentsAfter  *int64 `json:"amountCentsAfter"`
	}
	var diffJSON []byte
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT actor_staff_id, diff FROM activity WHERE subject_kind = 'practice' AND subject_id = $1 AND action = 'practice_rate_changed'`,
		practiceID,
	).Scan(&actorStaffID, &diffJSON); err != nil {
		t.Fatalf("query activity: %v", err)
	}
	if actorStaffID != staffID {
		t.Fatalf("actor_staff_id = %q, want %q", actorStaffID, staffID)
	}
	if err := json.Unmarshal(diffJSON, &diff); err != nil {
		t.Fatalf("decode diff: %v", err)
	}
	if diff.Kind != birthKind || diff.AmountCentsBefore == nil || *diff.AmountCentsBefore != 240000 || diff.AmountCentsAfter != nil {
		t.Fatalf("diff = %s, want birth from 240000 to null", diffJSON)
	}
}

// TestDeleteRateHandler_RepeatRecordsOnce proves a retry, and a removal
// of a rate that was never set, write and record nothing new.
func TestDeleteRateHandler_RepeatRecordsOnce(t *testing.T) {
	db := testdb.New(t)
	const uid = "delete-repeat"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)
	seedRate(t, db, practiceID, 240000)

	srv, session := newRateServer(t, db, uid)
	defer srv.Close()

	for _, kind := range []string{birthKind, birthKind, "postpartum"} {
		resp := deleteRate(t, srv, session, practiceID, kind)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("DELETE %s status = %d, want %d", kind, resp.StatusCode, http.StatusOK)
		}
	}

	if count := countRateChanged(t, db, practiceID); count != 1 {
		t.Fatalf("activity rows = %d, want 1 (only the removal that removed something)", count)
	}
}
