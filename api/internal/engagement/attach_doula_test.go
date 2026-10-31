package engagement_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/testdb"
)

// The tests in this file are #2230: an Owner or an Admin puts a named
// Doula on an Engagement that already exists, the fifth writer of a
// granted Attachment. GLOSSARY.md's Attachment entry: "An Admin may
// attach an employee directly". A contractor who holds neither Owner nor
// Admin is still attached only by her own acceptance of an Offer.

// attachedDoulaName is the Doula put on in every test here.
const attachedDoulaName = "Priya Raman"

// attachingOwnerName is the Owner or Admin who presses, and
// malformedUUID is an id no row can carry.
const (
	attachingOwnerName = "Renata Alvarez"
	malformedUUID      = "not-a-uuid-at-all"
	grantedOrigin      = "granted"
)

func attachDoulaURL(base, practiceID, engagementID, staffID string) string {
	return base + "/api/practices/" + practiceID + "/engagements/" + engagementID + "/attachments/" + staffID
}

// attachDoulaAs PUTs staffID's Attachment as uid.
func attachDoulaAs(t *testing.T, db *testdb.DB, srv *httptest.Server, uid, practiceID, engagementID, staffID string) (int, attachSelfBody) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut, attachDoulaURL(srv.URL, practiceID, engagementID, staffID), http.NoBody)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, authntest.SeedSession(t, db.App, uid))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	var out attachSelfBody
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp.StatusCode, out
}

// newAttachDoulaServer seeds a Practice, the caller (callerName, holding
// callerRoles as an employee), the Doula to put on (attachedDoulaName,
// holding doulaRoles as doulaType), and an Engagement in status with
// nobody on it.
func newAttachDoulaServer(t *testing.T, db *testdb.DB, uid string, callerRoles, doulaRoles []string, doulaType, status string) (srv *httptest.Server, practiceID, callerID, doulaID, engagementID string) {
	t.Helper()
	practiceID = testdb.SeedPractice(t, db, "Attach Doula "+uid)
	callerID = testdb.SeedNamedStaffAtPractice(t, db, practiceID, uid, attachingOwnerName, callerRoles, employeeType)
	doulaID = testdb.SeedNamedStaffAtPractice(t, db, practiceID, uid+"-doula", attachedDoulaName, doulaRoles, doulaType)
	_, engagementID = testdb.SeedEngagementInStatus(t, db, practiceID, "Client", uid+"-client@example.com", status)
	srv, _ = newServer(t, db, uid)
	t.Cleanup(srv.Close)
	return srv, practiceID, callerID, doulaID, engagementID
}

// TestAttachDoulaHandler_RoleTable is the rule at the boundary that
// enforces it, for who presses and for who is put on.
func TestAttachDoulaHandler_RoleTable(t *testing.T) {
	cases := []struct {
		name        string
		callerRoles []string
		doulaRoles  []string
		doulaType   string
		wantStatus  int
		wantCode    apierr.Code
	}{
		{"owner puts an employee doula on", []string{ownerRole}, []string{doulaRole}, employeeType, http.StatusOK, ""},
		{"admin puts an employee doula on", []string{adminRole}, []string{doulaRole}, employeeType, http.StatusOK, ""},
		{"owner puts on an admin who is a contractor doula", []string{ownerRole}, []string{adminRole, doulaRole}, contractorType, http.StatusOK, ""},
		{"a doula cannot put a colleague on", []string{doulaRole}, []string{doulaRole}, employeeType, http.StatusForbidden, apierr.CodeForbidden},
		{"a contractor goes on by an offer", []string{ownerRole}, []string{doulaRole}, contractorType, http.StatusUnprocessableEntity, apierr.CodeInvalidArgument},
		{"a staff member with no doula role", []string{ownerRole}, []string{adminRole}, employeeType, http.StatusUnprocessableEntity, apierr.CodeInvalidArgument},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testdb.New(t)
			uid := "attach-doula-" + tc.name
			srv, practiceID, callerID, doulaID, engagementID := newAttachDoulaServer(t, db, uid, tc.callerRoles, tc.doulaRoles, tc.doulaType, engagement.StatusIntake)

			status, body := attachDoulaAs(t, db, srv, uid, practiceID, engagementID, doulaID)

			if tc.wantStatus != http.StatusOK {
				if status != tc.wantStatus || body.Code != tc.wantCode {
					t.Fatalf("status = %d code = %q, want %d %q", status, body.Code, tc.wantStatus, tc.wantCode)
				}
				if body.Message == "" {
					t.Fatal("refusal carries no message, want the reason")
				}
				if got := readOpenAttachments(t, db, engagementID, doulaID); len(got) != 0 {
					t.Fatalf("a refused press wrote an attachment: %+v", got)
				}
				if n := countActivityActions(t, db, engagementID, "doula_attached"); n != 0 {
					t.Fatalf("doula_attached entries = %d, want 0", n)
				}
				return
			}

			if status != http.StatusOK {
				t.Fatalf("status = %d, want 200: %s", status, body.Message)
			}
			if len(body.Doulas) != 1 || body.Doulas[0].StaffID != doulaID || body.Doulas[0].Name != attachedDoulaName {
				t.Fatalf("response doulas = %+v, want the Doula alone", body.Doulas)
			}
			rows := readOpenAttachments(t, db, engagementID, doulaID)
			if len(rows) != 1 || rows[0].origin != grantedOrigin || rows[0].attachedBy != callerID {
				t.Fatalf("attachment = %+v, want one granted row attached by the caller", rows)
			}
			if rows[0].feeCents != nil || rows[0].feeTerms != nil {
				t.Fatalf("attachment carries a fee: %+v", rows[0])
			}

			// The audit trail: the caller is the actor, the diff and the
			// ledger's sentence name the Doula, and the entry has a time.
			assertDiff(t, readFactDiff(t, db, engagementID, "doula_attached"), map[string]any{"attachedStaffId": doulaID})
			ledger := readActivity(t, db, practiceID, engagementID, uid)
			if len(ledger.Items) != 1 {
				t.Fatalf("ledger items = %d, want 1", len(ledger.Items))
			}
			entry := ledger.Items[0]
			if entry.ActorName != attachingOwnerName || entry.Detail != "Put on this Engagement as the Doula: "+attachedDoulaName || entry.CreatedAt.IsZero() {
				t.Fatalf("ledger entry = %+v, want who, whom and when", entry)
			}
		})
	}
}

