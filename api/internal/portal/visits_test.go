package portal_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/pagecursor"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// TestVisitsHandler_ReturnsScheduledAndPastDistinguishably is #478's
// third acceptance criterion: both are returned, and the consumer can
// tell "Thursday at 2pm" from "she came on 18 August" without asking a
// second time or comparing clocks itself.
func TestVisitsHandler_ReturnsScheduledAndPastDistinguishably(t *testing.T) {
	e := newEnv(t)
	past := e.visit(time.Now().Add(-30 * 24 * time.Hour))
	upcoming := e.visit(time.Now().Add(7 * 24 * time.Hour))

	out := e.visitsPage("")
	if len(out.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(out.Items))
	}
	// Furthest-future first, so every scheduled Visit precedes every past
	// one in the one stream -- the ordering the section renders from.
	if out.Items[0].VisitID != upcoming || out.Items[0].HasHappened {
		t.Fatalf("first item = %+v, want the upcoming Visit %q with hasHappened=false", out.Items[0], upcoming)
	}
	if out.Items[1].VisitID != past || !out.Items[1].HasHappened {
		t.Fatalf("second item = %+v, want the past Visit %q with hasHappened=true", out.Items[1], past)
	}
	if out.Items[0].DoulaName != doulaName {
		t.Fatalf("doulaName = %q, want %q", out.Items[0].DoulaName, doulaName)
	}
	if out.Items[0].ScheduledAt.IsZero() {
		t.Fatal("scheduledAt is zero, want the Visit's own scheduled instant")
	}
}

// TestVisitsHandler_OmitsUnscheduledVisit is #478's own third acceptance
// criterion: created_at is when a Doula typed the row, not when anyone
// visited, so a Visit nobody has scheduled must not appear at all.
func TestVisitsHandler_OmitsUnscheduledVisit(t *testing.T) {
	e := newEnv(t)
	e.visit(time.Time{})
	scheduled := e.visit(time.Now().Add(48 * time.Hour))

	out := e.visitsPage("")
	if len(out.Items) != 1 || out.Items[0].VisitID != scheduled {
		t.Fatalf("items = %+v, want only the scheduled Visit %q", out.Items, scheduled)
	}
}

// TestVisitsHandler_EmptyEngagementReturnsAnEmptyPage proves the
// postpartum-only and nothing-booked-yet cases answer with an empty
// list rather than a null -- what the section's own empty state renders
// from.
func TestVisitsHandler_EmptyEngagementReturnsAnEmptyPage(t *testing.T) {
	e := newEnv(t)

	body := reqtest.ExpectBody(t, http.MethodGet, e.engagementURL("/visits"), http.StatusOK, e.AsClient)
	if !strings.Contains(string(body), `"items":[]`) {
		t.Fatalf("body = %s, want an empty items array", body)
	}
}

// TestVisitsHandler_CarriesNoStaffOnlyField is #478's second acceptance
// criterion asserted against the wire, not the struct: a field this DTO
// never declares cannot be added back by accident without this failing.
func TestVisitsHandler_CarriesNoStaffOnlyField(t *testing.T) {
	e := newEnv(t)
	visitID := e.visit(time.Now().Add(24 * time.Hour))
	e.Exec(`UPDATE visits SET notes = 'discussed the birth plan' WHERE id = $1`, visitID)

	body := reqtest.ExpectBody(t, http.MethodGet, e.engagementURL("/visits"), http.StatusOK, e.AsClient)
	for _, forbidden := range []string{"staffId", e.Staff.ID, "notes", "discussed the birth plan", "type", "prenatal", "postpartum"} {
		if strings.Contains(string(body), forbidden) {
			t.Fatalf("body = %s, want no %q in it", body, forbidden)
		}
	}
}

// TestVisitsHandler_RefusesAnotherEngagementsVisits is #478's own
// isolation criterion at the HTTP edge: a Client naming an Engagement
// she does not hold is refused by clientauth.Middleware before the
// handler runs, and her own Visits are not quietly substituted either.
// The RLS half of the same refusal -- the fence that holds if this one
// ever stops -- is rlsguardrail's own case.
func TestVisitsHandler_RefusesAnotherEngagementsVisits(t *testing.T) {
	e := newEnv(t)
	e.visit(time.Now().Add(24 * time.Hour))
	someoneElse := e.AnotherEngagement()
	otherVisit := e.visitFor(someoneElse.ID, e.Colleague().ID, time.Now().Add(24*time.Hour))

	body := reqtest.ExpectBody(t, http.MethodGet, e.URL("/api/portal/engagements/"+someoneElse.ID+"/visits"), http.StatusForbidden, e.AsClient)
	if strings.Contains(string(body), otherVisit) {
		t.Fatalf("body = %s, want no trace of the other Engagement's Visit", body)
	}
}

