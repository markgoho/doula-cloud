package contracts_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/objectstore"
	"doula-cloud/api/internal/pdftext"
	"doula-cloud/api/internal/push"
	"doula-cloud/api/internal/staffauth"
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
	db := testdb.New(t)
	const (
		identityUID = "client-signs-pdf-signature"
		staffUID    = "owner-reads-pdf-signature"
		typedName   = "Nadia Haddad"
	)
	practiceID := testdb.SeedPractice(t, db, "Okonkwo Birth Support")
	testdb.SetPracticeTimezone(t, db, practiceID, "America/Los_Angeles")
	testdb.SeedStaffAtPractice(t, db, practiceID, staffUID, []string{ownerRole}, "employee")
	clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, typedName, "nadia@example.com")
	testdb.SeedPortalUser(t, db, testdb.PortalUID(identityUID), clientID)
	seedContract(t, db, engagementID, "sent", "By signing below, both parties agree to the terms described above.")

	signedAt := time.Date(2026, time.October, 27, 18, 5, 0, 0, time.UTC)
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	contracts.Mount(g, idempotency.NewRouter(g, db.App), db.App, objectstore.NewMemoryStore(), push.NewFakePusher())
	srv := httptest.NewServer(clock.Middleware(func() time.Time { return signedAt })(mux))
	defer srv.Close()
	portalSession := authntest.SeedSession(t, db.App, testdb.PortalUID(identityUID))
	staffSession := authntest.SeedSession(t, db.App, staffUID)

	resp := postSignContract(t, srv, portalSession, engagementID, "  "+typedName+"  ", true)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("sign status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	row := fetchSignedRow(t, db, engagementID)
	if row.signedAt == nil || !row.signedAt.Equal(signedAt) {
		t.Fatalf("persisted signed_at = %v, want the request clock's %v", row.signedAt, signedAt)
	}

	portalResp := getClientContractPDF(t, srv, portalSession, engagementID)
	defer portalResp.Body.Close()
	staffResp := getContractPDF(t, srv, staffSession, practiceID, engagementID)
	defer staffResp.Body.Close()
	portalPDF := readPDF(t, portalResp)
	staffPDF := readPDF(t, staffResp)
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

func readPDF(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("pdf status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read pdf: %v", err)
	}
	return body
}
