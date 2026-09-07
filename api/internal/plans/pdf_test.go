package plans_test

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/testdb"
)

func getInstancePDF(t *testing.T, client *http.Client, url, session string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if session != "" {
		authntest.AddSessionCookie(req, session)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func requirePDFBody(t *testing.T, resp *http.Response) {
	t.Helper()
	if ct := resp.Header.Get("Content-Type"); ct != "application/pdf" {
		t.Fatalf("Content-Type = %q, want application/pdf", ct)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !bytes.HasPrefix(body, []byte("%PDF-")) {
		t.Fatalf("body does not start with the PDF magic header: %q", body[:min(16, len(body))])
	}
}

// TestGetInstancePDFHandler_Success proves Staff can download a rendered
// PDF of a Plan Instance they can already view as JSON (#306).
func TestGetInstancePDFHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-instance-pdf-success"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedInstance(t, db, engagementID, birthPlanType,
		`[{"id":"location","type":"single_select","label":"Planned birth location","options":["Home","Hospital"],"order":0}]`,
		`{"location":"Hospital"}`,
	)

	srv, session := newPlanServer(t, db, uid)
	defer srv.Close()

	resp := getInstancePDF(t, srv.Client(), srv.URL+instancePath(practiceID, engagementID, birthPlanType)+"/pdf", session)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	requirePDFBody(t, resp)
}

// TestGetInstancePDFHandler_NoInstance proves an Engagement with no Plan
// Instance yet 404s, same as GetInstanceHandler.
func TestGetInstancePDFHandler_NoInstance(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-instance-pdf-no-instance"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, session := newPlanServer(t, db, uid)
	defer srv.Close()

	resp := getInstancePDF(t, srv.Client(), srv.URL+instancePath(practiceID, engagementID, birthPlanType)+"/pdf", session)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestGetInstancePDFHandler_CrossPracticeRejected proves a Staff member
// at Practice A can't download the PDF for an Engagement at Practice B.
func TestGetInstancePDFHandler_CrossPracticeRejected(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-instance-pdf-cross-practice"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	otherPracticeID := testdb.SeedPractice(t, db, "Other Practice")
	_, otherEngagementID := testdb.SeedEngagement(t, db, otherPracticeID)
	seedInstance(t, db, otherEngagementID, birthPlanType,
		`[{"id":"location","type":"single_select","label":"Planned birth location","order":0}]`, `{}`)

	srv, session := newPlanServer(t, db, uid)
	defer srv.Close()

	resp := getInstancePDF(t, srv.Client(), srv.URL+instancePath(practiceID, otherEngagementID, birthPlanType)+"/pdf", session)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestGetInstancePDFHandler_ContractorWithoutAttachmentForbidden proves
// ADR-0008's attachment rule applies to the PDF the same way it does to
// GetInstanceHandler's JSON read: an unattached contractor Doula 404s.
func TestGetInstancePDFHandler_ContractorWithoutAttachmentForbidden(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-instance-pdf-contractor-unattached"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "contractor")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	seedInstance(t, db, engagementID, birthPlanType,
		`[{"id":"f1","type":"short_text","label":"Name","order":0}]`,
		`{"f1":"Jamie"}`,
	)

	srv, session := newPlanServer(t, db, uid)
	defer srv.Close()

	resp := getInstancePDF(t, srv.Client(), srv.URL+instancePath(practiceID, engagementID, birthPlanType)+"/pdf", session)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestGetInstancePDFHandler_Unauthenticated proves a request with no
// credential 401s before ever reaching the handler.
func TestGetInstancePDFHandler_Unauthenticated(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-instance-pdf-unauthenticated"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	srv, _ := newPlanServer(t, db, uid)
	defer srv.Close()

	resp := getInstancePDF(t, srv.Client(), srv.URL+instancePath(practiceID, engagementID, birthPlanType)+"/pdf", "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

// TestClientGetBirthPlanPDFHandler_Success proves a Client can download a
// rendered PDF of her own Birth Plan (#306).
func TestClientGetBirthPlanPDFHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-birth-plan-pdf-success"
	practiceID := testdb.SeedPractice(t, db, "Practice")
	clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	testdb.SeedPortalUser(t, db, identityUID, clientID)
	seedInstance(t, db, engagementID, birthPlanType,
		`[{"id":"location","type":"single_select","label":"Planned birth location","options":["Home","Hospital"],"order":0}]`,
		`{"location":"Hospital"}`,
	)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := getInstancePDF(t, srv.Client(), srv.URL+"/api/portal/engagements/"+engagementID+"/birth-plan/pdf", session)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	requirePDFBody(t, resp)
}

// TestClientGetBirthPlanPDFHandler_NoInstance proves an Engagement whose
// Birth Plan hasn't been created yet 404s, same as ClientGetBirthPlanHandler.
func TestClientGetBirthPlanPDFHandler_NoInstance(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-birth-plan-pdf-no-instance"
	practiceID := testdb.SeedPractice(t, db, "Practice")
	clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	testdb.SeedPortalUser(t, db, identityUID, clientID)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := getInstancePDF(t, srv.Client(), srv.URL+"/api/portal/engagements/"+engagementID+"/birth-plan/pdf", session)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestClientGetBirthPlanPDFHandler_PostpartumEngagementRefused proves
// #311's AC directly, same as ClientGetBirthPlanHandler: a postpartum-only
// Engagement never offers a Birth Plan PDF either, even reached straight
// by URL.
func TestClientGetBirthPlanPDFHandler_PostpartumEngagementRefused(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-birth-plan-pdf-postpartum-only"
	practiceID := testdb.SeedPractice(t, db, "Practice")
	clientID, engagementID := testdb.SeedEngagementWithKind(t, db, practiceID, "Jordan Client", "jordan@example.com", "postpartum")
	testdb.SeedPortalUser(t, db, identityUID, clientID)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := getInstancePDF(t, srv.Client(), srv.URL+"/api/portal/engagements/"+engagementID+"/birth-plan/pdf", session)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestClientGetBirthPlanPDFHandler_OtherClientsEngagementRejected proves a
// Client can't download the PDF for an Engagement that isn't hers --
// clientauth.Middleware itself rejects this with 403 before the handler
// ever runs, same as TestClientGetBirthPlanHandler_OtherClientsEngagementRejected.
func TestClientGetBirthPlanPDFHandler_OtherClientsEngagementRejected(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-birth-plan-pdf-other-engagement"
	practiceID := testdb.SeedPractice(t, db, "Practice")
	_, otherEngagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Other Client", "other@example.com")
	clientID, _ := testdb.SeedNamedEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")
	testdb.SeedPortalUser(t, db, identityUID, clientID)
	seedInstance(t, db, otherEngagementID, birthPlanType,
		`[{"id":"location","type":"single_select","label":"Planned birth location","order":0}]`, `{}`)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := getInstancePDF(t, srv.Client(), srv.URL+"/api/portal/engagements/"+otherEngagementID+"/birth-plan/pdf", session)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}
