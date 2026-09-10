package portal_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/pagecursor"
	"doula-cloud/api/internal/portal"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// doulaRole avoids goconst flagging the literal at every seed call below.
const doulaRole = "doula"

// seedEngagementForVisits is seedEngagementForActivity plus a Doula to
// name on a Visit -- an Engagement with no Staff at its Practice cannot
// carry one at all, since visits.staff_id is NOT NULL.
func seedEngagementForVisits(t *testing.T, db *testdb.DB, identityUID, practiceName, doulaName string) (engagementID, doulaID string) {
	t.Helper()
	practiceID, engagementID := seedEngagementForActivity(t, db, identityUID, practiceName)
	doulaID = testdb.SeedNamedStaffAtPractice(t, db, practiceID, identityUID+"-doula", doulaName, []string{doulaRole}, "employee")
	return engagementID, doulaID
}

// seedPortalVisit inserts one Visit, with scheduledAt as an explicit
// instant or -- for the zero value -- no scheduled instant at all, which
// is the row #478 asks this read to leave out.
func seedPortalVisit(t *testing.T, db *testdb.DB, engagementID, staffID string, scheduledAt time.Time) (visitID string) {
	t.Helper()
	var at any
	if !scheduledAt.IsZero() {
		at = scheduledAt
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO visits (engagement_id, staff_id, scheduled_at) VALUES ($1, $2, $3) RETURNING id`,
		engagementID, staffID, at,
	).Scan(&visitID); err != nil {
		t.Fatalf("seed visit: %v", err)
	}
	return visitID
}

func decodeVisits(t *testing.T, resp *http.Response) portal.VisitsResponse {
	t.Helper()
	var out portal.VisitsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}

// TestVisitsHandler_ReturnsScheduledAndPastDistinguishably is #478's
// third acceptance criterion: both are returned, and the consumer can
// tell "Thursday at 2pm" from "she came on 18 August" without asking a
// second time or comparing clocks itself.
func TestVisitsHandler_ReturnsScheduledAndPastDistinguishably(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "portal-visits-both"
	engagementID, doulaID := seedEngagementForVisits(t, db, identityUID, "Visits Practice", "Priya Raman")
	past := seedPortalVisit(t, db, engagementID, doulaID, time.Now().Add(-30*24*time.Hour))
	upcoming := seedPortalVisit(t, db, engagementID, doulaID, time.Now().Add(7*24*time.Hour))

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedActivityGet(t, session, srv.URL+"/api/portal/engagements/"+engagementID+"/visits")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	out := decodeVisits(t, resp)
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
	if out.Items[0].DoulaName != "Priya Raman" {
		t.Fatalf("doulaName = %q, want %q", out.Items[0].DoulaName, "Priya Raman")
	}
	if out.Items[0].ScheduledAt.IsZero() {
		t.Fatal("scheduledAt is zero, want the Visit's own scheduled instant")
	}
}

// TestVisitsHandler_OmitsUnscheduledVisit is #478's own third acceptance
// criterion: created_at is when a Doula typed the row, not when anyone
// visited, so a Visit nobody has scheduled must not appear at all.
func TestVisitsHandler_OmitsUnscheduledVisit(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "portal-visits-unscheduled"
	engagementID, doulaID := seedEngagementForVisits(t, db, identityUID, "Unscheduled Practice", "Priya Raman")
	seedPortalVisit(t, db, engagementID, doulaID, time.Time{})
	scheduled := seedPortalVisit(t, db, engagementID, doulaID, time.Now().Add(48*time.Hour))

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedActivityGet(t, session, srv.URL+"/api/portal/engagements/"+engagementID+"/visits")
	defer resp.Body.Close()

	out := decodeVisits(t, resp)
	if len(out.Items) != 1 || out.Items[0].VisitID != scheduled {
		t.Fatalf("items = %+v, want only the scheduled Visit %q", out.Items, scheduled)
	}
}

// TestVisitsHandler_EmptyEngagementReturnsAnEmptyPage proves the
// postpartum-only and nothing-booked-yet cases answer with an empty
// list rather than a null -- what the section's own empty state renders
// from.
func TestVisitsHandler_EmptyEngagementReturnsAnEmptyPage(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "portal-visits-empty"
	engagementID, _ := seedEngagementForVisits(t, db, identityUID, "Empty Practice", "Priya Raman")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedActivityGet(t, session, srv.URL+"/api/portal/engagements/"+engagementID+"/visits")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(string(body), `"items":[]`) {
		t.Fatalf("body = %s, want an empty items array", body)
	}
}

// TestVisitsHandler_CarriesNoStaffOnlyField is #478's second acceptance
// criterion asserted against the wire, not the struct: a field this DTO
// never declares cannot be added back by accident without this failing.
func TestVisitsHandler_CarriesNoStaffOnlyField(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "portal-visits-no-staff-fields"
	engagementID, doulaID := seedEngagementForVisits(t, db, identityUID, "Fields Practice", "Priya Raman")
	visitID := seedPortalVisit(t, db, engagementID, doulaID, time.Now().Add(24*time.Hour))
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE visits SET notes = 'discussed the birth plan' WHERE id = $1`, visitID,
	); err != nil {
		t.Fatalf("seed visit notes: %v", err)
	}

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedActivityGet(t, session, srv.URL+"/api/portal/engagements/"+engagementID+"/visits")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	for _, forbidden := range []string{"staffId", doulaID, "notes", "discussed the birth plan", "type", "prenatal", "postpartum"} {
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
	db := testdb.New(t)
	const identityUID = "portal-visits-hers"
	hers, doulaID := seedEngagementForVisits(t, db, identityUID, "Her Practice", "Priya Raman")
	someoneElse, otherDoula := seedEngagementForVisits(t, db, "portal-visits-someone-else", "Other Practice", "Other Doula")
	seedPortalVisit(t, db, hers, doulaID, time.Now().Add(24*time.Hour))
	otherVisit := seedPortalVisit(t, db, someoneElse, otherDoula, time.Now().Add(24*time.Hour))

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedActivityGet(t, session, srv.URL+"/api/portal/engagements/"+someoneElse+"/visits")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if strings.Contains(string(body), otherVisit) {
		t.Fatalf("body = %s, want no trace of the other Engagement's Visit", body)
	}
}

// TestVisitsHandler_KeepsAVisitWhoseDoulaHasLeftThePractice is the
// truthfulness rule pointed the other way: "Maya came on 18 August" must
// still be there, name and all, after that Doula leaves. Removing a
// Membership deletes the practice_memberships row that 00009's
// Staff-visible-to-a-Client policy reaches through, so her Staff row
// went invisible to this Client -- under an inner join that took the
// Visit with it, and under the LEFT JOIN it took only her name (#1077).
// 00111 reaches her row through the Visit itself, so both survive.
func TestVisitsHandler_KeepsAVisitWhoseDoulaHasLeftThePractice(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "portal-visits-departed"
	engagementID, doulaID := seedEngagementForVisits(t, db, identityUID, "Departed Practice", "Maya Okonkwo")
	visitID := seedPortalVisit(t, db, engagementID, doulaID, time.Now().Add(-21*24*time.Hour))
	removeMembership(t, db, doulaID)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedActivityGet(t, session, srv.URL+"/api/portal/engagements/"+engagementID+"/visits")
	defer resp.Body.Close()

	out := decodeVisits(t, resp)
	if len(out.Items) != 1 || out.Items[0].VisitID != visitID {
		t.Fatalf("items = %+v, want the Visit %q she came to", out.Items, visitID)
	}
	if out.Items[0].DoulaName != "Maya Okonkwo" {
		t.Fatalf("doulaName = %q, want the name of the Doula who came", out.Items[0].DoulaName)
	}
}

// TestVisitsHandler_StandsInForADoulaWhoDeletedHerLogin is the one case
// 00111 deliberately does not name. ADR-0033's redaction overwrites
// staff.name with an internal string, so there is no name left to show
// and the Practice stands in -- what this Client already read before
// #1077, and the reason 00111's policy excludes a deleted row rather
// than putting "Deleted Staff Member" in front of her.
func TestVisitsHandler_StandsInForADoulaWhoDeletedHerLogin(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "portal-visits-deleted-login"
	engagementID, doulaID := seedEngagementForVisits(t, db, identityUID, "Deleted Login Practice", "Maya Okonkwo")
	visitID := seedPortalVisit(t, db, engagementID, doulaID, time.Now().Add(-21*24*time.Hour))
	removeMembership(t, db, doulaID)
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE staff SET name = $1, deleted_at = now() WHERE id = $2`,
		staffauth.DeletedStaffName, doulaID,
	); err != nil {
		t.Fatalf("redact staff row: %v", err)
	}

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedActivityGet(t, session, srv.URL+"/api/portal/engagements/"+engagementID+"/visits")
	defer resp.Body.Close()

	out := decodeVisits(t, resp)
	if len(out.Items) != 1 || out.Items[0].VisitID != visitID {
		t.Fatalf("items = %+v, want the Visit %q she came to", out.Items, visitID)
	}
	if out.Items[0].DoulaName != "Your practice" {
		t.Fatalf("doulaName = %q, want the Practice standing in for a name that no longer exists", out.Items[0].DoulaName)
	}
}

// removeMembership is what "she left the Practice" is in the schema: the
// practice_memberships row goes, and with it 00009's reach to her Staff
// row.
func removeMembership(t *testing.T, db *testdb.DB, staffID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`DELETE FROM practice_memberships WHERE staff_id = $1`, staffID,
	); err != nil {
		t.Fatalf("remove membership: %v", err)
	}
}

// TestVisitsHandler_RejectsAnUnreadableCursor keeps the envelope's
// contract (docs/api-design.md section 4): a cursor the server did not
// write is a client error, not an empty page.
func TestVisitsHandler_RejectsAnUnreadableCursor(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "portal-visits-bad-cursor"
	engagementID, _ := seedEngagementForVisits(t, db, identityUID, "Cursor Practice", "Priya Raman")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedActivityGet(t, session, srv.URL+"/api/portal/engagements/"+engagementID+"/visits?cursor=not-a-cursor")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestVisitsHandler_PagesFromItsOwnCursor proves the envelope pages: a
// full page reports hasMore with a cursor, and following it returns the
// next Visit and no repeat of the first page's last row.
func TestVisitsHandler_PagesFromItsOwnCursor(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "portal-visits-paging"
	engagementID, doulaID := seedEngagementForVisits(t, db, identityUID, "Paging Practice", "Priya Raman")
	base := time.Now().Add(-400 * 24 * time.Hour)
	// 31 Visits: one more than a page, each an hour after the last, so
	// the DESC order is unambiguous.
	for i := range 31 {
		seedPortalVisit(t, db, engagementID, doulaID, base.Add(time.Duration(i)*time.Hour))
	}

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	first := authedActivityGet(t, session, srv.URL+"/api/portal/engagements/"+engagementID+"/visits")
	defer first.Body.Close()
	page1 := decodeVisits(t, first)
	if len(page1.Items) != 30 || !page1.HasMore || page1.NextCursor == nil {
		t.Fatalf("first page = %d items, hasMore=%v, cursor=%v; want 30, true, a cursor", len(page1.Items), page1.HasMore, page1.NextCursor)
	}
	if _, err := pagecursor.Decode(*page1.NextCursor); err != nil {
		t.Fatalf("next cursor is not decodable: %v", err)
	}

	second := authedActivityGet(t, session, srv.URL+"/api/portal/engagements/"+engagementID+"/visits?cursor="+*page1.NextCursor)
	defer second.Body.Close()
	page2 := decodeVisits(t, second)
	if len(page2.Items) != 1 || page2.HasMore {
		t.Fatalf("second page = %d items, hasMore=%v; want 1, false", len(page2.Items), page2.HasMore)
	}
	if page2.Items[0].VisitID == page1.Items[29].VisitID {
		t.Fatalf("second page repeats the first page's last Visit %q", page2.Items[0].VisitID)
	}
}
