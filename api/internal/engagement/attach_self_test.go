package engagement_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
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

// attachSelf presses the control as the Staff member at the screen and
// requires want.
func (e *env) attachSelf(want int) attachSelfBody {
	e.t.Helper()
	return e.attachSelfTo(e.Engagement.ID, want)
}

func (e *env) attachSelfTo(engagementID string, want int) attachSelfBody {
	e.t.Helper()
	return reqtest.Expect[attachSelfBody](e.t, http.MethodPut, e.urlOf(engagementID, "/attachments/me"), want, e.signedIn)
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

// attachments is what the Staff member at the screen holds on the Engagement.
func (e *env) attachments() []attachmentRow {
	e.t.Helper()
	return readOpenAttachments(e.t, e.db, e.Engagement.ID, e.Staff.ID)
}

// countActivity counts the Engagement's activity rows carrying action.
func (e *env) countActivity(action string) int {
	e.t.Helper()
	return countActivityActions(e.t, e.db, e.Engagement.ID, action)
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
		name   string
		opts   []fixture.StaffOption
		wantOK bool
	}{
		{"employee doula", []fixture.StaffOption{fixture.Doula()}, true},
		{"owner who is an employee doula", []fixture.StaffOption{fixture.Owner(), fixture.Doula()}, true},
		{"admin who is an employee doula", []fixture.StaffOption{fixture.Admin(), fixture.Doula()}, true},
		{"owner with no doula role", []fixture.StaffOption{fixture.Owner()}, false},
		{"admin with no doula role", []fixture.StaffOption{fixture.Admin()}, false},
		{"staff with no role", []fixture.StaffOption{fixture.NoRole()}, false},
		{"owner who is a contractor doula", []fixture.StaffOption{fixture.Owner(), fixture.Doula(), fixture.Contractor()}, true},
		{"admin who is a contractor doula", []fixture.StaffOption{fixture.Admin(), fixture.Doula(), fixture.Contractor()}, true},
		{"owner who is a contractor with no doula role", []fixture.StaffOption{fixture.Owner(), fixture.Contractor()}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(append(tc.opts, fixture.StaffName(doulaName))...))

			before := e.detail()
			if before.CanAttachSelf != tc.wantOK {
				t.Fatalf("detail canAttachSelf = %v, want %v", before.CanAttachSelf, tc.wantOK)
			}
			if len(before.Doulas) != 0 {
				t.Fatalf("detail doulas = %v, want none before the press", before.Doulas)
			}

			body := e.attachSelf(okOrForbidden(tc.wantOK))

			if !tc.wantOK {
				if body.Code != apierr.CodeForbidden {
					t.Fatalf("code = %q, want FORBIDDEN", body.Code)
				}
				if body.Message == "" {
					t.Fatal("refusal carries no message, want the reason")
				}
				if got := e.attachments(); len(got) != 0 {
					t.Fatalf("a refused press wrote an attachment: %+v", got)
				}
				if n := e.countActivity("doula_attached"); n != 0 {
					t.Fatalf("doula_attached entries = %d, want 0", n)
				}
				return
			}

			if body.CanAttachSelf {
				t.Fatal("response canAttachSelf = true, want false once she is on it")
			}
			if len(body.Doulas) != 1 || body.Doulas[0].StaffID != e.Staff.ID || body.Doulas[0].Name != doulaName {
				t.Fatalf("response doulas = %+v, want her alone", body.Doulas)
			}

			rows := e.attachments()
			if len(rows) != 1 {
				t.Fatalf("open attachments = %d, want 1", len(rows))
			}
			if rows[0].origin != "granted" || rows[0].attachedBy != e.Staff.ID {
				t.Fatalf("attachment = %+v, want granted and attached_by herself", rows[0])
			}
			if rows[0].feeCents != nil || rows[0].feeTerms != nil {
				t.Fatalf("attachment carries a fee: %+v", rows[0])
			}

			// The audit trail: one entry, she is the actor, and the diff
			// names her as the Doula.
			if n := e.countActivity("doula_attached"); n != 1 {
				t.Fatalf("doula_attached entries = %d, want 1", n)
			}
			assertDiff(t, readFactDiff(t, e.db, e.Engagement.ID, "doula_attached"), map[string]any{"attachedStaffId": e.Staff.ID})
			ledger := e.activityAs(e.Staff)
			if len(ledger.Items) != 1 {
				t.Fatalf("ledger items = %d, want 1", len(ledger.Items))
			}
			entry := ledger.Items[0]
			if entry.ActorName != doulaName || entry.Detail != "Put on this work as the Doula: "+doulaName || entry.CreatedAt.IsZero() {
				t.Fatalf("ledger entry = %+v, want who, what and when", entry)
			}

			after := e.detail()
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
	e := newEnv(t, signedInAs(fixture.Contractor()))

	body := e.attachSelf(http.StatusForbidden)
	missing := e.attachSelfTo("11111111-1111-1111-1111-111111111111", http.StatusForbidden)

	if body.Code != apierr.CodeForbidden {
		t.Fatalf("code = %q, want FORBIDDEN", body.Code)
	}
	if missing.Message != body.Message {
		t.Fatalf("refusal changed with the Engagement: %q against %q", missing.Message, body.Message)
	}
	if got := e.attachments(); len(got) != 0 {
		t.Fatalf("a refused press wrote an attachment: %+v", got)
	}

	testdb.SeedGrantedAttachment(t, e.db, e.Engagement.ID, e.Staff.ID)
	if e.detail().CanAttachSelf {
		t.Fatal("detail canAttachSelf = true for a contractor, want false")
	}
}

// TestAttachSelfHandler_SafeToRepeat: a person already on the Engagement
// gets the same 200 and nothing is written, so a double press or a retry
// leaves one row and one ledger entry.
func TestAttachSelfHandler_SafeToRepeat(t *testing.T) {
	e := newEnv(t, withEngagement(fixture.Status(fixture.StatusActive)))

	first := e.attachSelf(http.StatusOK)
	second := e.attachSelf(http.StatusOK)

	if len(second.Doulas) != len(first.Doulas) || second.CanAttachSelf {
		t.Fatalf("second answer = %+v, want the first again", second)
	}
	if got := e.attachments(); len(got) != 1 {
		t.Fatalf("open attachments = %d, want 1", len(got))
	}
	if n := e.countActivity("doula_attached"); n != 1 {
		t.Fatalf("doula_attached entries = %d, want 1", n)
	}
}

// TestAttachSelfHandler_AlreadyGrantedByAnotherWriter: an Attachment an
// earlier writer granted (a Visit that named her, here with a different
// attached_by) is left as it is. The press does not rewrite who attached
// her, and the read draws no control.
func TestAttachSelfHandler_AlreadyGrantedByAnotherWriter(t *testing.T) {
	e := newEnv(t)
	admin := e.staff(fixture.Admin())
	e.exec(`INSERT INTO engagement_attachments (engagement_id, staff_id, origin, attached_by) VALUES ($1, $2, 'granted', $3)`,
		e.Engagement.ID, e.Staff.ID, admin.ID)

	if e.detail().CanAttachSelf {
		t.Fatal("detail canAttachSelf = true for a person already on it, want false")
	}
	e.attachSelf(http.StatusOK)

	rows := e.attachments()
	if len(rows) != 1 || rows[0].attachedBy != admin.ID {
		t.Fatalf("attachment = %+v, want the earlier writer's row unchanged", rows)
	}
	if n := e.countActivity("doula_attached"); n != 0 {
		t.Fatalf("doula_attached entries = %d, want 0", n)
	}
}

// TestAttachSelfHandler_UpgradesAnAccruedAttachment: an employee Doula
// who has done work on the Engagement holds an accrued Attachment, which
// is a record of work and not the claim (GLOSSARY.md, Attachment). She is
// not "the Doula" on any screen, so she has the control, and the press
// upgrades the row in place.
func TestAttachSelfHandler_UpgradesAnAccruedAttachment(t *testing.T) {
	e := newEnv(t, withEngagement(fixture.Status(fixture.StatusActive)))
	testdb.SeedAttachment(t, e.db, e.Engagement.ID, e.Staff.ID, "accrued", false)

	if d := e.detail(); !d.CanAttachSelf || len(d.Doulas) != 0 {
		t.Fatalf("detail = canAttachSelf %v, doulas %v; want the control and nobody named", d.CanAttachSelf, d.Doulas)
	}
	e.attachSelf(http.StatusOK)

	rows := e.attachments()
	if len(rows) != 1 || rows[0].origin != "granted" {
		t.Fatalf("attachments = %+v, want the one row, granted", rows)
	}
	if n := e.countActivity("doula_attached"); n != 1 {
		t.Fatalf("doula_attached entries = %d, want 1", n)
	}
}

// TestAttachSelfHandler_RefusesACompletedEngagement is the rule the
// Offer writer already has (offer.requireOpenEngagement): completion ends
// every Attachment, so a new one on a completed Engagement is the state
// completion just cleared. intake and active both admit the press.
func TestAttachSelfHandler_RefusesACompletedEngagement(t *testing.T) {
	e := newEnv(t, withEngagement(fixture.Status(fixture.StatusCompleted)))

	if e.detail().CanAttachSelf {
		t.Fatal("detail canAttachSelf = true on a completed Engagement, want false")
	}
	body := e.attachSelf(http.StatusConflict)

	if body.Code != apierr.CodeConflict {
		t.Fatalf("code = %q, want CONFLICT", body.Code)
	}
	if got := e.attachments(); len(got) != 0 {
		t.Fatalf("a refused press wrote an attachment: %+v", got)
	}
}

// TestAttachSelfHandler_UnknownEngagement: an Engagement at another
// Practice is not found, the same as one that does not exist, and a
// malformed id is a 400.
func TestAttachSelfHandler_UnknownEngagement(t *testing.T) {
	e := newEnv(t)
	elsewhere := fixture.Engagement(t, e.db, fixture.Practice(t, e.db))

	tests := []struct {
		name         string
		engagementID string
		want         int
	}{
		{"another Practice's Engagement", elsewhere.ID, http.StatusNotFound},
		{"an Engagement that does not exist", "11111111-1111-1111-1111-111111111111", http.StatusNotFound},
		{"an id that is not a uuid", "not-a-uuid", http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(*testing.T) {
			e.attachSelfTo(tt.engagementID, tt.want)
		})
	}
}

// TestDetailHandler_DoulasNamesEveryGrantedAttachment: the Engagement
// says who is on it. Only an open, granted Attachment names a Doula; an
// accrued one and an ended one do not. A colleague on it does not take
// the reader's own control away.
func TestDetailHandler_DoulasNamesEveryGrantedAttachment(t *testing.T) {
	e := newEnv(t)
	granted := e.staff(fixture.StaffName("Renata Alvarez"))
	accrued := e.staff(fixture.StaffName("Priya Shah"))
	ended := e.staff(fixture.StaffName("Mei Tan"))
	testdb.SeedGrantedAttachment(t, e.db, e.Engagement.ID, granted.ID)
	testdb.SeedAttachment(t, e.db, e.Engagement.ID, accrued.ID, "accrued", false)
	testdb.SeedAttachment(t, e.db, e.Engagement.ID, ended.ID, "granted", true)

	d := e.detail()

	if len(d.Doulas) != 1 || d.Doulas[0].StaffID != granted.ID || d.Doulas[0].Name != "Renata Alvarez" {
		t.Fatalf("doulas = %+v, want Renata Alvarez alone", d.Doulas)
	}
	if !d.CanAttachSelf {
		t.Fatal("canAttachSelf = false with a colleague on it, want true")
	}
}
