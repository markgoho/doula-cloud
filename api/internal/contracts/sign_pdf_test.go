package contracts_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/pdftext"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// TestClientPostSignContractHandler_SignedPDFCarriesTheSignature is
// #2214: the signed PDF says, below the prose, who signed it, when, that
// it was signed electronically, and what she confirmed. The time is the
// stored signed_at read in the Practice's own zone (ADR-0036) -- seeded
// here as Los Angeles, not the seed's New York default, so a PDF printed
// in UTC or in a fixed zone would fail -- and the Practice-side and
// portal downloads hand back the same bytes.
func TestClientPostSignContractHandler_SignedPDFCarriesTheSignature(t *testing.T) {
	const typedName = "Nadia Haddad"
	signedAt := time.Date(2026, time.October, 27, 18, 5, 0, 0, time.UTC)
	e := newEnv(t,
		withPractice(fixture.PracticeName("Okonkwo Birth Support")),
		signedInAs(fixture.Owner()),
		withClient(fixture.ClientName(typedName), fixture.WithPortalLogin()),
		atInstant(signedAt))
	testdb.SetPracticeTimezone(t, e.db, e.Practice.ID, "America/Los_Angeles")
	seedContract(t, e.db, e.Engagement.ID, statusSent, "By signing below, both parties agree to the terms described above.")

	reqtest.ExpectStatus(t, http.MethodPost, e.portalURL("/sign"), http.StatusOK, e.asClient, signing("  "+typedName+"  ", true))

	row := e.signRow()
	if row.signedAt == nil || !row.signedAt.Equal(signedAt) {
		t.Fatalf("persisted signed_at = %v, want the request clock's %v", row.signedAt, signedAt)
	}

	portalPDF := reqtest.ExpectBody(t, http.MethodGet, e.portalURL("/pdf"), http.StatusOK, e.asClient)
	staffPDF := reqtest.ExpectBody(t, http.MethodGet, e.contractURL("/pdf"), http.StatusOK, e.signedIn)
	if string(portalPDF) != string(staffPDF) {
		t.Fatal("the portal and Practice-side downloads served different files")
	}

	got := strings.Split(pdftext.Text(portalPDF), "\n")
	want := []string{
		"By signing below, both parties agree to the terms described above.",
		"Signed electronically by: Nadia Haddad",
		"Signed on: Oct 27, 2026, 11:05am PDT",
		"Confirmed: I have read this contract and I am signing it electronically",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("PDF text = %q, want %q", got, want)
	}
}