// TestAttachDoulaHandler_ContractorRefusalNamesTheOffer: the refusal
// names her and the way she does go on.
func TestAttachDoulaHandler_ContractorRefusalNamesTheOffer(t *testing.T) {
	db := testdb.New(t)
	const uid = "attach-doula-contractor-words"
	srv, practiceID, _, doulaID, engagementID := newAttachDoulaServer(t, db, uid, []string{ownerRole}, []string{doulaRole}, contractorType, engagement.StatusActive)

	_, body := attachDoulaAs(t, db, srv, uid, practiceID, engagementID, doulaID)
	want := attachedDoulaName + " contracts for this Practice, so goes on an Engagement by accepting an Offer. Offer this work instead."
	if body.Message != want {
		t.Fatalf("message = %q, want %q", body.Message, want)
	}
}

// TestAttachDoulaHandler_SafeToRepeat: a Doula already on it gets the
// same 200 and nothing more is written.
func TestAttachDoulaHandler_SafeToRepeat(t *testing.T) {
	db := testdb.New(t)
	const uid = "attach-doula-repeat"
	srv, practiceID, _, doulaID, engagementID := newAttachDoulaServer(t, db, uid, []string{ownerRole}, []string{doulaRole}, employeeType, engagement.StatusActive)

	first, _ := attachDoulaAs(t, db, srv, uid, practiceID, engagementID, doulaID)
	second, secondBody := attachDoulaAs(t, db, srv, uid, practiceID, engagementID, doulaID)
	if first != http.StatusOK || second != http.StatusOK {
		t.Fatalf("statuses = %d, %d, want 200 twice", first, second)
	}
	if len(secondBody.Doulas) != 1 {
		t.Fatalf("second answer doulas = %+v, want one", secondBody.Doulas)
	}
	if n := countActivityActions(t, db, engagementID, "doula_attached"); n != 1 {
		t.Fatalf("doula_attached entries = %d, want 1", n)
	}
}

// TestAttachDoulaHandler_ACallerWhoIsADoulaKeepsHerOwnControl: an Owner
// who is a Doula puts a colleague on, and the answer still says she can
// put herself on too.
func TestAttachDoulaHandler_ACallerWhoIsADoulaKeepsHerOwnControl(t *testing.T) {
	db := testdb.New(t)
	const uid = "attach-doula-owner-doula"
	srv, practiceID, _, doulaID, engagementID := newAttachDoulaServer(t, db, uid, []string{ownerRole, doulaRole}, []string{doulaRole}, employeeType, engagement.StatusIntake)

	status, body := attachDoulaAs(t, db, srv, uid, practiceID, engagementID, doulaID)
	if status != http.StatusOK || !body.CanAttachSelf {
		t.Fatalf("status = %d canAttachSelf = %v, want 200 and true", status, body.CanAttachSelf)
	}
}

// TestAttachDoulaHandler_Refusals: a completed Engagement, an unknown or
// another Practice's Engagement, a person not at this Practice, and
// malformed ids.
func TestAttachDoulaHandler_Refusals(t *testing.T) {
	db := testdb.New(t)
	const uid = "attach-doula-refusals"
	srv, practiceID, _, doulaID, engagementID := newAttachDoulaServer(t, db, uid, []string{ownerRole}, []string{doulaRole}, employeeType, engagement.StatusIntake)
	_, completed := testdb.SeedEngagementInStatus(t, db, practiceID, "Done", uid+"-done@example.com", engagement.StatusCompleted)
	otherPractice := testdb.SeedPractice(t, db, "Another Practice")
	_, otherEngagement := testdb.SeedEngagement(t, db, otherPractice)
	stranger := testdb.SeedStaffAtPractice(t, db, otherPractice, uid+"-stranger", []string{doulaRole}, employeeType)
	const missing = "11111111-1111-1111-1111-111111111111"

	cases := []struct {
		name         string
		engagementID string
		staffID      string
		wantStatus   int
	}{
		{"completed", completed, doulaID, http.StatusConflict},
		{"another practice's engagement", otherEngagement, doulaID, http.StatusNotFound},
		{"unknown engagement", missing, doulaID, http.StatusNotFound},
		{"a person at another practice", engagementID, stranger, http.StatusUnprocessableEntity},
		{"malformed engagement", malformedUUID, doulaID, http.StatusBadRequest},
		{"malformed staff", engagementID, malformedUUID, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if status, _ := attachDoulaAs(t, db, srv, uid, practiceID, tc.engagementID, tc.staffID); status != tc.wantStatus {
				t.Fatalf("status = %d, want %d", status, tc.wantStatus)
			}
		})
	}
	if got := readOpenAttachments(t, db, engagementID, stranger); len(got) != 0 {
		t.Fatalf("a refused press wrote an attachment: %+v", got)
	}
}
