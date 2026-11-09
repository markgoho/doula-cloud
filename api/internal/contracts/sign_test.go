package contracts_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// failingStore is an objectstore.ObjectStore whose Put and Get always
// fail, standing in for a real GCS outage -- mirrors
// message/handlers_test.go's failingStore. Package-local per client_test.go's
// note that Go test doubles aren't exported across packages.
type failingStore struct{}

func (failingStore) Put(_ context.Context, _, _ string, _ io.Reader) error {
	return errors.New("simulated store failure")
}

func (failingStore) Get(_ context.Context, _ string) (io.ReadCloser, error) {
	return nil, errors.New("simulated store failure")
}

// signedRow is the row shape sign_test.go reads back via db.Admin to
// assert what ClientPostSignContractHandler actually persisted.
type signedRow struct {
	id                  string
	status              string
	signerFullName      sqlNullString
	signerAttestation   bool
	signedAt            *time.Time
	signerIP            sqlNullString
	signedPDFObjectPath sqlNullString
}

type sqlNullString struct {
	String string
	Valid  bool
}

func fetchSignedRow(t *testing.T, db *testdb.DB, engagementID string) signedRow {
	t.Helper()
	var row signedRow
	var name, ip, pdfObjectPath *string
	var signedAt *time.Time
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT id, status, signer_full_name, signer_attestation, signed_at, signer_ip, signed_pdf_object_path
		 FROM contracts WHERE engagement_id = $1`,
		engagementID,
	).Scan(&row.id, &row.status, &name, &row.signerAttestation, &signedAt, &ip, &pdfObjectPath); err != nil {
		t.Fatalf("fetch contract row: %v", err)
	}
	if name != nil {
		row.signerFullName = sqlNullString{String: *name, Valid: true}
	}
	if ip != nil {
		row.signerIP = sqlNullString{String: *ip, Valid: true}
	}
	if pdfObjectPath != nil {
		row.signedPDFObjectPath = sqlNullString{String: *pdfObjectPath, Valid: true}
	}
	row.signedAt = signedAt
	return row
}

// signing is the body of a Sign request.
func signing(fullLegalName string, attestation bool) reqtest.Option {
	return reqtest.JSON(contracts.SignContractRequest{FullLegalName: fullLegalName, Attestation: attestation})
}

// signRow reads back what Sign persisted on the Engagement's Contract.
func (e *env) signRow() signedRow {
	e.T.Helper()
	return fetchSignedRow(e.T, e.DB, e.Engagement.ID)
}

// TestClientPostSignContractHandler_Success proves the sent -> signed
// transition, that the typed full legal name and attestation checkbox
// state persist, and that the response reflects the new status while
// keeping the Contract's prose/mergeFields/values intact.
func TestClientPostSignContractHandler_Success(t *testing.T) {
	e := newEnv(t, withPortalLogin())
	e.seedContract(statusSent)

	before := time.Now().Add(-time.Minute)
	out := reqtest.Expect[contracts.ContractResponse](t, http.MethodPost, e.portalURL("/sign"), http.StatusOK, e.AsClient, signing("Jordan Client", true))

	if out.Status != statusSigned {
		t.Fatalf("status = %q, want signed", out.Status)
	}
	if out.Prose != mergeFieldProse {
		t.Fatalf("prose = %q, want the original snapshot unchanged", out.Prose)
	}

	row := e.signRow()
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
// comes from the X-Forwarded-For entry the proxies appended, not the
// body's "signerIp"/"ip" keys nor a first entry the caller wrote into
// the header itself (#1865).
func TestClientPostSignContractHandler_ClientSuppliedSignedAtAndIPIgnored(t *testing.T) {
	e := newEnv(t, withPortalLogin())
	e.seedContract(statusSent)

	spoofed := `{"fullLegalName":"Jordan Client","attestation":true,"signedAt":"1999-01-01T00:00:00Z","signerIp":"10.0.0.1","ip":"10.0.0.1"}`
	before := time.Now().Add(-time.Minute)
	reqtest.ExpectStatus(t, http.MethodPost, e.portalURL("/sign"), http.StatusOK, e.AsClient,
		reqtest.RawJSON([]byte(spoofed)), reqtest.Header("X-Forwarded-For", "10.0.0.2, 203.0.113.7, 74.125.210.71"))

	row := e.signRow()
	if row.signedAt == nil || row.signedAt.Before(before) {
		t.Fatalf("persisted signed_at = %v, want a server-derived timestamp, not the spoofed 1999 value", row.signedAt)
	}
	if !row.signerIP.Valid || row.signerIP.String != "203.0.113.7" {
		t.Fatalf("persisted signer_ip = %+v, want the caller the proxies appended, 203.0.113.7, not the body-spoofed 10.0.0.1 or the header-spoofed 10.0.0.2", row.signerIP)
	}
}

// TestClientPostSignContractHandler_NoXFFFallsBackToRemoteAddr proves
// clientip.From falls back to r.RemoteAddr when there's no X-Forwarded-For
// header -- the local-dev/test path, with no Cloud Run GFE in front of
// the process.
func TestClientPostSignContractHandler_NoXFFFallsBackToRemoteAddr(t *testing.T) {
	e := newEnv(t, withPortalLogin())
	e.seedContract(statusSent)

	reqtest.ExpectStatus(t, http.MethodPost, e.portalURL("/sign"), http.StatusOK, e.AsClient, signing("Jordan Client", true))

	row := e.signRow()
	if !row.signerIP.Valid || row.signerIP.String != "127.0.0.1" {
		t.Fatalf("persisted signer_ip = %+v, want the httptest RemoteAddr fallback 127.0.0.1", row.signerIP)
	}
}

// TestClientPostSignContractHandler_OtherClientsEngagementRejected proves
// a Client can only sign a Contract on their own Engagement --
// clientauth.Middleware's ownership check rejects a foreign Engagement id
// before this handler ever runs.
func TestClientPostSignContractHandler_OtherClientsEngagementRejected(t *testing.T) {
	e := newEnv(t, withPortalLogin())
	other := e.AnotherEngagement()
	seedContract(t, e.DB, other.ID, statusSent, mergeFieldProse)

	reqtest.ExpectStatus(t, http.MethodPost, e.portalURLOf(other.ID, "/sign"), http.StatusForbidden, e.AsClient, signing("Jordan Client", true))

	if row := fetchSignedRow(t, e.DB, other.ID); row.status != statusSent {
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
			e := newEnv(t, withPortalLogin())
			e.seedContract(testCase.status)

			reqtest.ExpectStatus(t, http.MethodPost, e.portalURL("/sign"), testCase.want, e.AsClient, signing("Jordan Client", true))
		})
	}
}

// TestClientPostSignContractHandler_NoContractYet404 proves an Engagement
// with no Contract row at all 404s, not a crash.
func TestClientPostSignContractHandler_NoContractYet404(t *testing.T) {
	e := newEnv(t, withPortalLogin())

	reqtest.ExpectStatus(t, http.MethodPost, e.portalURL("/sign"), http.StatusNotFound, e.AsClient, signing("Jordan Client", true))
}

// TestClientPostSignContractHandler_RefusesAnUnsignableRequest proves what
// a Sign request is refused for with 400, none of which leaves the
// Contract transitioned: a body that is not JSON, a blank or
// whitespace-only typed name, and an unchecked attestation.
func TestClientPostSignContractHandler_RefusesAnUnsignableRequest(t *testing.T) {
	cases := []struct {
		name string
		body reqtest.Option
	}{
		{"malformed JSON", reqtest.RawJSON([]byte("not json"))},
		{"blank name", signing("", true)},
		{"whitespace-only name", signing("   ", true)},
		{"unchecked attestation", signing("Jordan Client", false)},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			e := newEnv(t, withPortalLogin())
			e.seedContract(statusSent)

			reqtest.ExpectStatus(t, http.MethodPost, e.portalURL("/sign"), http.StatusBadRequest, e.AsClient, testCase.body)

			if row := e.signRow(); row.status != statusSent {
				t.Fatalf("contract status = %q, want unchanged sent", row.status)
			}
		})
	}
}

// TestClientPostSignContractHandler_RendersAndStoresSignedPDF proves the
// sent -> signed transition renders a PDF-shaped payload and stores it in
// the injected ObjectStore under the Contract's own
// SignedPDFObjectPath key (#299: Engagement id plus Contract id), and
// persists that same key on the contracts row -- the #71 AC, checked by
// content-shape (the "%PDF" magic bytes) rather than byte-for-byte
// content.
func TestClientPostSignContractHandler_RendersAndStoresSignedPDF(t *testing.T) {
	e := newEnv(t, withPortalLogin())
	e.seedContract(statusSent)

	reqtest.ExpectStatus(t, http.MethodPost, e.portalURL("/sign"), http.StatusOK, e.AsClient, signing("Jordan Client", true))

	row := e.signRow()
	wantKey := contracts.SignedPDFObjectPath(e.Engagement.ID, row.id)
	if !row.signedPDFObjectPath.Valid || row.signedPDFObjectPath.String != wantKey {
		t.Fatalf("persisted signed_pdf_object_path = %+v, want %q", row.signedPDFObjectPath, wantKey)
	}

	obj, err := e.store.Get(t.Context(), wantKey)
	if err != nil {
		t.Fatalf("get stored pdf at %q: %v", wantKey, err)
	}
	defer func() { _ = obj.Close() }()
	pdfBytes, err := io.ReadAll(obj)
	if err != nil {
		t.Fatalf("read stored pdf: %v", err)
	}
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF")) {
		t.Fatalf("stored object does not look like a PDF, starts with %q", pdfBytes[:min(4, len(pdfBytes))])
	}
}

// TestClientPostSignContractHandler_PDFStoreFailureRejected proves a
// Storage.Put failure (a GCS outage) 500s the Sign request and leaves the
// Contract untransitioned -- rendering/storing the PDF happens before the
// status UPDATE, so a store failure never leaves a 'signed' row with no
// PDF behind it.
func TestClientPostSignContractHandler_PDFStoreFailureRejected(t *testing.T) {
	e := newEnv(t, withPortalLogin(), withStore(failingStore{}))
	e.seedContract(statusSent)

	reqtest.ExpectStatus(t, http.MethodPost, e.portalURL("/sign"), http.StatusInternalServerError, e.AsClient, signing("Jordan Client", true))

	if row := e.signRow(); row.status != statusSent {
		t.Fatalf("contract status = %q, want unchanged sent after a store failure", row.status)
	}
}
