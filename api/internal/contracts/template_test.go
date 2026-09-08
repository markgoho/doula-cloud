package contracts_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/objectstore"
	"doula-cloud/api/internal/push"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

const doulaRole = "doula"

// ownerRole is named once so golangci-lint's goconst check doesn't see
// repeated "owner" literals across this package's test surface.
const ownerRole = "owner"

// seedContractTemplate seeds a Contract Template row directly (bypassing
// the handlers under test). Stays local under this name rather than
// testdb: plans/template_test.go's own seedTemplate writes plan_templates
// (fields JSON), and clientfieldtemplate's writes client_field_templates
// -- three different tables sharing one generic old name.
func seedContractTemplate(t *testing.T, db *testdb.DB, practiceID, prose string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO contract_templates (practice_id, prose) VALUES ($1, $2)`,
		practiceID, prose,
	); err != nil {
		t.Fatalf("seed template: %v", err)
	}
}

// seedContract seeds a Contract row directly (bypassing the handlers
// under test), with an explicit status so tests can exercise a
// non-'draft' Contract that PutContractHandler must reject -- a state no
// endpoint in this ticket can reach on its own. merge_field_values is
// left at its NOT NULL DEFAULT '{}'::jsonb, matching every caller. Stays
// local: testdb has no "seed a Contract in an arbitrary status" export,
// and this package is the only one that needs to force a non-draft row.
func seedContract(t *testing.T, db *testdb.DB, engagementID, status, prose string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO contracts (engagement_id, status, prose) VALUES ($1, $2::contract_status, $3)`,
		engagementID, status, prose,
	); err != nil {
		t.Fatalf("seed contract: %v", err)
	}
}

// seedContractWithValues seeds a 'draft' Contract row, at mergeFieldProse
// (every caller's own prose), with an explicit merge field Values map,
// for a test that sends the seeded Contract through
// PostSendContractHandler -- #258's completeness check refuses a Draft
// whose values are still at seedContract's default `{}`, so a caller
// proving Send succeeds needs every key mergeFieldProse parses filled
// in. Always 'draft': every caller here is proving something about
// Send, which only ever accepts a Draft in the first place.
func seedContractWithValues(t *testing.T, db *testdb.DB, engagementID string, values contracts.MergeFieldValues) {
	t.Helper()
	valuesJSON, err := json.Marshal(values)
	if err != nil {
		t.Fatalf("marshal values: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO contracts (engagement_id, status, prose, merge_field_values) VALUES ($1, 'draft'::contract_status, $2, $3)`,
		engagementID, mergeFieldProse, valuesJSON,
	); err != nil {
		t.Fatalf("seed contract: %v", err)
	}
}

// seedSignedContract seeds a 'signed' Contract row directly (prose fixed
// at mergeFieldProse, the same prose every caller needs), with
// signed_pdf_object_path set -- exercising GetSignedContractPDFHandler /
// ClientGetSignedContractPDFHandler's DB read without going through the
// full Sign transition. It returns the row's id and the object-store key
// its Signed PDF belongs at; the key is derived from the id (#299), so
// the caller cannot compute it before the INSERT and the helper hands it
// back rather than taking it. Callers separately Put matching bytes into
// the objectstore.ObjectStore the test server was built with, at that
// key, unless the test wants the "PDF row found but object missing" case.
func seedSignedContract(t *testing.T, db *testdb.DB, engagementID string) (contractID, pdfObjectPath string) {
	t.Helper()
	return seedSignedContractRow(t, db, engagementID, contracts.StatusSigned, "0 seconds")
}

// seedPriorSignedContract seeds an *older* Contract on the same
// Engagement that was signed and has since been voided -- the
// void-then-recreate history #72's partial unique index permits, and the
// only way an Engagement comes to hold two rows with a stored Signed PDF.
// created_at is pushed an hour back explicitly rather than left to a
// second now(): "the most recently created signed Contract" must be
// decided by the fixture, not by two clock reads a microsecond apart.
func seedPriorSignedContract(t *testing.T, db *testdb.DB, engagementID string) (contractID, pdfObjectPath string) {
	t.Helper()
	return seedSignedContractRow(t, db, engagementID, contracts.StatusVoided, "1 hour")
}

// seedSignedContractRow is the shared body of the two helpers above. It
// inserts, reads the generated id back, and only then writes the object
// path, because contracts.SignedPDFObjectPath is keyed on that id. age is
// how far behind the DB clock the row's created_at sits, as a Postgres
// interval -- the fixture, not two clock reads, decides which of an
// Engagement's Contracts is the most recent one.
func seedSignedContractRow(t *testing.T, db *testdb.DB, engagementID string, status contracts.Status, age string) (contractID, pdfObjectPath string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO contracts (engagement_id, status, prose, created_at)
		 VALUES ($1, $2::contract_status, $3, now() - $4::interval) RETURNING id`,
		engagementID, string(status), mergeFieldProse, age,
	).Scan(&contractID); err != nil {
		t.Fatalf("seed signed contract: %v", err)
	}
	pdfObjectPath = contracts.SignedPDFObjectPath(engagementID, contractID)
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE contracts SET signed_pdf_object_path = $1 WHERE id = $2`,
		pdfObjectPath, contractID,
	); err != nil {
		t.Fatalf("seed signed contract pdf path: %v", err)
	}
	return contractID, pdfObjectPath
}

