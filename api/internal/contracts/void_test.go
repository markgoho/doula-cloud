package contracts_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/testdb"
)

func voidContractURL(srv *httptest.Server, practiceID, engagementID string) string {
	return srv.URL + "/api/practices/" + practiceID + "/engagements/" + engagementID + "/contract/void"
}

func postVoidContract(t *testing.T, srv *httptest.Server, session string, practiceID, engagementID string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, voidContractURL(srv, practiceID, engagementID), nil)
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

func TestPostVoidContractHandler_InvalidEngagementID(t *testing.T) {
	db := testdb.New(t)
	const uid = "void-invalid-engagement-id"
	practiceID := seedMember(t, db, uid)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := postVoidContract(t, srv, session, practiceID, "not-a-uuid")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestPostVoidContractHandler_EngagementNotFound(t *testing.T) {
	db := testdb.New(t)
	const uid = "void-no-engagement"
	practiceID := seedMember(t, db, uid)
	otherPracticeID := seedPractice(t, db, "Other Practice")
	otherEngagementID := seedEngagement(t, db, otherPracticeID)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := postVoidContract(t, srv, session, practiceID, otherEngagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestPostVoidContractHandler_NoContract(t *testing.T) {
	db := testdb.New(t)
	const uid = "void-no-contract"
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := postVoidContract(t, srv, session, practiceID, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestPostVoidContractHandler_NonSignedRejected proves Void 409s from
// every status but 'signed' -- draft/sent haven't reached an agreement
// yet, and an already-voided Contract can't be voided a second time.
func TestPostVoidContractHandler_NonSignedRejected(t *testing.T) {
	for _, status := range []string{statusDraft, statusSent, statusVoided} {
		t.Run(status, func(t *testing.T) {
			db := testdb.New(t)
			uid := "void-non-signed-" + status
			practiceID := seedMember(t, db, uid)
			engagementID := seedEngagement(t, db, practiceID)
			seedContract(t, db, engagementID, status, mergeFieldProse)

			srv, session := newContractServer(t, db, uid)
			defer srv.Close()

			resp := postVoidContract(t, srv, session, practiceID, engagementID)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusConflict {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
			}
		})
	}
}

// TestPostVoidContractHandler_Success proves the signed -> voided
// transition, that the response and a subsequent GET both reflect the
// new status while prose/mergeFields/values stay intact, and that the
// Signed PDF's object reference (signed_pdf_object_path) is left
// untouched by Void -- the ticket's "never deletes or alters" AC for the
// historical record.
func TestPostVoidContractHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const uid = "void-success"
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)
	objectPath := contracts.SignedPDFObjectPath(engagementID)
	seedSignedContract(t, db, engagementID, objectPath)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := postVoidContract(t, srv, session, practiceID, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out contracts.ContractResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Status != statusVoided {
		t.Fatalf("status = %q, want voided", out.Status)
	}
	if out.Prose != mergeFieldProse {
		t.Fatalf("prose = %q, want the original snapshot unchanged", out.Prose)
	}

	getResp := getContract(t, srv, session, practiceID, engagementID)
	defer getResp.Body.Close()
	var getOut contracts.ContractResponse
	if err := json.NewDecoder(getResp.Body).Decode(&getOut); err != nil {
		t.Fatalf("decode GET response: %v", err)
	}
	if getOut.Status != statusVoided {
		t.Fatalf("GET status after Void = %q, want voided (the transition persisted)", getOut.Status)
	}

	var storedObjectPath sql.NullString
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT signed_pdf_object_path FROM contracts WHERE engagement_id = $1`,
		engagementID,
	).Scan(&storedObjectPath); err != nil {
		t.Fatalf("query signed_pdf_object_path: %v", err)
	}
	if !storedObjectPath.Valid || storedObjectPath.String != objectPath {
		t.Fatalf("signed_pdf_object_path = %+v, want unchanged at %q", storedObjectPath, objectPath)
	}
}

// TestPostContractHandler_AllowedAfterVoid proves the recreate-after-void
// flow the ticket's "What to build" section describes -- and #66's user
// story 22 -- actually works: contracts_engagement_id_active_key
// (00020_contracts_recreate_after_void.sql) permits a second row for the
// same Engagement once the first is voided, so POST no longer 409s
// forever the way it would under the original table-wide UNIQUE
// (engagement_id). It also proves the new row is independent of the old
// one: creating and then Sending it must never touch the voided row's
// status or its signed_pdf_object_path, which is exactly what targeting
// fetchContract's returned id (rather than re-deriving "the" row from
// engagement_id) in every UPDATE is for.
func TestPostContractHandler_AllowedAfterVoid(t *testing.T) {
	db := testdb.New(t)
	const uid = "post-after-void"
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)
	seedTemplate(t, db, practiceID, mergeFieldProse)
	oldObjectPath := contracts.SignedPDFObjectPath(engagementID)
	seedSignedContract(t, db, engagementID, oldObjectPath)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	voidResp := postVoidContract(t, srv, session, practiceID, engagementID)
	defer voidResp.Body.Close()
	if voidResp.StatusCode != http.StatusOK {
		t.Fatalf("void status = %d, want %d", voidResp.StatusCode, http.StatusOK)
	}

	createResp := postContract(t, srv, session, practiceID, engagementID)
	defer createResp.Body.Close()
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create-after-void status = %d, want %d (the recreate flow must not 409 forever)", createResp.StatusCode, http.StatusCreated)
	}
	var created contracts.ContractResponse
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.Status != statusDraft {
		t.Fatalf("new contract status = %q, want draft", created.Status)
	}

	sendResp := postSendContract(t, srv, session, practiceID, engagementID)
	defer sendResp.Body.Close()
	if sendResp.StatusCode != http.StatusOK {
		t.Fatalf("send-after-recreate status = %d, want %d", sendResp.StatusCode, http.StatusOK)
	}

	rows, err := db.Admin.QueryContext(t.Context(),
		`SELECT status, signed_pdf_object_path FROM contracts WHERE engagement_id = $1 ORDER BY created_at`,
		engagementID,
	)
	if err != nil {
		t.Fatalf("query contract rows: %v", err)
	}
	defer rows.Close()

	type row struct {
		status     string
		objectPath sql.NullString
	}
	var got []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.status, &r.objectPath); err != nil {
			t.Fatalf("scan row: %v", err)
		}
		got = append(got, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate rows: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("contract rows for engagement = %d, want 2 (the voided original plus the recreated one)", len(got))
	}
	if got[0].status != statusVoided || !got[0].objectPath.Valid || got[0].objectPath.String != oldObjectPath {
		t.Fatalf("original row = %+v, want voided with its signed_pdf_object_path untouched by the recreate/send", got[0])
	}
	if got[1].status != statusSent || got[1].objectPath.Valid {
		t.Fatalf("recreated row = %+v, want sent with no signed_pdf_object_path", got[1])
	}
}
