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

// The tests in this file are #1598: a Doula puts herself on an
// Engagement with one press, the fourth writer of a granted Attachment
// (ADR-0008's amendment on #1515). #1625 added the Owner and the Admin
// who are contractor Doulas to the people the press is for.

// attachSelfBody decodes both shapes the endpoint answers with: the
// success DTO, and apierr's own error envelope.
type attachSelfBody struct {
	engagement.AttachSelfResponse
	Code    apierr.Code `json:"code"`
	Message string      `json:"message"`
}

// doulaName is the person at the screen in every test here.
const doulaName = "Hana Kim"

func attachSelfURL(base, practiceID, engagementID string) string {
	return base + "/api/practices/" + practiceID + "/engagements/" + engagementID + "/attachments/me"
}

// attachSelfAs PUTs the caller's own Attachment as uid.
func attachSelfAs(t *testing.T, db *testdb.DB, srv *httptest.Server, uid, practiceID, engagementID string) (int, attachSelfBody) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut, attachSelfURL(srv.URL, practiceID, engagementID), http.NoBody)
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

// attachmentRow is one engagement_attachments row, read past RLS.
type attachmentRow struct {
	origin     string
	attachedBy string
	feeCents   *int64
	feeTerms   *string
}

// readOpenAttachments reads every open Attachment staffID holds on
// engagementID. The partial unique index allows one at most.
func readOpenAttachments(t *testing.T, db *testdb.DB, engagementID, staffID string) []attachmentRow {
	t.Helper()
	rows, err := db.Admin.QueryContext(t.Context(),
		`SELECT origin::text, attached_by::text, fee_amount_cents, fee_terms
		   FROM engagement_attachments
		  WHERE engagement_id = $1 AND staff_id = $2 AND ended_at IS NULL`,
		engagementID, staffID)
	if err != nil {
		t.Fatalf("read attachments: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var out []attachmentRow
	for rows.Next() {
		var row attachmentRow
		if err := rows.Scan(&row.origin, &row.attachedBy, &row.feeCents, &row.feeTerms); err != nil {
			t.Fatalf("scan attachment: %v", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate attachments: %v", err)
	}
	return out
}

// readDetail GETs the Engagement as uid.
func readDetail(t *testing.T, db *testdb.DB, srv *httptest.Server, uid, practiceID, engagementID string) engagement.Detail {
	t.Helper()
	resp := authedGet(t, authntest.SeedSession(t, db.App, uid), srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("detail status = %d, want 200", resp.StatusCode)
	}
	var d engagement.Detail
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	return d
}

// newAttachSelfServer seeds a Practice, one Staff member named name
// holding roles as employmentType, and an Engagement in status with
// nobody on it.
func newAttachSelfServer(t *testing.T, db *testdb.DB, uid, name string, roles []string, employmentType, status string) (srv *httptest.Server, practiceID, staffID, engagementID string) {
	t.Helper()
	practiceID = testdb.SeedPractice(t, db, "Attach Self "+uid)
	staffID = testdb.SeedNamedStaffAtPractice(t, db, practiceID, uid, name, roles, employmentType)
	_, engagementID = testdb.SeedEngagementInStatus(t, db, practiceID, "Client", uid+"-client@example.com", status)
	srv, _ = newServer(t, db, uid)
	t.Cleanup(srv.Close)
	return srv, practiceID, staffID, engagementID
}

// TestAttachSelfHandler_RoleTable is the whole rule, at the boundary
// that enforces it: a Member with the Doula role is attached where she
// is an employee, or where she holds Owner or Admin, and nobody else is.
// An Owner who is also a Doula is attached: ADR-0008's "an Owner or Admin
// is never attached" is about an accrued Attachment, and this one is
// granted. An Owner or an Admin who is a contractor Doula is attached
// too (#1625): "only her acceptance of an Offer" is the rule for a
// contractor who holds neither role. The read agrees with the write in
// every row, so the screen never draws a control the write refuses.
func TestAttachSelfHandler_RoleTable(t *testing.T) {
	cases := []struct {
		name           string
		roles          []string
		employmentType string
		wantOK         bool
	}{
		{"employee doula", []string{doulaRole}, employeeType, true},
		{"owner who is an employee doula", []string{ownerRole, doulaRole}, employeeType, true},
		{"admin who is an employee doula", []string{adminRole, doulaRole}, employeeType, true},
		{"owner with no doula role", []string{ownerRole}, employeeType, false},
		{"admin with no doula role", []string{adminRole}, employeeType, false},
		{"staff with no role", []string{}, employeeType, false},
		{"owner who is a contractor doula", []string{ownerRole, doulaRole}, contractorType, true},
		{"admin who is a contractor doula", []string{adminRole, doulaRole}, contractorType, true},
		{"owner who is a contractor with no doula role", []string{ownerRole}, contractorType, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testdb.New(t)
			uid := "attach-self-" + tc.name
			srv, practiceID, staffID, engagementID := newAttachSelfServer(t, db, uid, doulaName, tc.roles, tc.employmentType, engagement.StatusIntake)

			before := readDetail(t, db, srv, uid, practiceID, engagementID)
			if before.CanAttachSelf != tc.wantOK {
				t.Fatalf("detail canAttachSelf = %v, want %v", before.CanAttachSelf, tc.wantOK)
			}
			if len(before.Doulas) != 0 {
				t.Fatalf("detail doulas = %v, want none before the press", before.Doulas)
			}

			status, body := attachSelfAs(t, db, srv, uid, practiceID, engagementID)

			if !tc.wantOK {
				if status != http.StatusForbidden || body.Code != apierr.CodeForbidden {
					t.Fatalf("status = %d code = %q, want 403 FORBIDDEN", status, body.Code)
				}
				if body.Message == "" {
					t.Fatal("refusal carries no message, want the reason")
				}
				if got := readOpenAttachments(t, db, engagementID, staffID); len(got) != 0 {
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
			if body.CanAttachSelf {
				t.Fatal("response canAttachSelf = true, want false once she is on it")
			}
			if len(body.Doulas) != 1 || body.Doulas[0].StaffID != staffID || body.Doulas[0].Name != doulaName {
				t.Fatalf("response doulas = %+v, want her alone", body.Doulas)
			}

			rows := readOpenAttachments(t, db, engagementID, staffID)
			if len(rows) != 1 {
				t.Fatalf("open attachments = %d, want 1", len(rows))
			}
			if rows[0].origin != "granted" || rows[0].attachedBy != staffID {
				t.Fatalf("attachment = %+v, want granted and attached_by herself", rows[0])
			}
			if rows[0].feeCents != nil || rows[0].feeTerms != nil {
				t.Fatalf("attachment carries a fee: %+v", rows[0])
			}

			// The audit trail: one entry, she is the actor, and the diff
			// names her as the Doula.
			if n := countActivityActions(t, db, engagementID, "doula_attached"); n != 1 {
				t.Fatalf("doula_attached entries = %d, want 1", n)
			}
			assertDiff(t, readFactDiff(t, db, engagementID, "doula_attached"), map[string]any{"attachedStaffId": staffID})
			ledger := readActivity(t, db, practiceID, engagementID, uid)
			if len(ledger.Items) != 1 {
				t.Fatalf("ledger items = %d, want 1", len(ledger.Items))
			}
			entry := ledger.Items[0]
			if entry.ActorName != doulaName || entry.Detail != "Put on this Engagement as the Doula: "+doulaName || entry.CreatedAt.IsZero() {
				t.Fatalf("ledger entry = %+v, want who, what and when", entry)
			}

			after := readDetail(t, db, srv, uid, practiceID, engagementID)
			if after.CanAttachSelf {
				t.Fatal("detail canAttachSelf = true after the press, want false")
			}
			if len(after.Doulas) != 1 || after.Doulas[0].Name != doulaName {
				t.Fatalf("detail doulas = %+v, want her alone", after.Doulas)
			}
		})
	}
}

// TestAttachSelfHandler_ContractorDoula: a plain contractor is refused
// for her Employment type, and the answer does not change with the
// Engagement, so it tells her nothing about one she cannot read. On an
// Engagement she holds an Attachment on (her own acceptance of an Offer)
// the read draws no control.
func TestAttachSelfHandler_ContractorDoula(t *testing.T) {
	db := testdb.New(t)
	const uid = "attach-self-contractor"
	srv, practiceID, staffID, engagementID := newAttachSelfServer(t, db, uid, "Lena Vasquez", []string{doulaRole}, contractorType, engagement.StatusIntake)

	status, body := attachSelfAs(t, db, srv, uid, practiceID, engagementID)
	if status != http.StatusForbidden || body.Code != apierr.CodeForbidden {
		t.Fatalf("status = %d code = %q, want 403 FORBIDDEN", status, body.Code)
	}
	missing, missingBody := attachSelfAs(t, db, srv, uid, practiceID, "11111111-1111-1111-1111-111111111111")
	if missing != status || missingBody.Message != body.Message {
		t.Fatalf("refusal changed with the Engagement: %d %q against %d %q", missing, missingBody.Message, status, body.Message)
	}
	if got := readOpenAttachments(t, db, engagementID, staffID); len(got) != 0 {
		t.Fatalf("a refused press wrote an attachment: %+v", got)
	}

	testdb.SeedGrantedAttachment(t, db, engagementID, staffID)
	if d := readDetail(t, db, srv, uid, practiceID, engagementID); d.CanAttachSelf {
		t.Fatal("detail canAttachSelf = true for a contractor, want false")
	}
}

// TestAttachSelfHandler_SafeToRepeat: a person already on the Engagement
// gets the same 200 and nothing is written, so a double press or a retry
// leaves one row and one ledger entry.
func TestAttachSelfHandler_SafeToRepeat(t *testing.T) {
	db := testdb.New(t)
	const uid = "attach-self-repeat"
	srv, practiceID, staffID, engagementID := newAttachSelfServer(t, db, uid, doulaName, []string{doulaRole}, employeeType, engagement.StatusActive)

	first, firstBody := attachSelfAs(t, db, srv, uid, practiceID, engagementID)
	second, secondBody := attachSelfAs(t, db, srv, uid, practiceID, engagementID)
	if first != http.StatusOK || second != http.StatusOK {
		t.Fatalf("statuses = %d, %d, want 200 twice", first, second)
	}
	if len(secondBody.Doulas) != len(firstBody.Doulas) || secondBody.CanAttachSelf {
		t.Fatalf("second answer = %+v, want the first again", secondBody)
	}
	if got := readOpenAttachments(t, db, engagementID, staffID); len(got) != 1 {
		t.Fatalf("open attachments = %d, want 1", len(got))
	}
	if n := countActivityActions(t, db, engagementID, "doula_attached"); n != 1 {
		t.Fatalf("doula_attached entries = %d, want 1", n)
	}
}

// TestAttachSelfHandler_AlreadyGrantedByAnotherWriter: an Attachment an
// earlier writer granted (a Visit that named her, here with a different
// attached_by) is left as it is. The press does not rewrite who attached
// her, and the read draws no control.
func TestAttachSelfHandler_AlreadyGrantedByAnotherWriter(t *testing.T) {
	db := testdb.New(t)
	const uid = "attach-self-already"
	srv, practiceID, staffID, engagementID := newAttachSelfServer(t, db, uid, doulaName, []string{doulaRole}, employeeType, engagement.StatusIntake)
	adminID := testdb.SeedStaffAtPractice(t, db, practiceID, "attach-self-already-admin", []string{adminRole}, employeeType)
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO engagement_attachments (engagement_id, staff_id, origin, attached_by) VALUES ($1, $2, 'granted', $3)`,
		engagementID, staffID, adminID,
	); err != nil {
		t.Fatalf("seed attachment: %v", err)
	}

	if d := readDetail(t, db, srv, uid, practiceID, engagementID); d.CanAttachSelf {
		t.Fatal("detail canAttachSelf = true for a person already on it, want false")
	}
	status, _ := attachSelfAs(t, db, srv, uid, practiceID, engagementID)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	rows := readOpenAttachments(t, db, engagementID, staffID)
	if len(rows) != 1 || rows[0].attachedBy != adminID {
		t.Fatalf("attachment = %+v, want the earlier writer's row unchanged", rows)
	}
	if n := countActivityActions(t, db, engagementID, "doula_attached"); n != 0 {
		t.Fatalf("doula_attached entries = %d, want 0", n)
	}
}

// TestAttachSelfHandler_UpgradesAnAccruedAttachment: an employee Doula
// who has done work on the Engagement holds an accrued Attachment, which
// is a record of work and not the claim (CONTEXT.md, Attachment). She is
// not "the Doula" on any screen, so she has the control, and the press
// upgrades the row in place.
func TestAttachSelfHandler_UpgradesAnAccruedAttachment(t *testing.T) {
	db := testdb.New(t)
	const uid = "attach-self-accrued"
	srv, practiceID, staffID, engagementID := newAttachSelfServer(t, db, uid, doulaName, []string{doulaRole}, employeeType, engagement.StatusActive)
	testdb.SeedAttachment(t, db, engagementID, staffID, "accrued", false)

	if d := readDetail(t, db, srv, uid, practiceID, engagementID); !d.CanAttachSelf || len(d.Doulas) != 0 {
		t.Fatalf("detail = canAttachSelf %v, doulas %v; want the control and nobody named", d.CanAttachSelf, d.Doulas)
	}
	status, _ := attachSelfAs(t, db, srv, uid, practiceID, engagementID)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	rows := readOpenAttachments(t, db, engagementID, staffID)
	if len(rows) != 1 || rows[0].origin != "granted" {
		t.Fatalf("attachments = %+v, want the one row, granted", rows)
	}
	if n := countActivityActions(t, db, engagementID, "doula_attached"); n != 1 {
		t.Fatalf("doula_attached entries = %d, want 1", n)
	}
}

// TestAttachSelfHandler_RefusesACompletedEngagement is the rule the
// Offer writer already has (offer.requireOpenEngagement): completion ends
// every Attachment, so a new one on a completed Engagement is the state
// completion just cleared. intake and active both admit the press.
func TestAttachSelfHandler_RefusesACompletedEngagement(t *testing.T) {
	db := testdb.New(t)
	const uid = "attach-self-completed"
	srv, practiceID, staffID, engagementID := newAttachSelfServer(t, db, uid, doulaName, []string{doulaRole}, employeeType, engagement.StatusCompleted)

	if d := readDetail(t, db, srv, uid, practiceID, engagementID); d.CanAttachSelf {
		t.Fatal("detail canAttachSelf = true on a completed Engagement, want false")
	}
	status, body := attachSelfAs(t, db, srv, uid, practiceID, engagementID)
	if status != http.StatusConflict || body.Code != apierr.CodeConflict {
		t.Fatalf("status = %d code = %q, want 409 CONFLICT", status, body.Code)
	}
	if got := readOpenAttachments(t, db, engagementID, staffID); len(got) != 0 {
		t.Fatalf("a refused press wrote an attachment: %+v", got)
	}
}

// TestAttachSelfHandler_UnknownEngagement: an Engagement at another
// Practice is not found, the same as one that does not exist, and a
// malformed id is a 400.
func TestAttachSelfHandler_UnknownEngagement(t *testing.T) {
	db := testdb.New(t)
	const uid = "attach-self-unknown"
	srv, practiceID, _, _ := newAttachSelfServer(t, db, uid, doulaName, []string{doulaRole}, employeeType, engagement.StatusIntake)
	otherPractice := testdb.SeedPractice(t, db, "Another Practice")
	_, otherEngagement := testdb.SeedEngagement(t, db, otherPractice)

	if status, _ := attachSelfAs(t, db, srv, uid, practiceID, otherEngagement); status != http.StatusNotFound {
		t.Fatalf("another Practice's Engagement: status = %d, want 404", status)
	}
	if status, _ := attachSelfAs(t, db, srv, uid, practiceID, "11111111-1111-1111-1111-111111111111"); status != http.StatusNotFound {
		t.Fatalf("unknown Engagement: status = %d, want 404", status)
	}
	if status, _ := attachSelfAs(t, db, srv, uid, practiceID, "not-a-uuid"); status != http.StatusBadRequest {
		t.Fatalf("malformed id: status = %d, want 400", status)
	}
}

// TestDetailHandler_DoulasNamesEveryGrantedAttachment: the Engagement
// says who is on it. Only an open, granted Attachment names a Doula; an
// accrued one and an ended one do not. A colleague on it does not take
// the reader's own control away.
func TestDetailHandler_DoulasNamesEveryGrantedAttachment(t *testing.T) {
	db := testdb.New(t)
	const uid = "detail-doulas"
	srv, practiceID, _, engagementID := newAttachSelfServer(t, db, uid, doulaName, []string{doulaRole}, employeeType, engagement.StatusIntake)
	granted := testdb.SeedNamedStaffAtPractice(t, db, practiceID, "detail-doulas-granted", "Renata Alvarez", []string{doulaRole}, employeeType)
	accrued := testdb.SeedNamedStaffAtPractice(t, db, practiceID, "detail-doulas-accrued", "Priya Shah", []string{doulaRole}, employeeType)
	ended := testdb.SeedNamedStaffAtPractice(t, db, practiceID, "detail-doulas-ended", "Mei Tan", []string{doulaRole}, employeeType)
	testdb.SeedGrantedAttachment(t, db, engagementID, granted)
	testdb.SeedAttachment(t, db, engagementID, accrued, "accrued", false)
	testdb.SeedAttachment(t, db, engagementID, ended, "granted", true)

	d := readDetail(t, db, srv, uid, practiceID, engagementID)
	if len(d.Doulas) != 1 || d.Doulas[0].StaffID != granted || d.Doulas[0].Name != "Renata Alvarez" {
		t.Fatalf("doulas = %+v, want Renata Alvarez alone", d.Doulas)
	}
	if !d.CanAttachSelf {
		t.Fatal("canAttachSelf = false with a colleague on it, want true")
	}
}