// voidContractRow moves a seeded signed Contract to 'voided' directly,
// without going through PostVoidContractHandler -- a test about the
// *read* should not depend on the write route's own role gate, and the
// row-level effect (status moves, signed_pdf_object_path does not) is
// exactly what PostVoidContractHandler produces, asserted in its own
// test.
func voidContractRow(t *testing.T, db *testdb.DB, contractID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE contracts SET status = $1::contract_status WHERE id = $2`,
		string(contracts.StatusVoided), contractID,
	); err != nil {
		t.Fatalf("void contract row: %v", err)
	}
}

func newContractServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	return newContractServerWithPusherAndStore(t, db, uid, push.NewFakePusher(), objectstore.NewMemoryStore())
}

// newContractServerWithPusher mirrors newContractServer but lets the
// caller inject pusher, so a test can inspect what Send triggers --
// mirrors message/handlers_test.go's newServerWithPusher.
func newContractServerWithPusher(t *testing.T, db *testdb.DB, uid string, pusher push.Pusher) (srv *httptest.Server, session string) {
	t.Helper()
	return newContractServerWithPusherAndStore(t, db, uid, pusher, objectstore.NewMemoryStore())
}

// newContractServerWithStore mirrors newContractServer but lets the
// caller inject store, so a test can seed what the Signed PDF endpoint
// reads back.
func newContractServerWithStore(t *testing.T, db *testdb.DB, uid string, store objectstore.ObjectStore) (srv *httptest.Server, session string) {
	t.Helper()
	return newContractServerWithPusherAndStore(t, db, uid, push.NewFakePusher(), store)
}

func newContractServerWithPusherAndStore(t *testing.T, db *testdb.DB, uid string, pusher push.Pusher, store objectstore.ObjectStore) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	contracts.Mount(g, ir, db.App, store, pusher)
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

func getTemplate(t *testing.T, srv *httptest.Server, session string, practiceID string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/api/practices/"+practiceID+"/contract-template", nil)
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

func putTemplateRaw(t *testing.T, srv *httptest.Server, session string, practiceID string, body []byte) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut, srv.URL+"/api/practices/"+practiceID+"/contract-template", bytes.NewReader(body))
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

func putTemplate(t *testing.T, srv *httptest.Server, session string, practiceID string, body contracts.TemplateResponse) *http.Response {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	return putTemplateRaw(t, srv, session, practiceID, payload)
}

func TestGetTemplateHandler_NotFound(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-not-found"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := getTemplate(t, srv, session, practiceID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestGetTemplateHandler_AnyMemberAllowed proves a non-Owner member (a
// doula) can read the template -- only PUT is Owner-gated.
func TestGetTemplateHandler_AnyMemberAllowed(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-any-member"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	seedContractTemplate(t, db, practiceID, "Some prose with {{client_name}}")

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := getTemplate(t, srv, session, practiceID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var out contracts.TemplateResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Prose != "Some prose with {{client_name}}" {
		t.Fatalf("prose = %q, want the seeded prose", out.Prose)
	}
}

func TestPutTemplateHandler_NonOwnerForbidden(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-non-owner"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := putTemplate(t, srv, session, practiceID, contracts.TemplateResponse{Prose: "Some prose"})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestPutTemplateHandler_InvalidBody(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-invalid-body"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := putTemplateRaw(t, srv, session, practiceID, []byte("not json"))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestPutTemplateHandler_BlankProseRejected table-drives an empty and a
// whitespace-only prose, both of which normalize to "" after trimming.
func TestPutTemplateHandler_BlankProseRejected(t *testing.T) {
	cases := []struct {
		name  string
		prose string
	}{
		{"empty", ""},
		{"whitespace only", "   \n\t  "},
	}

	db := testdb.New(t)
	const uid = "put-blank-prose"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := putTemplate(t, srv, session, practiceID, contracts.TemplateResponse{Prose: tc.prose})
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
			}
		})
	}
}

// TestPutTemplateHandler_Success proves a full replace round-trips through
// GET, and that surrounding whitespace is trimmed.
func TestPutTemplateHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-success"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	putResp := putTemplate(t, srv, session, practiceID, contracts.TemplateResponse{Prose: "  Agreement with {{client_name}}  "})
	defer putResp.Body.Close()

	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("PUT status = %d, want %d", putResp.StatusCode, http.StatusOK)
	}

	var putOut contracts.TemplateResponse
	if err := json.NewDecoder(putResp.Body).Decode(&putOut); err != nil {
		t.Fatalf("decode PUT response: %v", err)
	}
	if putOut.Prose != "Agreement with {{client_name}}" {
		t.Fatalf("PUT prose = %q, want trimmed", putOut.Prose)
	}

	getResp := getTemplate(t, srv, session, practiceID)
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d, want %d", getResp.StatusCode, http.StatusOK)
	}
	var getOut contracts.TemplateResponse
	if err := json.NewDecoder(getResp.Body).Decode(&getOut); err != nil {
		t.Fatalf("decode GET response: %v", err)
	}
	if getOut.Prose != "Agreement with {{client_name}}" {
		t.Fatalf("GET prose after PUT = %q, want the just-written prose", getOut.Prose)
	}
}

// TestPutTemplateHandler_ReplacesExistingRow proves PUT overwrites a row
// seeded by signup (or a prior PUT) rather than conflicting with it -- the
// ON CONFLICT DO UPDATE path.
func TestPutTemplateHandler_ReplacesExistingRow(t *testing.T) {
	db := testdb.New(t)
	const uid = "put-replace"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	seedContractTemplate(t, db, practiceID, "Old prose")

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := putTemplate(t, srv, session, practiceID, contracts.TemplateResponse{Prose: "New prose"})
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	getResp := getTemplate(t, srv, session, practiceID)
	defer getResp.Body.Close()
	var out contracts.TemplateResponse
	if err := json.NewDecoder(getResp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Prose != "New prose" {
		t.Fatalf("prose = %q, want only the replacement prose", out.Prose)
	}
}
