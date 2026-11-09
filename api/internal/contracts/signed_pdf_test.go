package contracts_test

import (
	"database/sql"
	"net/http"
	"strings"
	"testing"

	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

const signedPDFBytes = "%PDF-1.4 fake signed contract pdf"

// priorSignedPDFBytes stands in for the Signed PDF of an earlier,
// since-voided Contract on the same Engagement -- deliberately different
// bytes from signedPDFBytes, so a test can tell which of the two the
// route resolved to.
const priorSignedPDFBytes = "%PDF-1.4 fake superseded contract pdf"

// storePDF puts content in the object store at path, as a stored Signed PDF.
func (e *env) storePDF(path, content string) {
	e.T.Helper()
	if err := e.store.Put(e.T.Context(), path, "application/pdf", strings.NewReader(content)); err != nil {
		e.T.Fatalf("seed stored pdf: %v", err)
	}
}

// seedSignedAndStored seeds the Engagement's signed Contract and the bytes
// of its Signed PDF in the store, and returns the row's id and object path.
func (e *env) seedSignedAndStored() (contractID, objectPath string) {
	e.T.Helper()
	contractID, objectPath = seedSignedContract(e.T, e.DB, e.Engagement.ID)
	e.storePDF(objectPath, signedPDFBytes)
	return contractID, objectPath
}

// requireNoSignedContract proves a PDF route refuses with 404 and the one
// message every refusal for a missing document carries. The refusal must
// be about a document that was never produced, not about the Contract's
// status right now, and a status-shaped message drifting back in is the
// exact regression #299 fixed.
func requireNoSignedContract(t *testing.T, url string, session reqtest.Option) {
	t.Helper()
	got := apierrtest.Expect(t, http.MethodGet, url, http.StatusNotFound, session)
	if got.Message != contracts.MsgNoSignedContract {
		t.Fatalf("error message = %q, want %q", got.Message, contracts.MsgNoSignedContract)
	}
}

// requirePDF proves a PDF route serves the Signed PDF stored under want.
func requirePDF(t *testing.T, url string, session reqtest.Option, want string) {
	t.Helper()
	if body := reqtest.ExpectBody(t, http.MethodGet, url, http.StatusOK, session); string(body) != want {
		t.Fatalf("body = %q, want %q", body, want)
	}
}

// TestGetSignedContractPDFHandler_Success proves an Owner can retrieve
// the stored Signed PDF for a signed Contract -- #836 mounted this route
// through contracts.Mount's real declaration, which the old test mux
// never enforced. TestGetSignedContractPDFHandler_ContractorForbidden
// below proves the one role this route still refuses, per #282.
func TestGetSignedContractPDFHandler_Success(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	e.seedSignedAndStored()

	if pdf := reqtest.ExpectPDF(t, http.MethodGet, e.contractURL("/pdf"), e.SignedIn); string(pdf.Body) != signedPDFBytes {
		t.Fatalf("body = %q, want %q", pdf.Body, signedPDFBytes)
	}
}

// TestGetSignedContractPDFHandler_Unauthenticated proves a request with
// no credential 401s before ever reaching the handler.
func TestGetSignedContractPDFHandler_Unauthenticated(t *testing.T) {
	e := newEnv(t)

	reqtest.ExpectStatus(t, http.MethodGet, e.contractURL("/pdf"), http.StatusUnauthorized)
}

// TestGetSignedContractPDFHandler_CrossPracticeRejected proves an Owner
// at Practice A can't retrieve the Signed PDF for an Engagement at
// Practice B -- resolveContractRequest's requireEngagementAtPractice
// check rejects it before the PDF lookup runs.
func TestGetSignedContractPDFHandler_CrossPracticeRejected(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	otherPracticeID := testdb.SeedPractice(t, e.DB, "Other Practice")
	_, otherEngagementID := testdb.SeedEngagement(t, e.DB, otherPracticeID)
	seedSignedContract(t, e.DB, otherEngagementID)

	reqtest.ExpectStatus(t, http.MethodGet, e.contractURLOf(otherEngagementID, "/pdf"), http.StatusNotFound, e.SignedIn)
}

// TestGetSignedContractPDFHandler_NotYetSigned proves a sent (not yet
// signed) Contract 404s -- there is no PDF to serve yet.
func TestGetSignedContractPDFHandler_NotYetSigned(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	e.seedContract(statusSent)

	requireNoSignedContract(t, e.contractURL("/pdf"), e.SignedIn)
}

// TestGetSignedContractPDFHandler_MissingObjectReturnsNotFound proves a
// signed row whose signed_pdf_object_path points at nothing in the store
// (should never happen given Sign's atomic write, but defensively
// checked) 404s -- ObjectStore.ErrNotFound now distinguishes this from a
// genuine storage-layer failure, which stays a 500 (see
// TestGetSignedContractPDFHandler_StoreGetFailureReturns500).
func TestGetSignedContractPDFHandler_MissingObjectReturnsNotFound(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	seedSignedContract(t, e.DB, e.Engagement.ID)

	reqtest.ExpectStatus(t, http.MethodGet, e.contractURL("/pdf"), http.StatusNotFound, e.SignedIn)
}

// TestGetSignedContractPDFHandler_StoreGetFailureReturns500 proves a
// genuine ObjectStore failure (a GCS outage, not a missing object) still
// 500s.
func TestGetSignedContractPDFHandler_StoreGetFailureReturns500(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()), withStore(failingStore{}))
	seedSignedContract(t, e.DB, e.Engagement.ID)

	reqtest.ExpectStatus(t, http.MethodGet, e.contractURL("/pdf"), http.StatusInternalServerError, e.SignedIn)
}

