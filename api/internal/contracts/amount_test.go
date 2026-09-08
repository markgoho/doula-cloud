package contracts_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/testdb"
)

func contractAmountURL(srv *httptest.Server, practiceID, engagementID string) string {
	return contractURL(srv, practiceID, engagementID) + "/amount"
}

func putContractAmountRaw(t *testing.T, srv *httptest.Server, session, practiceID, engagementID string, body []byte) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut, contractAmountURL(srv, practiceID, engagementID), bytes.NewReader(body))
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

func putContractAmount(t *testing.T, srv *httptest.Server, session, practiceID, engagementID string, amountCents int64) *http.Response {
	t.Helper()
	payload, err := json.Marshal(contracts.PutContractAmountRequest{AmountCents: amountCents})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	return putContractAmountRaw(t, srv, session, practiceID, engagementID, payload)
}

// TestPostContractHandler_NoRateSet proves #967's AC: creating a Contract
// on a Practice with no rate set for the Engagement's kind fails in a
// way a person can act on, naming the missing kind -- a Contract
// Template exists (so this refusal isn't NoTemplate's), but no
// practice_rates row does.
func TestPostContractHandler_NoRateSet(t *testing.T) {
	db := testdb.New(t)
	const uid = "post-no-rate"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContractTemplate(t, db, practiceID, mergeFieldProse)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := postContract(t, srv, session, practiceID, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
	var out apierr.APIError
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Code != string(apierr.CodeFailedPrecondition) {
		t.Fatalf("code = %q, want %q", out.Code, apierr.CodeFailedPrecondition)
	}
	if !strings.Contains(out.Message, "birth") {
		t.Fatalf("message = %q, want it to name the missing kind %q", out.Message, "birth")
	}
}

// TestPutContractAmountHandler_NonOwnerAdminForbidden table-drives #967's
// AC that the API refuses a Doula, employee or contractor.
func TestPutContractAmountHandler_NonOwnerAdminForbidden(t *testing.T) {
	cases := []struct {
		name           string
		employmentType string
	}{
		{"employee doula", "employee"},
		{"contractor doula", "contractor"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testdb.New(t)
			uid := "amount-forbidden-" + tc.name
			practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, tc.employmentType)
			_, engagementID := testdb.SeedEngagement(t, db, practiceID)
			seedContract(t, db, engagementID, statusDraft, mergeFieldProse)

			srv, session := newContractServer(t, db, uid)
			defer srv.Close()

			resp := putContractAmount(t, srv, session, practiceID, engagementID, 20000)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusForbidden {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
			}
		})
	}
}

// TestPutContractAmountHandler_AdminAllowed proves an Admin, not only an
// Owner, may override a Contract's amount.
func TestPutContractAmountHandler_AdminAllowed(t *testing.T) {
	db := testdb.New(t)
	const uid = "amount-admin-allowed"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{adminRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, statusDraft, mergeFieldProse)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := putContractAmount(t, srv, session, practiceID, engagementID, 20000)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

// TestPutContractAmountHandler_Success proves an Owner's override
// persists, round-trips through GET (the "price" merge field renders the
// new amount), and is recorded in the activity ledger with before/after
// amounts and the acting Staff member.
func TestPutContractAmountHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const uid = "amount-success"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, statusDraft, mergeFieldProse)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := putContractAmount(t, srv, session, practiceID, engagementID, 30000)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out contracts.ContractAmountResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.AmountCents != 30000 {
		t.Fatalf("amountCents = %d, want 30000", out.AmountCents)
	}

	getResp := getContract(t, srv, session, practiceID, engagementID)
	defer getResp.Body.Close()
	var getOut contracts.ContractResponse
	if err := json.NewDecoder(getResp.Body).Decode(&getOut); err != nil {
		t.Fatalf("decode GET response: %v", err)
	}
	if getOut.Values[priceKey] != "$300.00" {
		t.Fatalf("values[price] after override = %q, want %q", getOut.Values[priceKey], "$300.00")
	}

	var action, actorStaffID string
	var diff []byte
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT action, actor_staff_id, diff FROM activity WHERE subject_kind = 'engagement' AND subject_id = $1 AND action = 'contract_amount_overridden'`,
		engagementID,
	).Scan(&action, &actorStaffID, &diff); err != nil {
		t.Fatalf("query activity: %v", err)
	}
	if actorStaffID != staffID {
		t.Fatalf("actor_staff_id = %q, want %q", actorStaffID, staffID)
	}
	var parsedDiff struct {
		AmountCentsBefore int64 `json:"amountCentsBefore"`
		AmountCentsAfter  int64 `json:"amountCentsAfter"`
	}
	if err := json.Unmarshal(diff, &parsedDiff); err != nil {
		t.Fatalf("unmarshal diff: %v", err)
	}
	if parsedDiff.AmountCentsBefore != 15000 || parsedDiff.AmountCentsAfter != 30000 {
		t.Fatalf("diff = %+v, want before=15000 after=30000", parsedDiff)
	}

	// #968's AC: an overridden Contract keeps the override and never
	// re-derives when a later rate change comes through -- amount_overridden
	// is the column practicerate.PutRateHandler's reprice pass excludes on.
	var overridden bool
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT amount_overridden FROM contracts WHERE engagement_id = $1`, engagementID,
	).Scan(&overridden); err != nil {
		t.Fatalf("query amount_overridden: %v", err)
	}
	if !overridden {
		t.Fatalf("amount_overridden = false, want true after an Owner/Admin override")
	}
	if getOut.AmountChangedAt == nil {
		t.Fatalf("GET amountChangedAt = nil, want set -- an Owner/Admin can see the price changed and when (#968)")
	}
}

