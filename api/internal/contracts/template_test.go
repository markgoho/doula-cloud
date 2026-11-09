package contracts_test

import (
	"encoding/json/v2"
	"net/http"
	"testing"

	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

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
		`INSERT INTO contracts (engagement_id, status, prose, amount_cents) VALUES ($1, $2::contract_status, $3, 15000)`,
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
		`INSERT INTO contracts (engagement_id, status, prose, merge_field_values, amount_cents) VALUES ($1, 'draft'::contract_status, $2, $3, 15000)`,
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
		`INSERT INTO contracts (engagement_id, status, prose, created_at, amount_cents)
		 VALUES ($1, $2::contract_status, $3, now() - $4::interval, 15000) RETURNING id`,
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

// putTemplate sends a PUT of prose to the Practice's Contract Template.
func (e *env) putTemplate(prose string, want int) contracts.TemplateResponse {
	e.t.Helper()
	return reqtest.Expect[contracts.TemplateResponse](e.t, http.MethodPut, e.templateURL(), want, e.signedIn,
		reqtest.JSON(contracts.TemplateResponse{Prose: prose}))
}

// template reads the Practice's Contract Template.
func (e *env) template() contracts.TemplateResponse {
	e.t.Helper()
	return reqtest.Expect[contracts.TemplateResponse](e.t, http.MethodGet, e.templateURL(), http.StatusOK, e.signedIn)
}

func TestGetTemplateHandler_NotFound(t *testing.T) {
	e := newEnv(t)

	reqtest.ExpectStatus(t, http.MethodGet, e.templateURL(), http.StatusNotFound, e.signedIn)
}

// TestGetTemplateHandler_AnyMemberAllowed proves a non-Owner member (a
// doula) can read the template -- only PUT is Owner-gated.
func TestGetTemplateHandler_AnyMemberAllowed(t *testing.T) {
	e := newEnv(t)
	e.seedTemplate("Some prose with {{client_name}}")

	if out := e.template(); out.Prose != "Some prose with {{client_name}}" {
		t.Fatalf("prose = %q, want the seeded prose", out.Prose)
	}
}

// TestPutTemplateHandler_NonOwnerForbidden proves the mount's OwnerOnly
// declaration (#970) refuses both a plain Doula and an Admin -- the
// in-handler check this replaced was reader.Has("owner") alone, and the
// mount declaration must keep exactly that population, not widen it to
// Owner and Admin the way most of this package's other role rules do.
func TestPutTemplateHandler_NonOwnerForbidden(t *testing.T) {
	cases := map[string]fixture.StaffOption{"doula": fixture.Doula(), caseAdmin: fixture.Admin()}
	for name, role := range cases {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, signedInAs(role))

			e.putTemplate("Some prose", http.StatusForbidden)
		})
	}
}

// TestPutTemplateHandler_Refusals table-drives a body that is not JSON
// and an empty and a whitespace-only prose, the last two of which
// normalize to "" after trimming.
func TestPutTemplateHandler_Refusals(t *testing.T) {
	cases := []struct {
		name string
		body reqtest.Option
	}{
		{caseBodyNotJSON, reqtest.RawJSON([]byte("not json"))},
		{"empty", reqtest.JSON(contracts.TemplateResponse{Prose: ""})},
		{"whitespace only", reqtest.JSON(contracts.TemplateResponse{Prose: "   \n\t  "})},
	}
	e := newEnv(t, signedInAs(fixture.Owner()))

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reqtest.ExpectStatus(t, http.MethodPut, e.templateURL(), http.StatusBadRequest, e.signedIn, tc.body)
		})
	}
}

// TestPutTemplateHandler_Success proves a full replace round-trips through
// GET, and that surrounding whitespace is trimmed.
func TestPutTemplateHandler_Success(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))

	if out := e.putTemplate("  Agreement with {{client_name}}  ", http.StatusOK); out.Prose != "Agreement with {{client_name}}" {
		t.Fatalf("PUT prose = %q, want trimmed", out.Prose)
	}

	if out := e.template(); out.Prose != "Agreement with {{client_name}}" {
		t.Fatalf("GET prose after PUT = %q, want the just-written prose", out.Prose)
	}
}

// TestPutTemplateHandler_ReplacesExistingRow proves PUT overwrites a row
// seeded by signup (or a prior PUT) rather than conflicting with it -- the
// ON CONFLICT DO UPDATE path.
func TestPutTemplateHandler_ReplacesExistingRow(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	e.seedTemplate("Old prose")

	e.putTemplate("New prose", http.StatusOK)

	if out := e.template(); out.Prose != "New prose" {
		t.Fatalf("prose = %q, want only the replacement prose", out.Prose)
	}
}
