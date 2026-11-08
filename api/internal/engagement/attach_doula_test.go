package engagement_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
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

// attachDoula puts staffID on engagementID as the Staff member at the
// screen and requires want.
func (e *env) attachDoula(engagementID, staffID string, want int) attachSelfBody {
	e.t.Helper()
	return reqtest.Expect[attachSelfBody](e.t, http.MethodPut, e.urlOf(engagementID, "/attachments/"+staffID), want, e.signedIn)
}

// attachingEnv is the env with the Doula to put on, named
// attachedDoulaName, and the caller named attachingOwnerName.
type attachingEnv struct {
	*env
	Doula fixture.StaffRecord
}

func newAttachingEnv(t *testing.T, caller roleCase, doula []fixture.StaffOption, opts ...envOption) attachingEnv {
	t.Helper()
	opts = append([]envOption{signedInAs(append(caller.opts, fixture.StaffName(attachingOwnerName))...)}, opts...)
	e := attachingEnv{env: newEnv(t, opts...)}
	e.Doula = e.staff(append(doula, fixture.StaffName(attachedDoulaName))...)
	return e
}

func (e attachingEnv) put(want int) attachSelfBody {
	e.t.Helper()
	return e.attachDoula(e.Engagement.ID, e.Doula.ID, want)
}

// TestAttachDoulaHandler_RoleTable is the rule at the boundary that
// enforces it, for who presses and for who is put on.
func TestAttachDoulaHandler_RoleTable(t *testing.T) {
	asContractor := []fixture.StaffOption{fixture.Contractor()}
	cases := []struct {
		name       string
		caller     roleCase
		doula      []fixture.StaffOption
		wantStatus int
		wantCode   apierr.Code
	}{
		{"owner puts an employee doula on", ownerCase, nil, http.StatusOK, ""},
		{"admin puts an employee doula on", adminCase, nil, http.StatusOK, ""},
		{"owner puts on an admin who is a contractor doula", ownerCase, []fixture.StaffOption{fixture.Admin(), fixture.Doula(), fixture.Contractor()}, http.StatusOK, ""},
		{"a doula cannot put a colleague on", employeeDoulaCase, nil, http.StatusForbidden, apierr.CodeForbidden},
		{"a contractor goes on by an offer", ownerCase, asContractor, http.StatusUnprocessableEntity, apierr.CodeInvalidArgument},
		{"a staff member with no doula role", ownerCase, []fixture.StaffOption{fixture.Roles(adminRole)}, http.StatusUnprocessableEntity, apierr.CodeInvalidArgument},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newAttachingEnv(t, tc.caller, tc.doula)

			body := e.put(tc.wantStatus)

			if tc.wantStatus != http.StatusOK {
				if body.Code != tc.wantCode {
					t.Fatalf("code = %q, want %q", body.Code, tc.wantCode)
				}
				if body.Message == "" {
					t.Fatal("refusal carries no message, want the reason")
				}
				if got := readOpenAttachments(t, e.db, e.Engagement.ID, e.Doula.ID); len(got) != 0 {
					t.Fatalf("a refused press wrote an attachment: %+v", got)
				}
				if n := e.countActivity("doula_attached"); n != 0 {
					t.Fatalf("doula_attached entries = %d, want 0", n)
				}
				return
			}

			if len(body.Doulas) != 1 || body.Doulas[0].StaffID != e.Doula.ID || body.Doulas[0].Name != attachedDoulaName {
				t.Fatalf("response doulas = %+v, want the Doula alone", body.Doulas)
			}
			rows := readOpenAttachments(t, e.db, e.Engagement.ID, e.Doula.ID)
			if len(rows) != 1 || rows[0].origin != grantedOrigin || rows[0].attachedBy != e.Staff.ID {
				t.Fatalf("attachment = %+v, want one granted row attached by the caller", rows)
			}
			if rows[0].feeCents != nil || rows[0].feeTerms != nil {
				t.Fatalf("attachment carries a fee: %+v", rows[0])
			}

			// The audit trail: the caller is the actor, the diff and the
			// ledger's sentence name the Doula, and the entry has a time.
			assertDiff(t, readFactDiff(t, e.db, e.Engagement.ID, "doula_attached"), map[string]any{"attachedStaffId": e.Doula.ID})
			ledger := e.activityAs(e.Staff)
			if len(ledger.Items) != 1 {
				t.Fatalf("ledger items = %d, want 1", len(ledger.Items))
			}
			entry := ledger.Items[0]
			if entry.ActorName != attachingOwnerName || entry.Detail != "Put on this work as the Doula: "+attachedDoulaName || entry.CreatedAt.IsZero() {
				t.Fatalf("ledger entry = %+v, want who, whom and when", entry)
			}
		})
	}
}