// TestPutContractAmountHandler_RepeatedIdenticalValueRecordsNoNewActivity
// proves the idempotent-PUT shape: a retry with the same amount writes
// no second activity row, mirroring practicerate's own repeat case.
func TestPutContractAmountHandler_RepeatedIdenticalValueRecordsNoNewActivity(t *testing.T) {
	db := testdb.New(t)
	const uid = "amount-repeat"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, statusDraft, mergeFieldProse)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	first := putContractAmount(t, srv, session, practiceID, engagementID, 15000)
	defer first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first PUT status = %d, want %d", first.StatusCode, http.StatusOK)
	}

	second := putContractAmount(t, srv, session, practiceID, engagementID, 15000)
	defer second.Body.Close()
	if second.StatusCode != http.StatusOK {
		t.Fatalf("second PUT status = %d, want %d", second.StatusCode, http.StatusOK)
	}

	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM activity WHERE subject_kind = 'engagement' AND subject_id = $1 AND action = 'contract_amount_overridden'`,
		engagementID,
	).Scan(&count); err != nil {
		t.Fatalf("count activity rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("activity rows = %d, want 0 (the amount never actually changed, 15000 == 15000)", count)
	}
}

// TestPutContractAmountHandler_NonPositiveAmountRejected table-drives
// zero and negative amounts, both invalid.
func TestPutContractAmountHandler_NonPositiveAmountRejected(t *testing.T) {
	cases := []struct {
		name        string
		amountCents int64
	}{
		{"zero", 0},
		{"negative", -100},
	}

	db := testdb.New(t)
	const uid = "amount-non-positive"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, statusDraft, mergeFieldProse)
	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := putContractAmount(t, srv, session, practiceID, engagementID, tc.amountCents)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
			}
		})
	}
}

// TestPutContractAmountHandler_InvalidBody proves malformed JSON is
// refused.
func TestPutContractAmountHandler_InvalidBody(t *testing.T) {
	db := testdb.New(t)
	const uid = "amount-invalid-body"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, statusDraft, mergeFieldProse)
	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := putContractAmountRaw(t, srv, session, practiceID, engagementID, []byte("not json"))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPutContractAmountHandler_InvalidEngagementID proves a malformed
// path segment is refused before any query runs.
func TestPutContractAmountHandler_InvalidEngagementID(t *testing.T) {
	db := testdb.New(t)
	const uid = "amount-invalid-engagement"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := putContractAmount(t, srv, session, practiceID, "not-a-uuid", 20000)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPutContractAmountHandler_EngagementNotFound proves an Engagement
// belonging to another Practice 404s rather than leaking that it exists.
func TestPutContractAmountHandler_EngagementNotFound(t *testing.T) {
	db := testdb.New(t)
	const uid = "amount-engagement-not-found"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	otherPracticeID := testdb.SeedPractice(t, db, "Other Practice")
	_, otherEngagementID := testdb.SeedEngagement(t, db, otherPracticeID)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := putContractAmount(t, srv, session, practiceID, otherEngagementID, 20000)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestPutContractAmountHandler_NoContractNotFound proves overriding an
// Engagement with no Contract at all 404s.
func TestPutContractAmountHandler_NoContractNotFound(t *testing.T) {
	db := testdb.New(t)
	const uid = "amount-no-contract"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := putContractAmount(t, srv, session, practiceID, engagementID, 20000)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestPutContractAmountHandler_NotDraftRejected table-drives #967's AC
// directly: "a signed Contract's amount never changes for any reason" --
// TransitionOverrideAmount refuses everything but Draft, so sent, signed
// and voided all 409.
func TestPutContractAmountHandler_NotDraftRejected(t *testing.T) {
	cases := []string{"sent", "signed", "voided"}

	for _, status := range cases {
		t.Run(status, func(t *testing.T) {
			db := testdb.New(t)
			uid := "amount-not-draft-" + status
			practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
			_, engagementID := testdb.SeedEngagement(t, db, practiceID)
			seedContract(t, db, engagementID, status, mergeFieldProse)

			srv, session := newContractServer(t, db, uid)
			defer srv.Close()

			resp := putContractAmount(t, srv, session, practiceID, engagementID, 20000)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusConflict {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
			}
		})
	}
}
