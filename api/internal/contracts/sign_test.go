package contracts_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/testdb"
)

func signContractURL(srv *httptest.Server, engagementID string) string {
	return srv.URL + "/portal/engagements/" + engagementID + "/contract/sign"
}

func postSignContractRaw(t *testing.T, srv *httptest.Server, engagementID string, body []byte) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, signContractURL(srv, engagementID), bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func postSignContractRawWithXFF(t *testing.T, srv *httptest.Server, engagementID string, body []byte, xff string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, signContractURL(srv, engagementID), bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", xff)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func postSignContract(t *testing.T, srv *httptest.Server, engagementID, fullLegalName string, attestation bool) *http.Response {
	t.Helper()
	payload, err := json.Marshal(contracts.SignContractRequest{FullLegalName: fullLegalName, Attestation: attestation})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	return postSignContractRaw(t, srv, engagementID, payload)
}

// signedRow is the row shape sign_test.go reads back via db.Admin to
// assert what ClientPostSignContractHandler actually persisted.
type signedRow struct {
	status            string
	signerFullName    sqlNullString
	signerAttestation bool
	signedAt          *time.Time
	signerIP          sqlNullString
}

type sqlNullString struct {
	String string
	Valid  bool
}

func fetchSignedRow(t *testing.T, db *testdb.DB, engagementID string) signedRow {
	t.Helper()
	var row signedRow
	var name, ip *string
	var signedAt *time.Time
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status, signer_full_name, signer_attestation, signed_at, signer_ip
		 FROM contracts WHERE engagement_id = $1`,
		engagementID,
	).Scan(&row.status, &name, &row.signerAttestation, &signedAt, &ip); err != nil {
		t.Fatalf("fetch contract row: %v", err)
	}
	if name != nil {
		row.signerFullName = sqlNullString{String: *name, Valid: true}
	}
	if ip != nil {
		row.signerIP = sqlNullString{String: *ip, Valid: true}
	}
	row.signedAt = signedAt
	return row
}

// TestClientPostSignContractHandler_Success proves the sent -> signed
// transition, that the typed full legal name and attestation checkbox
// state persist, and that the response reflects the new status while
// keeping the Contract's prose/mergeFields/values intact.
func TestClientPostSignContractHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-signing-sent-contract"
	practiceID := seedPractice(t, db, "Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	seedPortalUser(t, db, identityUID, clientID)
	seedContract(t, db, engagementID, "sent", mergeFieldProse)

	srv := newPortalServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	before := time.Now().Add(-time.Minute)
	resp := postSignContract(t, srv, engagementID, "Jordan Client", true)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out contracts.ContractResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Status != statusSigned {
		t.Fatalf("status = %q, want signed", out.Status)
	}
	if out.Prose != mergeFieldProse {
		t.Fatalf("prose = %q, want the original snapshot unchanged", out.Prose)
	}

	row := fetchSignedRow(t, db, engagementID)
	if row.status != statusSigned {
		t.Fatalf("persisted status = %q, want signed", row.status)
	}
	if !row.signerFullName.Valid || row.signerFullName.String != "Jordan Client" {
		t.Fatalf("persisted signer_full_name = %+v, want %q", row.signerFullName, "Jordan Client")
	}
	if !row.signerAttestation {
		t.Fatalf("persisted signer_attestation = false, want true")
	}
	if row.signedAt == nil || row.signedAt.Before(before) {
		t.Fatalf("persisted signed_at = %v, want a server-derived timestamp at/after %v", row.signedAt, before)
	}
	if !row.signerIP.Valid || row.signerIP.String == "" {
		t.Fatalf("persisted signer_ip is empty, want the caller's remote address")
	}
}

// TestClientPostSignContractHandler_ClientSuppliedSignedAtAndIPIgnored
// proves signed_at and signer_ip are always server-derived: a request
// body that tries to smuggle its own values for either field has no
// effect, since SignContractRequest has no fields for them -- signer_ip
// comes from X-Forwarded-For (set by Cloud Run's Google Front End, not
// the caller) rather than the body's "signerIp"/"ip" keys.
func TestClientPostSignContractHandler_ClientSuppliedSignedAtAndIPIgnored(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-spoofing-signed-at"
	practiceID := seedPractice(t, db, "Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	seedPortalUser(t, db, identityUID, clientID)
	seedContract(t, db, engagementID, "sent", mergeFieldProse)

	srv := newPortalServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	spoofed := `{"fullLegalName":"Jordan Client","attestation":true,"signedAt":"1999-01-01T00:00:00Z","signerIp":"10.0.0.1","ip":"10.0.0.1"}`
	before := time.Now().Add(-time.Minute)
	resp := postSignContractRawWithXFF(t, srv, engagementID, []byte(spoofed), "203.0.113.7, 10.0.0.1")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	row := fetchSignedRow(t, db, engagementID)
	if row.signedAt == nil || row.signedAt.Before(before) {
		t.Fatalf("persisted signed_at = %v, want a server-derived timestamp, not the spoofed 1999 value", row.signedAt)
	}
	if !row.signerIP.Valid || row.signerIP.String != "203.0.113.7" {
		t.Fatalf("persisted signer_ip = %+v, want the GFE-set X-Forwarded-For first entry 203.0.113.7, not the body-spoofed 10.0.0.1", row.signerIP)
	}
}

// TestClientPostSignContractHandler_NoXFFFallsBackToRemoteAddr proves
// clientIP falls back to r.RemoteAddr when there's no X-Forwarded-For
// header -- the local-dev/test path, with no Cloud Run GFE in front of
// the process.
func TestClientPostSignContractHandler_NoXFFFallsBackToRemoteAddr(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-signing-no-xff"
	practiceID := seedPractice(t, db, "Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	seedPortalUser(t, db, identityUID, clientID)
	seedContract(t, db, engagementID, "sent", mergeFieldProse)

	srv := newPortalServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	resp := postSignContract(t, srv, engagementID, "Jordan Client", true)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	row := fetchSignedRow(t, db, engagementID)
	if !row.signerIP.Valid || row.signerIP.String != "127.0.0.1" {
		t.Fatalf("persisted signer_ip = %+v, want the httptest RemoteAddr fallback 127.0.0.1", row.signerIP)
	}
}

// TestClientPostSignContractHandler_OtherClientsEngagementRejected proves
// a Client can only sign a Contract on their own Engagement --
// clientauth.Middleware's ownership check rejects a foreign Engagement id
// before this handler ever runs.
func TestClientPostSignContractHandler_OtherClientsEngagementRejected(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-not-linked-sign"
	practiceID := seedPractice(t, db, "Practice")
	_, otherEngagementID := seedClientEngagement(t, db, practiceID, "Other Client", "other@example.com")
	seedContract(t, db, otherEngagementID, "sent", mergeFieldProse)
	clientID, _ := seedClientEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	seedPortalUser(t, db, identityUID, clientID)

	srv := newPortalServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	resp := postSignContract(t, srv, otherEngagementID, "Jordan Client", true)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}

	row := fetchSignedRow(t, db, otherEngagementID)
	if row.status != "sent" {
		t.Fatalf("other client's contract status = %q, want unchanged sent", row.status)
	}
}

// TestClientPostSignContractHandler_NonSentRejected proves Sign 409s for
// an already-signed or voided Contract, and 404s for a Draft (unreachable
// via the Client-portal RLS policy, same as ClientGetContractHandler).
func TestClientPostSignContractHandler_NonSentRejected(t *testing.T) {
	cases := []struct {
		status string
		want   int
	}{
		{statusDraft, http.StatusNotFound},
		{statusSigned, http.StatusConflict},
		{statusVoided, http.StatusConflict},
	}
	for _, testCase := range cases {
		t.Run(testCase.status, func(t *testing.T) {
			db := testdb.New(t)
			identityUID := "client-sign-non-sent-" + testCase.status
			practiceID := seedPractice(t, db, "Practice")
			clientID, engagementID := seedClientEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
			seedPortalUser(t, db, identityUID, clientID)
			seedContract(t, db, engagementID, testCase.status, mergeFieldProse)

			srv := newPortalServer(fakeVerifier{uid: identityUID}, db)
			defer srv.Close()

			resp := postSignContract(t, srv, engagementID, "Jordan Client", true)
			defer resp.Body.Close()

			if resp.StatusCode != testCase.want {
				t.Fatalf("status = %d, want %d", resp.StatusCode, testCase.want)
			}
		})
	}
}

// TestClientPostSignContractHandler_NoContractYet404 proves an Engagement
// with no Contract row at all 404s, not a crash.
func TestClientPostSignContractHandler_NoContractYet404(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-sign-no-contract"
	practiceID := seedPractice(t, db, "Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	seedPortalUser(t, db, identityUID, clientID)

	srv := newPortalServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	resp := postSignContract(t, srv, engagementID, "Jordan Client", true)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestClientPostSignContractHandler_InvalidBody proves malformed JSON
// 400s.
func TestClientPostSignContractHandler_InvalidBody(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-sign-invalid-body"
	practiceID := seedPractice(t, db, "Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	seedPortalUser(t, db, identityUID, clientID)
	seedContract(t, db, engagementID, "sent", mergeFieldProse)

	srv := newPortalServer(fakeVerifier{uid: identityUID}, db)
	defer srv.Close()

	resp := postSignContractRaw(t, srv, engagementID, []byte("not json"))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestClientPostSignContractHandler_MissingFieldsRejected proves both a
// blank/whitespace-only typed name and an unchecked attestation are
// individually rejected with 400, and that neither leaves the Contract
// transitioned.
func TestClientPostSignContractHandler_MissingFieldsRejected(t *testing.T) {
	cases := []struct {
		name          string
		fullLegalName string
		attestation   bool
	}{
		{"blank name", "", true},
		{"whitespace-only name", "   ", true},
		{"unchecked attestation", "Jordan Client", false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			db := testdb.New(t)
			identityUID := "client-sign-missing-" + testCase.name
			practiceID := seedPractice(t, db, "Practice")
			clientID, engagementID := seedClientEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
			seedPortalUser(t, db, identityUID, clientID)
			seedContract(t, db, engagementID, "sent", mergeFieldProse)

			srv := newPortalServer(fakeVerifier{uid: identityUID}, db)
			defer srv.Close()

			resp := postSignContract(t, srv, engagementID, testCase.fullLegalName, testCase.attestation)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
			}

			row := fetchSignedRow(t, db, engagementID)
			if row.status != "sent" {
				t.Fatalf("contract status = %q, want unchanged sent", row.status)
			}
		})
	}
}