// TestClientGetSignedContractPDFHandler_Success proves the owning Client
// can retrieve the stored Signed PDF for their own Contract.
func TestClientGetSignedContractPDFHandler_Success(t *testing.T) {
	e := newEnv(t, withPortalLogin())
	e.seedSignedAndStored()

	reqtest.ExpectPDF(t, http.MethodGet, e.portalURL("/pdf"), e.AsClient)
}

// TestClientGetSignedContractPDFHandler_Unauthenticated proves a request
// with no credential 401s.
func TestClientGetSignedContractPDFHandler_Unauthenticated(t *testing.T) {
	e := newEnv(t, withPortalLogin())

	reqtest.ExpectStatus(t, http.MethodGet, e.portalURL("/pdf"), http.StatusUnauthorized)
}

// TestClientGetSignedContractPDFHandler_OtherClientsEngagementRejected
// proves a Client can't retrieve another Client's Signed PDF --
// clientauth.Middleware's ownership check rejects a foreign Engagement id
// before this handler ever runs. It holds whether or not the other
// Client's Contract has since been voided: the void does not widen the
// portal route.
func TestClientGetSignedContractPDFHandler_OtherClientsEngagementRejected(t *testing.T) {
	cases := []struct {
		name   string
		voided bool
	}{
		{statusSigned, false},
		{"voided", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t, withPortalLogin())
			other := e.AnotherEngagement()
			otherContractID, _ := seedSignedContract(t, e.DB, other.ID)
			if c.voided {
				voidContractRow(t, e.DB, otherContractID)
			}

			reqtest.ExpectStatus(t, http.MethodGet, e.portalURLOf(other.ID, "/pdf"), http.StatusForbidden, e.AsClient)
		})
	}
}

// TestClientGetSignedContractPDFHandler_NotYetSigned proves a sent (not
// yet signed) Contract 404s from the Client-portal side too.
func TestClientGetSignedContractPDFHandler_NotYetSigned(t *testing.T) {
	e := newEnv(t, withPortalLogin())
	e.seedContract(statusSent)

	requireNoSignedContract(t, e.portalURL("/pdf"), e.AsClient)
}

// TestClientGetSignedContractPDFHandler_MissingObjectReturnsNotFound
// mirrors the Staff-side defensive check for a signed row whose stored
// path resolves to nothing in the ObjectStore.
func TestClientGetSignedContractPDFHandler_MissingObjectReturnsNotFound(t *testing.T) {
	e := newEnv(t, withPortalLogin())
	seedSignedContract(t, e.DB, e.Engagement.ID)

	reqtest.ExpectStatus(t, http.MethodGet, e.portalURL("/pdf"), http.StatusNotFound, e.AsClient)
}

// TestGetSignedContractPDFHandler_VoidedContractStillServes is #299's
// root: a Contract that has been voided keeps its Signed PDF reachable
// on the Practice route. Voiding cancels the agreement and deliberately
// leaves signed_pdf_object_path and the stored object alone (void.go),
// so refusing to serve it lost the Practice its own copy of what it
// canceled. An employed Doula reads it too -- the void changes nothing
// about who may (ADR-0008's money row as amended by #282).
func TestGetSignedContractPDFHandler_VoidedContractStillServes(t *testing.T) {
	cases := []struct {
		name  string
		staff fixture.StaffOption
	}{
		{"to an Owner", fixture.Owner()},
		{"to an employed Doula", fixture.Doula()},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(c.staff))
			contractID, objectPath := e.seedSignedAndStored()
			voidContractRow(t, e.DB, contractID)

			requirePDF(t, e.contractURL("/pdf"), e.SignedIn, signedPDFBytes)

			storedObjectPath := testdb.Scalar[sql.NullString](t, e.DB.Admin, `SELECT signed_pdf_object_path FROM contracts WHERE id = $1`, contractID)
			if !storedObjectPath.Valid || storedObjectPath.String != objectPath {
				t.Fatalf("signed_pdf_object_path = %+v, want unchanged at %q (a voided Contract's Signed PDF is preserved evidence, not a withdrawn document)", storedObjectPath, objectPath)
			}
		})
	}
}

