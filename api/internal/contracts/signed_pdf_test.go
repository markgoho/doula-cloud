package contracts_test

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/objectstore"
	"doula-cloud/api/internal/testdb"
)

const signedPDFBytes = "%PDF-1.4 fake signed contract pdf"

// TestGetSignedContractPDFHandler_Success proves an Owner or Admin can
// retrieve the stored Signed PDF for a signed Contract -- #836 mounted
// this route through contracts.Mount's real OwnerAndAdmin declaration,
// which the old test mux never enforced; a plain Doula 403s now (this
// package carries no separate test for that 403, since gate_test.go-shaped
// role coverage lives at the api/ package's own guardrail level).
func TestGetSignedContractPDFHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-pdf-success"
	practiceID := seedOwner(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)
	objectPath := contracts.SignedPDFObjectPath(engagementID)
	seedSignedContract(t, db, engagementID, objectPath)

	store := objectstore.NewMemoryStore()
	if err := store.Put(t.Context(), objectPath, "application/pdf", bytes.NewReader([]byte(signedPDFBytes))); err != nil {
		t.Fatalf("seed stored pdf: %v", err)
	}

	srv, session := newContractServerWithStore(t, db, uid, store)
	defer srv.Close()

	resp := getContractPDF(t, srv, session, practiceID, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/pdf" {
		t.Fatalf("Content-Type = %q, want application/pdf", ct)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != signedPDFBytes {
		t.Fatalf("body = %q, want %q", body, signedPDFBytes)
	}
}

// TestGetSignedContractPDFHandler_Unauthenticated proves a request with
// no credential 401s before ever reaching the handler.
func TestGetSignedContractPDFHandler_Unauthenticated(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-pdf-unauthenticated"
	practiceID := seedMember(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)

	srv, _ := newContractServer(t, db, uid)
	defer srv.Close()

	resp := getContractPDFRaw(t, srv, "", practiceID, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

// TestGetSignedContractPDFHandler_CrossPracticeRejected proves an Owner
// at Practice A can't retrieve the Signed PDF for an Engagement at
// Practice B -- resolveContractRequest's requireEngagementAtPractice
// check rejects it before the PDF lookup runs.
func TestGetSignedContractPDFHandler_CrossPracticeRejected(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-pdf-cross-practice"
	practiceID := seedOwner(t, db, uid)
	otherPracticeID := seedPractice(t, db, "Other Practice")
	otherEngagementID := seedEngagement(t, db, otherPracticeID)
	objectPath := contracts.SignedPDFObjectPath(otherEngagementID)
	seedSignedContract(t, db, otherEngagementID, objectPath)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := getContractPDF(t, srv, session, practiceID, otherEngagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestGetSignedContractPDFHandler_NotYetSigned proves a sent (not yet
// signed) Contract 404s -- there is no PDF to serve yet.
func TestGetSignedContractPDFHandler_NotYetSigned(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-pdf-not-signed"
	practiceID := seedOwner(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, "sent", mergeFieldProse)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := getContractPDF(t, srv, session, practiceID, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestGetSignedContractPDFHandler_MissingObjectIsInternalError proves a
// signed row whose signed_pdf_object_path points at nothing in the store
// (should never happen given Sign's atomic write, but defensively
// checked) 500s rather than serving a broken response.
func TestGetSignedContractPDFHandler_MissingObjectIsInternalError(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-pdf-missing-object"
	practiceID := seedOwner(t, db, uid)
	engagementID := seedEngagement(t, db, practiceID)
	seedSignedContract(t, db, engagementID, contracts.SignedPDFObjectPath(engagementID))

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := getContractPDF(t, srv, session, practiceID, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}

// TestClientGetSignedContractPDFHandler_Success proves the owning Client
// can retrieve the stored Signed PDF for their own Contract.
func TestClientGetSignedContractPDFHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-get-pdf-success"
	practiceID := seedPractice(t, db, "Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	seedPortalUser(t, db, identityUID, clientID)
	objectPath := contracts.SignedPDFObjectPath(engagementID)
	seedSignedContract(t, db, engagementID, objectPath)

	store := objectstore.NewMemoryStore()
	if err := store.Put(t.Context(), objectPath, "application/pdf", bytes.NewReader([]byte(signedPDFBytes))); err != nil {
		t.Fatalf("seed stored pdf: %v", err)
	}

	srv, session := newPortalServerWithStore(t, db, identityUID, store)
	defer srv.Close()

	resp := getClientContractPDF(t, srv, session, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/pdf" {
		t.Fatalf("Content-Type = %q, want application/pdf", ct)
	}
}

// TestClientGetSignedContractPDFHandler_Unauthenticated proves a request
// with no credential 401s.
func TestClientGetSignedContractPDFHandler_Unauthenticated(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-get-pdf-unauthenticated"
	practiceID := seedPractice(t, db, "Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	seedPortalUser(t, db, identityUID, clientID)

	srv, _ := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := getClientContractPDFRaw(t, srv, "", engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

// TestClientGetSignedContractPDFHandler_OtherClientsEngagementRejected
// proves a Client can't retrieve another Client's Signed PDF --
// clientauth.Middleware's ownership check rejects a foreign Engagement id
// before this handler ever runs.
func TestClientGetSignedContractPDFHandler_OtherClientsEngagementRejected(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-get-pdf-not-linked"
	practiceID := seedPractice(t, db, "Practice")
	_, otherEngagementID := seedClientEngagement(t, db, practiceID, "Other Client", "other@example.com")
	seedSignedContract(t, db, otherEngagementID, contracts.SignedPDFObjectPath(otherEngagementID))
	clientID, _ := seedClientEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	seedPortalUser(t, db, identityUID, clientID)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := getClientContractPDFRaw(t, srv, session, otherEngagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// TestClientGetSignedContractPDFHandler_NotYetSigned proves a sent (not
// yet signed) Contract 404s from the Client-portal side too.
func TestClientGetSignedContractPDFHandler_NotYetSigned(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-get-pdf-not-signed"
	practiceID := seedPractice(t, db, "Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	seedPortalUser(t, db, identityUID, clientID)
	seedContract(t, db, engagementID, "sent", mergeFieldProse)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := getClientContractPDF(t, srv, session, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestClientGetSignedContractPDFHandler_MissingObjectIsInternalError
// mirrors the Staff-side defensive check for a signed row whose stored
// path resolves to nothing in the ObjectStore.
func TestClientGetSignedContractPDFHandler_MissingObjectIsInternalError(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-get-pdf-missing-object"
	practiceID := seedPractice(t, db, "Practice")
	clientID, engagementID := seedClientEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	seedPortalUser(t, db, identityUID, clientID)
	seedSignedContract(t, db, engagementID, contracts.SignedPDFObjectPath(engagementID))

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := getClientContractPDF(t, srv, session, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}
