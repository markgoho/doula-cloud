package export_test

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/testdb"
)

// TestHandler_OwnerGetsHerWholeArchive is the central acceptance
// criterion: an Owner downloads a ZIP of UTF-8 CSVs whose rows carry the
// ids needed to join them back together.
func TestHandler_OwnerGetsHerWholeArchive(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-export"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Ada Lovelace", "ada@example.com")

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/export")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := resp.Header.Get("Content-Type"); got != "application/zip" {
		t.Fatalf("Content-Type = %q, want application/zip", got)
	}
	if !strings.Contains(resp.Header.Get("Content-Disposition"), ".zip") {
		t.Fatalf("Content-Disposition = %q, want a .zip filename", resp.Header.Get("Content-Disposition"))
	}

	files := readZIP(t, resp)
	for _, want := range []string{"README.txt", "practice.csv", "client.csv", "engagement.csv", "activity.csv"} {
		if _, ok := files[want]; !ok {
			t.Fatalf("archive missing %s; have %v", want, keys(files))
		}
	}

	clientRows := parseCSV(t, files["client.csv"])
	if len(clientRows) != 2 { // header + one Client
		t.Fatalf("client.csv rows = %d, want 2", len(clientRows))
	}
	idCol := indexOf(t, clientRows[0], "id")
	if clientRows[1][idCol] != clientID {
		t.Fatalf("client.csv id = %q, want %q", clientRows[1][idCol], clientID)
	}

	engagementRows := parseCSV(t, files["engagement.csv"])
	clientIDCol := indexOf(t, engagementRows[0], "client_id")
	if engagementRows[1][clientIDCol] != clientID {
		t.Fatalf("engagement.csv client_id = %q, want %q (the join key back to client.csv)", engagementRows[1][clientIDCol], clientID)
	}
	_ = engagementID

	// Exactly one Activity row, subject the Practice, actor the Owner.
	activityRows := parseCSV(t, files["activity.csv"])
	if len(activityRows) != 2 {
		t.Fatalf("activity.csv rows = %d, want 2 (header + one export row)", len(activityRows))
	}
	subjectKindCol := indexOf(t, activityRows[0], "subject_kind")
	subjectIDCol := indexOf(t, activityRows[0], "subject_id")
	actionCol := indexOf(t, activityRows[0], "action")
	if activityRows[1][subjectKindCol] != "practice" || activityRows[1][subjectIDCol] != practiceID {
		t.Fatalf("activity row = %v, want subject practice/%s", activityRows[1], practiceID)
	}
	if activityRows[1][actionCol] != "practice_data_exported" {
		t.Fatalf("activity action = %q, want practice_data_exported", activityRows[1][actionCol])
	}
}

// TestHandler_RefusesEveryRoleButOwner is the second acceptance
// criterion: a non-Owner Staff member of the same Practice is refused at
// the API with a coded error. Table-driven over both non-Owner roles,
// the same shape client.TestEraseHandler_RefusesEveryRoleButOwner uses
// for the same reasoning -- Admin is the near-miss role most other
// Practice-wide reads admit (staffauth.OwnerAndAdmin), so it gets its
// own row rather than trusting doulaRole alone to stand in for it.
func TestHandler_RefusesEveryRoleButOwner(t *testing.T) {
	for _, role := range []string{doulaRole, adminRole} {
		t.Run(role, func(t *testing.T) {
			db := testdb.New(t)
			const uid = "staff-export-refused"
			practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{role}, "employee")

			srv, session := newServer(t, db, uid)
			defer srv.Close()

			resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/export")
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusForbidden {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
			}
			var apiErr apierr.APIError
			if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
				t.Fatalf("decode error body: %v", err)
			}
			if apiErr.Code != string(apierr.CodeForbidden) {
				t.Fatalf("code = %q, want %q", apiErr.Code, apierr.CodeForbidden)
			}
		})
	}
}

// TestHandler_NeverCrossesIntoAnotherPractice is the third acceptance
// criterion: a Staff member of a different Practice is refused, and no
// row belonging to another Practice appears in any file.
func TestHandler_NeverCrossesIntoAnotherPractice(t *testing.T) {
	db := testdb.New(t)
	const otherUID = "owner-other-practice"
	otherPracticeID := testdb.SeedPractice(t, db, "Other Practice")
	testdb.SeedNamedClient(t, db, otherPracticeID, "Someone Else", "else@example.com")

	const uid = "owner-own-practice"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	testdb.SeedNamedEngagement(t, db, practiceID, "Ada Lovelace", "ada@example.com")

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	// The other Practice's Owner cannot reach this Practice's export at all.
	otherSrv, otherSession := newServer(t, db, otherUID)
	defer otherSrv.Close()
	resp := authedGet(t, otherSession, otherSrv.URL+"/api/practices/"+practiceID+"/export")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-practice status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}

	// This Practice's own export names only her own Client, never the
	// other Practice's.
	resp2 := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/export")
	defer resp2.Body.Close()
	files := readZIP(t, resp2)
	if strings.Contains(files["client.csv"], "Someone Else") || strings.Contains(files["client.csv"], "else@example.com") {
		t.Fatalf("client.csv leaked another Practice's Client: %s", files["client.csv"])
	}
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func parseCSV(t *testing.T, s string) [][]string {
	t.Helper()
	rows, err := csv.NewReader(strings.NewReader(s)).ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	return rows
}

func indexOf(t *testing.T, header []string, col string) int {
	t.Helper()
	for i, h := range header {
		if h == col {
			return i
		}
	}
	t.Fatalf("header %v has no column %q", header, col)
	return -1
}
