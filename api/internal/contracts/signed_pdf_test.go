package contracts_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/objectstore"
	"doula-cloud/api/internal/testdb"
)

const signedPDFBytes = "%PDF-1.4 fake signed contract pdf"

// priorSignedPDFBytes stands in for the Signed PDF of an earlier,
// since-voided Contract on the same Engagement -- deliberately different
// bytes from signedPDFBytes, so a test can tell which of the two the
// route resolved to.
const priorSignedPDFBytes = "%PDF-1.4 fake superseded contract pdf"

// TestGetSignedContractPDFHandler_Success proves an Owner can retrieve
// the stored Signed PDF for a signed Contract -- #836 mounted this route
// through contracts.Mount's real declaration, which the old test mux
// never enforced. TestGetSignedContractPDFHandler_ContractorForbidden
// below proves the one role this route still refuses, per #282.
func TestGetSignedContractPDFHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-pdf-success"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	_, objectPath := seedSignedContract(t, db, engagementID)

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
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

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
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	otherPracticeID := testdb.SeedPractice(t, db, "Other Practice")
	_, otherEngagementID := testdb.SeedEngagement(t, db, otherPracticeID)
	seedSignedContract(t, db, otherEngagementID)

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
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, engagementID, "sent", mergeFieldProse)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := getContractPDF(t, srv, session, practiceID, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
	assertNoSignedContractMessage(t, resp)
}