// TestGetSignedContractPDFHandler_ContractorForbidden proves the one
// role ADR-0008's money row as amended by #282 still refuses: a
// contractor Doula, even one holding a granted attachment on the
// Engagement -- the whole PDF is money-bearing and cannot be split, so
// she is refused outright rather than given a partial view.
func TestGetSignedContractPDFHandler_ContractorForbidden(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Contractor()), attachedToEngagement())
	seedSignedContract(t, e.DB, e.Engagement.ID)

	reqtest.ExpectStatus(t, http.MethodGet, e.contractURL("/pdf"), http.StatusForbidden, e.SignedIn)
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
	e := newEnv(t, signedInAs(fixture.Owner()))
	_, priorObjectPath := seedPriorSignedContract(t, e.DB, e.Engagement.ID)
	_, currentObjectPath := seedSignedContract(t, e.DB, e.Engagement.ID)

	if priorObjectPath == currentObjectPath {
		t.Fatalf("both Contracts stored their Signed PDF at %q -- the second signing overwrote the first", currentObjectPath)
	}
	e.storePDF(priorObjectPath, priorSignedPDFBytes)
	e.storePDF(currentObjectPath, signedPDFBytes)

	requirePDF(t, e.contractURL("/pdf"), e.SignedIn, signedPDFBytes)

	// The superseded PDF is still in the store, addressable by its own
	// key -- preserved, just not what the Engagement-addressed route
	// resolves to.
	obj, err := e.store.Get(t.Context(), priorObjectPath)
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
	e := newEnv(t, withPortalLogin())
	contractID, _ := e.seedSignedAndStored()
	voidContractRow(t, e.DB, contractID)

	requirePDF(t, e.portalURL("/pdf"), e.AsClient, signedPDFBytes)
}

// TestSignedContractPDFHandlers_DraftNotFound is the other half of "has
// never been signed": a Draft Contract has no stored PDF either, and both
// routes still refuse it. The sent-but-unsigned case has its own test
// above; this one exists because the refusal is now the absence of a
// stored PDF rather than a status comparison, so every status that has
// never carried one has to be shown still refusing.
func TestSignedContractPDFHandlers_DraftNotFound(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()), withPortalLogin())
	e.seedDraft()

	requireNoSignedContract(t, e.contractURL("/pdf"), e.SignedIn)
	requireNoSignedContract(t, e.portalURL("/pdf"), e.AsClient)
}

// TestContractReads_VoidedContractReportsSignedPDF is #1119's root: the
// Contract read has to say that a Signed PDF exists, because the screens
// were deciding from status instead and hid the download the moment a
// Contract was voided -- while the route behind it went on serving. Both
// populations are asserted here: the Client is the one who loses her own
// copy of what she signed, and the Staff Engagement page reads the same
// fact for the Practice's copy.
func TestContractReads_VoidedContractReportsSignedPDF(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()), withPortalLogin())
	contractID, _ := seedSignedContract(t, e.DB, e.Engagement.ID)
	voidContractRow(t, e.DB, contractID)

	staffOut := e.contract()
	if staffOut.Status != statusVoided {
		t.Fatalf("staff status = %q, want %q", staffOut.Status, statusVoided)
	}
	if !staffOut.HasSignedPDF {
		t.Fatalf("staff hasSignedPdf = false on a voided Contract whose Signed PDF the route still serves (#1119)")
	}

	clientOut := reqtest.Expect[contracts.ContractResponse](t, http.MethodGet, e.portalURL(""), http.StatusOK, e.AsClient)
	if clientOut.Status != statusVoided {
		t.Fatalf("client status = %q, want %q", clientOut.Status, statusVoided)
	}
	if !clientOut.HasSignedPDF {
		t.Fatalf("client hasSignedPdf = false on a voided Contract she signed -- she has no path to her own copy (#1119)")
	}
}

