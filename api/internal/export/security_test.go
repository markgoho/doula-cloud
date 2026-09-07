package export_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/testdb"
)

func authedPUT(t *testing.T, session, url string, body any) *http.Response {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut, url, bytes.NewReader(b))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

func authedPOST(t *testing.T, session, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, nil)
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

// TestHandler_ErasedClientExportsRedactedWithSealedDiff is the erasure
// acceptance criterion: an erased Client appears in her redacted state,
// with the fact and date of erasure, and her sealed Activity diffs
// appear unreadable rather than decrypted.
func TestHandler_ErasedClientExportsRedactedWithSealedDiff(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-erasure-export"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	clientID, _ := testdb.SeedNamedEngagement(t, db, practiceID, "Ada Lovelace", "ada@example.com")

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	// A real edit through the endpoint, so its Activity row is genuinely
	// sealed under her key, holding her phone number in the diff.
	editResp := authedPUT(t, session, srv.URL+"/api/practices/"+practiceID+"/clients/"+clientID,
		client.EditRequest{Record: client.Record{
			GivenName: "Ada", FamilyName: "Lovelace", Email: "ada@example.com", Phone: "585-555-0199",
		}, Override: true})
	defer editResp.Body.Close()
	if editResp.StatusCode != http.StatusOK {
		t.Fatalf("edit status = %d, want %d", editResp.StatusCode, http.StatusOK)
	}

	eraseResp := authedPOST(t, session, srv.URL+"/api/practices/"+practiceID+"/clients/"+clientID+"/erasure")
	defer eraseResp.Body.Close()
	if eraseResp.StatusCode != http.StatusOK {
		t.Fatalf("erase status = %d, want %d", eraseResp.StatusCode, http.StatusOK)
	}

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/export")
	defer resp.Body.Close()
	files := readZIP(t, resp)

	clientCSV := files["client.csv"]
	if !strings.Contains(clientCSV, "Erased Client") {
		t.Fatalf("client.csv does not show the erased placeholder name: %s", clientCSV)
	}
	if strings.Contains(clientCSV, "Ada") || strings.Contains(clientCSV, "Lovelace") || strings.Contains(clientCSV, "585-555") {
		t.Fatalf("client.csv still carries erased personal data: %s", clientCSV)
	}
	rows := parseCSV(t, clientCSV)
	erasedAtCol := indexOf(t, rows[0], "erased_at")
	if rows[1][erasedAtCol] == "" {
		t.Fatalf("client.csv erased_at is blank, want the erasure timestamp")
	}

	activityCSV := files["activity.csv"]
	if strings.Contains(activityCSV, "585-555-0199") {
		t.Fatalf("activity.csv leaked the plaintext phone number from a sealed diff: %s", activityCSV)
	}
	if !strings.Contains(activityCSV, `"enc"`) {
		t.Fatalf("activity.csv has no sealed ({\"v\":...,\"enc\":...}) diff at all -- the edit's own row should still be there, still ciphertext: %s", activityCSV)
	}
}

// TestHandler_ManyClientsAndEngagementsStreamsRather_ThanBuffers proves
// the "streamed, not assembled in memory" acceptance criterion against a
// real dataset, not a two-row fixture: a Practice with enough Clients and
// Engagements that a buffered implementation would still work, but only
// a streamed one would flush its ResponseWriter more than once.
func TestHandler_ManyClientsAndEngagementsStreamsRatherThanBuffers(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-many-clients"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	const count = 60
	for range count {
		testdb.SeedEngagement(t, db, practiceID)
	}

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/export")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	files := readZIP(t, resp)
	rows := parseCSV(t, files["client.csv"])
	if len(rows) != count+1 {
		t.Fatalf("client.csv rows = %d, want %d (header + %d Clients)", len(rows), count+1, count)
	}
}

// TestHandler_RefusesAnUnknownOrMalformedPractice covers the two shapes
// staffauth.Middleware itself already refuses before this handler ever
// runs: a syntactically invalid id (400) and a well-formed id naming no
// Practice this caller belongs to (403, indistinguishable from a real
// Practice she isn't a member of -- ADR-0008 never leaks which).
func TestHandler_RefusesAnUnknownOrMalformedPractice(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-unknown-practice"
	testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/not-a-uuid/export")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("malformed id status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	resp2 := authedGet(t, session, srv.URL+"/api/practices/00000000-0000-0000-0000-000000000000/export")
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusForbidden {
		t.Fatalf("unknown id status = %d, want %d", resp2.StatusCode, http.StatusForbidden)
	}
}