// TestGetSignedContractPDFHandler_MissingObjectReturnsNotFound proves a
// signed row whose signed_pdf_object_path points at nothing in the store
// (should never happen given Sign's atomic write, but defensively
// checked) 404s -- ObjectStore.ErrNotFound now distinguishes this from a
// genuine storage-layer failure, which stays a 500 (see
// TestGetSignedContractPDFHandler_StoreGetFailureReturns500).
func TestGetSignedContractPDFHandler_MissingObjectReturnsNotFound(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-pdf-missing-object"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedSignedContract(t, db, engagementID)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := getContractPDF(t, srv, session, practiceID, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestGetSignedContractPDFHandler_StoreGetFailureReturns500 proves a
// genuine ObjectStore failure (a GCS outage, not a missing object) still
// 500s.
func TestGetSignedContractPDFHandler_StoreGetFailureReturns500(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-pdf-store-failure"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedSignedContract(t, db, engagementID)

	srv, session := newContractServerWithStore(t, db, uid, failingStore{})
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
	practiceID := testdb.SeedPractice(t, db, "Practice")
	clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	testdb.SeedPortalUser(t, db, identityUID, clientID)
	_, objectPath := seedSignedContract(t, db, engagementID)

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
	practiceID := testdb.SeedPractice(t, db, "Practice")
	clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	testdb.SeedPortalUser(t, db, identityUID, clientID)

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
	practiceID := testdb.SeedPractice(t, db, "Practice")
	_, otherEngagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Other Client", "other@example.com")
	seedSignedContract(t, db, otherEngagementID)
	clientID, _ := testdb.SeedNamedEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	testdb.SeedPortalUser(t, db, identityUID, clientID)

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
	practiceID := testdb.SeedPractice(t, db, "Practice")
	clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	testdb.SeedPortalUser(t, db, identityUID, clientID)
	seedContract(t, db, engagementID, "sent", mergeFieldProse)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := getClientContractPDF(t, srv, session, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
	assertNoSignedContractMessage(t, resp)
}

// TestClientGetSignedContractPDFHandler_MissingObjectReturnsNotFound
// mirrors the Staff-side defensive check for a signed row whose stored
// path resolves to nothing in the ObjectStore.
func TestClientGetSignedContractPDFHandler_MissingObjectReturnsNotFound(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-get-pdf-missing-object"
	practiceID := testdb.SeedPractice(t, db, "Practice")
	clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	testdb.SeedPortalUser(t, db, identityUID, clientID)
	seedSignedContract(t, db, engagementID)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := getClientContractPDF(t, srv, session, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// assertNoSignedContractMessage reads an apierr error body and checks it
// carries MsgNoSignedContract. Every refusal this package's two PDF
// routes produce for a missing document is that one message, so the
// helper pins it rather than taking it: the refusal must be about a
// document that was never produced, not about the Contract's status
// right now, and a status-shaped message drifting back in is the exact
// regression #299 fixed.
func assertNoSignedContractMessage(t *testing.T, resp *http.Response) {
	t.Helper()
	var body struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if body.Message != contracts.MsgNoSignedContract {
		t.Fatalf("error message = %q, want %q", body.Message, contracts.MsgNoSignedContract)
	}
}

// TestGetSignedContractPDFHandler_VoidedContractStillServes is #299's
// root: a Contract that has been voided keeps its Signed PDF reachable
// on the Practice route. Voiding cancels the agreement and deliberately
// leaves signed_pdf_object_path and the stored object alone (void.go),
// so refusing to serve it lost the Practice its own copy of what it
// canceled.
func TestGetSignedContractPDFHandler_VoidedContractStillServes(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-pdf-voided"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	contractID, objectPath := seedSignedContract(t, db, engagementID)

	store := objectstore.NewMemoryStore()
	if err := store.Put(t.Context(), objectPath, "application/pdf", bytes.NewReader([]byte(signedPDFBytes))); err != nil {
		t.Fatalf("seed stored pdf: %v", err)
	}
	voidContractRow(t, db, contractID)

	srv, session := newContractServerWithStore(t, db, uid, store)
	defer srv.Close()

	resp := getContractPDF(t, srv, session, practiceID, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d (a voided Contract's Signed PDF is preserved evidence, not a withdrawn document)", resp.StatusCode, http.StatusOK)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != signedPDFBytes {
		t.Fatalf("body = %q, want %q", body, signedPDFBytes)
	}

	var storedObjectPath sql.NullString
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT signed_pdf_object_path FROM contracts WHERE id = $1`, contractID,
	).Scan(&storedObjectPath); err != nil {
		t.Fatalf("query signed_pdf_object_path: %v", err)
	}
	if !storedObjectPath.Valid || storedObjectPath.String != objectPath {
		t.Fatalf("signed_pdf_object_path = %+v, want unchanged at %q", storedObjectPath, objectPath)
	}
}

// TestGetSignedContractPDFHandler_VoidedContractStillOpensToEmployedDoula
// proves the void changes nothing about who may read the PDF: an
// employed Doula reads it, per ADR-0008's money row as amended by #282.
func TestGetSignedContractPDFHandler_VoidedContractStillOpensToEmployedDoula(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-pdf-voided-doula"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	contractID, objectPath := seedSignedContract(t, db, engagementID)
	voidContractRow(t, db, contractID)

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
}

// TestGetSignedContractPDFHandler_ContractorForbidden proves the one
// role ADR-0008's money row as amended by #282 still refuses: a
// contractor Doula, even one holding a granted attachment on the
// Engagement -- the whole PDF is money-bearing and cannot be split, so
// she is refused outright rather than given a partial view.
func TestGetSignedContractPDFHandler_ContractorForbidden(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-pdf-contractor"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "contractor")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	testdb.SeedGrantedAttachment(t, db, engagementID, staffID)
	seedSignedContract(t, db, engagementID)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := getContractPDF(t, srv, session, practiceID, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// TestGetSignedContractPDFHandler_TwoSignedContractsServeTheNewest
// covers the consequence of dropping the status comparison: since #72's
// partial unique index lets voided rows accumulate, an Engagement can
// hold two Contracts that each carry a stored Signed PDF, and the read
// by Engagement alone now matches both. It resolves to the most recently
// created one, and the two PDFs are distinct objects -- the second
// signing must never have overwritten the first Contract's evidence,
// which the old Engagement-only object key made unavoidable.
func TestGetSignedContractPDFHandler_TwoSignedContractsServeTheNewest(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-pdf-two-signed"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	_, priorObjectPath := seedPriorSignedContract(t, db, engagementID)
	_, currentObjectPath := seedSignedContract(t, db, engagementID)

	if priorObjectPath == currentObjectPath {
		t.Fatalf("both Contracts stored their Signed PDF at %q -- the second signing overwrote the first", currentObjectPath)
	}

	store := objectstore.NewMemoryStore()
	if err := store.Put(t.Context(), priorObjectPath, "application/pdf", bytes.NewReader([]byte(priorSignedPDFBytes))); err != nil {
		t.Fatalf("seed prior stored pdf: %v", err)
	}
	if err := store.Put(t.Context(), currentObjectPath, "application/pdf", bytes.NewReader([]byte(signedPDFBytes))); err != nil {
		t.Fatalf("seed current stored pdf: %v", err)
	}

	srv, session := newContractServerWithStore(t, db, uid, store)
	defer srv.Close()

	resp := getContractPDF(t, srv, session, practiceID, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != signedPDFBytes {
		t.Fatalf("body = %q, want the most recently created signed Contract's PDF %q", body, signedPDFBytes)
	}

	// The superseded PDF is still in the store, addressable by its own
	// key -- preserved, just not what the Engagement-addressed route
	// resolves to.
	obj, err := store.Get(t.Context(), priorObjectPath)
	if err != nil {
		t.Fatalf("prior Signed PDF gone from the store at %q: %v", priorObjectPath, err)
	}
	defer func() { _ = obj.Close() }()
}

// TestClientGetSignedContractPDFHandler_VoidedContractStillServes is
// #299 on the portal side: the Client keeps her copy of what she signed
// after the Practice voids it. Same root, same fix, asserted separately
// because the two routes reach serveSignedPDF through different
// middleware and different RLS tiers.
func TestClientGetSignedContractPDFHandler_VoidedContractStillServes(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-get-pdf-voided"
	practiceID := testdb.SeedPractice(t, db, "Practice")
	clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	testdb.SeedPortalUser(t, db, identityUID, clientID)
	contractID, objectPath := seedSignedContract(t, db, engagementID)

	store := objectstore.NewMemoryStore()
	if err := store.Put(t.Context(), objectPath, "application/pdf", bytes.NewReader([]byte(signedPDFBytes))); err != nil {
		t.Fatalf("seed stored pdf: %v", err)
	}
	voidContractRow(t, db, contractID)

	srv, session := newPortalServerWithStore(t, db, identityUID, store)
	defer srv.Close()

	resp := getClientContractPDF(t, srv, session, engagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != signedPDFBytes {
		t.Fatalf("body = %q, want %q", body, signedPDFBytes)
	}
}

// TestClientGetSignedContractPDFHandler_VoidedStaysThisClientsOnly
// proves the void does not widen the portal route either: another
// Client of the same Practice is still refused before the handler runs.
func TestClientGetSignedContractPDFHandler_VoidedStaysThisClientsOnly(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-get-pdf-voided-other"
	practiceID := testdb.SeedPractice(t, db, "Practice")
	_, otherEngagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Other Client", "other@example.com")
	otherContractID, _ := seedSignedContract(t, db, otherEngagementID)
	voidContractRow(t, db, otherContractID)
	clientID, _ := testdb.SeedNamedEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	testdb.SeedPortalUser(t, db, identityUID, clientID)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := getClientContractPDFRaw(t, srv, session, otherEngagementID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// TestSignedContractPDFHandlers_DraftNotFound is the other half of "has
// never been signed": a Draft Contract has no stored PDF either, and both
// routes still refuse it. The sent-but-unsigned case has its own test
// above; this one exists because the refusal is now the absence of a
// stored PDF rather than a status comparison, so every status that has
// never carried one has to be shown still refusing.
func TestSignedContractPDFHandlers_DraftNotFound(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-pdf-draft"
	const identityUID = "client-get-pdf-draft"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	testdb.SeedPortalUser(t, db, identityUID, clientID)
	seedContract(t, db, engagementID, "draft", mergeFieldProse)

	srv, session := newContractServer(t, db, uid)
	defer srv.Close()

	resp := getContractPDF(t, srv, session, practiceID, engagementID)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("practice route status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
	assertNoSignedContractMessage(t, resp)

	portalSrv, portalSession := newPortalServer(t, db, identityUID)
	defer portalSrv.Close()

	portalResp := getClientContractPDF(t, portalSrv, portalSession, engagementID)
	defer portalResp.Body.Close()
	if portalResp.StatusCode != http.StatusNotFound {
		t.Fatalf("portal route status = %d, want %d", portalResp.StatusCode, http.StatusNotFound)
	}
	assertNoSignedContractMessage(t, portalResp)
}