// TestVisitsHandler_KeepsAVisitWhoseDoulaLeft holds the truthfulness rule
// pointed the other way: "Maya came on 18 August" must still be there
// after that Doula goes. Removing a Membership deletes the
// practice_memberships row that 00009's Staff-visible-to-a-Client policy
// reaches through, so her Staff row went invisible to this Client -- under
// an inner join that took the Visit with it, and under the LEFT JOIN it
// took only her name (#1077). 00111 reaches her row through the Visit
// itself, so both survive.
//
// The second case is the one 00111 deliberately does not name. ADR-0033's
// redaction overwrites staff.name with an internal string, so there is no
// name left to show and the Practice stands in -- what this Client already
// read before #1077, and the reason 00111's policy excludes a deleted row
// rather than putting "Deleted Staff Member" in front of her.
func TestVisitsHandler_KeepsAVisitWhoseDoulaLeft(t *testing.T) {
	tests := []struct {
		name         string
		deletedLogin bool
		wantName     string
	}{
		{"she left the Practice", false, "Maya Okonkwo"},
		{"she also deleted her login", true, "Your practice"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t, fixture.SignedInAs(fixture.StaffName("Maya Okonkwo")))
			visitID := e.visit(time.Now().Add(-21 * 24 * time.Hour))
			testdb.RemoveMembership(t, e.DB, e.Staff.ID)
			if tt.deletedLogin {
				e.Exec(`UPDATE staff SET first_name = $1, last_name = $2, deleted_at = now() WHERE id = $3`,
					staffauth.DeletedStaffFirstName, staffauth.DeletedStaffLastName, e.Staff.ID)
			}

			out := e.visitsPage("")
			if len(out.Items) != 1 || out.Items[0].VisitID != visitID {
				t.Fatalf("items = %+v, want the Visit %q she came to", out.Items, visitID)
			}
			if out.Items[0].DoulaName != tt.wantName {
				t.Fatalf("doulaName = %q, want %q", out.Items[0].DoulaName, tt.wantName)
			}
		})
	}
}

// TestVisitsHandler_RejectsAnUnreadableCursor keeps the envelope's
// contract (docs/api-design.md section 4): a cursor the server did not
// write is a client error, not an empty page.
func TestVisitsHandler_RejectsAnUnreadableCursor(t *testing.T) {
	e := newEnv(t)

	reqtest.ExpectStatus(t, http.MethodGet, e.engagementURL("/visits?cursor=not-a-cursor"), http.StatusBadRequest, e.AsClient)
}

// TestVisitsHandler_PagesFromItsOwnCursor proves the envelope pages: a
// full page reports hasMore with a cursor, and following it returns the
// next Visit and no repeat of the first page's last row.
func TestVisitsHandler_PagesFromItsOwnCursor(t *testing.T) {
	e := newEnv(t)
	base := time.Now().Add(-400 * 24 * time.Hour)
	// 31 Visits: one more than a page, each an hour after the last, so
	// the DESC order is unambiguous.
	for i := range 31 {
		e.visit(base.Add(time.Duration(i) * time.Hour))
	}

	page1 := e.visitsPage("")
	if len(page1.Items) != 30 || !page1.HasMore || page1.NextCursor == nil {
		t.Fatalf("first page = %d items, hasMore=%v, cursor=%v; want 30, true, a cursor", len(page1.Items), page1.HasMore, page1.NextCursor)
	}
	if _, err := pagecursor.Decode(*page1.NextCursor); err != nil {
		t.Fatalf("next cursor is not decodable: %v", err)
	}

	page2 := e.visitsPage("?cursor=" + *page1.NextCursor)
	if len(page2.Items) != 1 || page2.HasMore {
		t.Fatalf("second page = %d items, hasMore=%v; want 1, false", len(page2.Items), page2.HasMore)
	}
	if page2.Items[0].VisitID == page1.Items[29].VisitID {
		t.Fatalf("second page repeats the first page's last Visit %q", page2.Items[0].VisitID)
	}
}