// TestContractReads_UnsignedContractReportsNoSignedPDF is the other
// half: a Contract that has never been signed offers nothing to
// download, and the field says so on both surfaces, for both statuses
// that can precede a signature.
func TestContractReads_UnsignedContractReportsNoSignedPDF(t *testing.T) {
	for _, status := range []string{statusDraft, statusSent} {
		t.Run(status, func(t *testing.T) {
			e := newEnv(t, signedInAs(fixture.Owner()), withPortalLogin())
			e.seedContract(status)

			if e.contract().HasSignedPDF {
				t.Fatalf("staff hasSignedPdf = true on a %s Contract, which has no Signed PDF at all", status)
			}

			// The PDF route is the fact the field claims to mirror, so
			// the refusal is asserted beside it rather than assumed.
			reqtest.ExpectStatus(t, http.MethodGet, e.contractURL("/pdf"), http.StatusNotFound, e.SignedIn)

			if status == statusDraft {
				// A Draft is unreachable from the portal at all (RLS),
				// so there is no Client-side read to make here.
				return
			}
			clientOut := reqtest.Expect[contracts.ContractResponse](t, http.MethodGet, e.portalURL(""), http.StatusOK, e.AsClient)
			if clientOut.HasSignedPDF {
				t.Fatalf("client hasSignedPdf = true on a %s Contract, which has no Signed PDF at all", status)
			}
		})
	}
}

// TestGetContractHandler_DraftBesideVoidedSignedReportsSignedPDF proves
// the field is scoped to the Engagement, the way the route is, and not
// to the one Contract row the read happens to resolve. Void-then-recreate
// (#72) leaves a fresh Draft as the current Contract beside an older
// signed-and-voided row whose PDF is still what the route serves; a
// per-row reading would say "no PDF" for exactly that Engagement while
// the route streamed one.
func TestGetContractHandler_DraftBesideVoidedSignedReportsSignedPDF(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	_, priorObjectPath := seedPriorSignedContract(t, e.DB, e.Engagement.ID)
	e.seedDraft()
	e.storePDF(priorObjectPath, priorSignedPDFBytes)

	out := e.contract()

	if out.Status != statusDraft {
		t.Fatalf("status = %q, want %q -- the current Contract is the fresh Draft", out.Status, statusDraft)
	}
	if !out.HasSignedPDF {
		t.Fatalf("hasSignedPdf = false while the route still serves the voided Contract's PDF for this Engagement")
	}
	reqtest.ExpectStatus(t, http.MethodGet, e.contractURL("/pdf"), http.StatusOK, e.SignedIn)
}

// TestSignContractHandler_ResponseReportsSignedPDF proves the Sign
// response itself carries the field. Both Contract screens update their
// in-memory Contract from this body rather than reloading, so a false
// here would leave the download control hidden until a page refresh.
func TestSignContractHandler_ResponseReportsSignedPDF(t *testing.T) {
	e := newEnv(t, withPortalLogin())
	e.seedContract(statusSent)

	out := reqtest.Expect[contracts.ContractResponse](t, http.MethodPost, e.portalURL("/sign"), http.StatusOK, e.AsClient,
		reqtest.JSON(contracts.SignContractRequest{FullLegalName: "Jordan Client", Attestation: true}))

	if out.Status != statusSigned {
		t.Fatalf("status = %q, want %q", out.Status, statusSigned)
	}
	if !out.HasSignedPDF {
		t.Fatalf("hasSignedPdf = false on the Sign response -- the screen that just signed would still hide the download")
	}
}

// TestClientGetContractHandler_DraftBesideVoidedSignedReportsSignedPDF is
// the portal's own reading of the same fixture: an Engagement whose
// signed Contract was voided and then replaced by a fresh Draft. Her
// client-tier RLS policy hides the Draft, so the Contract she reads is
// the voided one -- and it reports its own PDF, which is the one the
// route serves her. The case where a screen could still lose her copy
// therefore does not exist, which is worth a test rather than an
// assertion.
func TestClientGetContractHandler_DraftBesideVoidedSignedReportsSignedPDF(t *testing.T) {
	e := newEnv(t, withPortalLogin())
	_, priorObjectPath := seedPriorSignedContract(t, e.DB, e.Engagement.ID)
	e.seedDraft()
	e.storePDF(priorObjectPath, priorSignedPDFBytes)

	out := reqtest.Expect[contracts.ContractResponse](t, http.MethodGet, e.portalURL(""), http.StatusOK, e.AsClient)

	if out.Status != statusVoided {
		t.Fatalf("status = %q, want %q -- the fresh Draft is hidden from her by RLS", out.Status, statusVoided)
	}
	if !out.HasSignedPDF {
		t.Fatalf("hasSignedPdf = false while the route still serves her the Contract she signed")
	}
	reqtest.ExpectStatus(t, http.MethodGet, e.portalURL("/pdf"), http.StatusOK, e.AsClient)
}