// TestAttachDoulaHandler_ContractorRefusalNamesTheOffer: the refusal
// names her and the way she does go on.
func TestAttachDoulaHandler_ContractorRefusalNamesTheOffer(t *testing.T) {
	e := newAttachingEnv(t, ownerCase, []fixture.StaffOption{fixture.Contractor()}, withEngagement(fixture.Status(fixture.StatusActive)))

	body := e.put(http.StatusUnprocessableEntity)

	want := attachedDoulaName + " contracts for this Practice, so takes on work by accepting an Offer. Offer this work instead."
	if body.Message != want {
		t.Fatalf("message = %q, want %q", body.Message, want)
	}
}

// TestAttachDoulaHandler_SafeToRepeat: a Doula already on it gets the
// same 200 and nothing more is written.
func TestAttachDoulaHandler_SafeToRepeat(t *testing.T) {
	e := newAttachingEnv(t, ownerCase, nil, withEngagement(fixture.Status(fixture.StatusActive)))

	e.put(http.StatusOK)
	second := e.put(http.StatusOK)

	if len(second.Doulas) != 1 {
		t.Fatalf("second answer doulas = %+v, want one", second.Doulas)
	}
	if n := e.countActivity("doula_attached"); n != 1 {
		t.Fatalf("doula_attached entries = %d, want 1", n)
	}
}

// TestAttachDoulaHandler_ACallerWhoIsADoulaKeepsHerOwnControl: an Owner
// who is a Doula puts a colleague on, and the answer still says she can
// put herself on too.
func TestAttachDoulaHandler_ACallerWhoIsADoulaKeepsHerOwnControl(t *testing.T) {
	e := newAttachingEnv(t, roleCase{"owner and doula", []fixture.StaffOption{fixture.Owner(), fixture.Doula()}}, nil)

	if body := e.put(http.StatusOK); !body.CanAttachSelf {
		t.Fatal("canAttachSelf = false, want true")
	}
}

// TestAttachDoulaHandler_Refusals: a completed Engagement, an unknown or
// another Practice's Engagement, a person not at this Practice, and
// malformed ids.
func TestAttachDoulaHandler_Refusals(t *testing.T) {
	e := newAttachingEnv(t, ownerCase, nil)
	completed := e.anotherEngagement(fixture.Status(fixture.StatusCompleted))
	otherPractice := fixture.Practice(t, e.db)
	otherEngagement := fixture.Engagement(t, e.db, otherPractice)
	stranger := fixture.Staff(t, e.db, otherPractice)
	const missing = "11111111-1111-1111-1111-111111111111"

	cases := []struct {
		name         string
		engagementID string
		staffID      string
		wantStatus   int
	}{
		{"completed", completed.ID, e.Doula.ID, http.StatusConflict},
		{"another practice's engagement", otherEngagement.ID, e.Doula.ID, http.StatusNotFound},
		{"unknown engagement", missing, e.Doula.ID, http.StatusNotFound},
		{"a person at another practice", e.Engagement.ID, stranger.ID, http.StatusUnprocessableEntity},
		{"malformed engagement", malformedUUID, e.Doula.ID, http.StatusBadRequest},
		{"malformed staff", e.Engagement.ID, malformedUUID, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(*testing.T) {
			e.attachDoula(tc.engagementID, tc.staffID, tc.wantStatus)
		})
	}
	if got := readOpenAttachments(t, e.db, e.Engagement.ID, stranger.ID); len(got) != 0 {
		t.Fatalf("a refused press wrote an attachment: %+v", got)
	}
}
