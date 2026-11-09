package website_test

import (
	"database/sql"
	"encoding/json/v2"
	"net/http"
	"strings"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/website"
)

// TestGetHandler_UndeclaredIsAnAnswerNotAnError proves a Practice that
// has never answered reads as "undeclared" with a 200. #442's payments
// screen asks this endpoint whether the Stripe button may be offered,
// and a 404 there would be a state to special-case rather than an answer.
func TestGetHandler_UndeclaredIsAnAnswerNotAnError(t *testing.T) {
	e := newEnv(t)

	got := e.get(e.SignedIn)

	if got.Mode != website.ModeUndeclared {
		t.Fatalf("mode = %q, want %q", got.Mode, website.ModeUndeclared)
	}
	if got.UpdatedBy != "" || got.UpdatedAt != "" {
		t.Fatalf("audit fields = %q/%q, want empty", got.UpdatedBy, got.UpdatedAt)
	}
}

// TestPutHandler_OwnerDeclaresHerOwnWebsite proves the "own" answer is
// stored normalized, and read back with who declared it and when.
func TestPutHandler_OwnerDeclaresHerOwnWebsite(t *testing.T) {
	e := newEnv(t)

	got := e.put(`{"mode":"own","ownUrl":"facebook.com/RochesterDoulas"}`)

	if got.Mode != website.ModeOwn {
		t.Fatalf("mode = %q, want %q", got.Mode, website.ModeOwn)
	}
	if got.OwnURL != "https://facebook.com/RochesterDoulas" {
		t.Fatalf("ownUrl = %q, want the normalized form", got.OwnURL)
	}
	if got.UpdatedBy != ownerName {
		t.Fatalf("updatedBy = %q, want the Owner who declared it", got.UpdatedBy)
	}
	if got.UpdatedAt == "" {
		t.Fatal("updatedAt is empty, want the moment she declared it")
	}
}

// TestPutHandler_OwnerPublishesAHostedPage proves the hosted answer
// stores exactly the two facts only she has.
func TestPutHandler_OwnerPublishesAHostedPage(t *testing.T) {
	e := newEnv(t)

	got := e.put(`{"mode":"hosted","serviceDescription":"Birth and postpartum doula support in Monroe County.","cancellationPolicy":"Two weeks' notice for a full refund."}`)

	if got.Mode != website.ModeHosted {
		t.Fatalf("mode = %q, want %q", got.Mode, website.ModeHosted)
	}
	if got.ServiceDescription != "Birth and postpartum doula support in Monroe County." {
		t.Fatalf("serviceDescription = %q", got.ServiceDescription)
	}
	if got.CancellationPolicy != "Two weeks' notice for a full refund." {
		t.Fatalf("cancellationPolicy = %q", got.CancellationPolicy)
	}
}

// TestPutHandler_ChangingHerMindKeepsWhatSheAlreadyWrote proves the
// choice is reversible without retyping: publishing a page, switching to
// her own site, and switching back leaves her cancellation policy where
// she left it.
func TestPutHandler_ChangingHerMindKeepsWhatSheAlreadyWrote(t *testing.T) {
	e := newEnv(t)
	e.put(hostedBody)

	afterSwitch := e.put(ownBody)
	if afterSwitch.Mode != website.ModeOwn {
		t.Fatalf("mode = %q, want %q", afterSwitch.Mode, website.ModeOwn)
	}
	if afterSwitch.CancellationPolicy != policyText {
		t.Fatalf("cancellationPolicy = %q, want it carried forward", afterSwitch.CancellationPolicy)
	}

	afterBack := e.put(hostedBody)
	if afterBack.Mode != website.ModeHosted {
		t.Fatalf("mode = %q, want %q", afterBack.Mode, website.ModeHosted)
	}
	if afterBack.OwnURL != ownSiteURL {
		t.Fatalf("ownUrl = %q, want the earlier declaration still there", afterBack.OwnURL)
	}
}

// TestPutHandler_RecordsWhoChangedItAndWhen proves the audit trail: one
// row per act, the first with no previous mode, each snapshotting what
// the page said at the time.
func TestPutHandler_RecordsWhoChangedItAndWhen(t *testing.T) {
	e := newEnv(t)
	e.put(hostedBody)
	e.put(ownBody)

	rows, err := e.DB.Admin.QueryContext(t.Context(),
		`SELECT previous_mode, mode, own_url, service_description, actor_staff_id
		   FROM practice_website_events WHERE practice_id = $1 ORDER BY created_at`, e.Practice.ID)
	if err != nil {
		t.Fatalf("query events: %v", err)
	}
	defer rows.Close()

	type event struct {
		previousMode, mode, ownURL, description, actor *string
	}
	var events []event
	for rows.Next() {
		var ev event
		if err := rows.Scan(&ev.previousMode, &ev.mode, &ev.ownURL, &ev.description, &ev.actor); err != nil {
			t.Fatalf("scan event: %v", err)
		}
		events = append(events, ev)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate events: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
	if events[0].previousMode != nil {
		t.Fatalf("first event previous_mode = %q, want NULL", *events[0].previousMode)
	}
	if *events[0].mode != website.ModeHosted || *events[0].description != "Birth support." {
		t.Fatalf("first event = %+v, want the published page snapshotted", events[0])
	}
	if events[1].previousMode == nil || *events[1].previousMode != website.ModeHosted {
		t.Fatalf("second event previous_mode = %v, want %q", events[1].previousMode, website.ModeHosted)
	}
	if *events[1].actor != e.Staff.ID {
		t.Fatalf("second event actor = %q, want the Owner %q", *events[1].actor, e.Staff.ID)
	}
}

// TestPutHandler_RepublishingTheSameWordsIsAnAct proves nothing
// short-circuits: the screen prints the date back to her, and a silent
// no-op would leave her looking at an old one after an act she just
// performed.
func TestPutHandler_RepublishingTheSameWordsIsAnAct(t *testing.T) {
	e := newEnv(t)

	e.put(hostedBody)
	e.put(hostedBody)

	count := testdb.Scalar[int](t, e.DB.Admin, `SELECT count(*) FROM practice_website_events WHERE practice_id = $1`, e.Practice.ID)
	if count != 2 {
		t.Fatalf("got %d events, want 2 -- a re-assertion is a real act", count)
	}
}

// TestPutHandler_RefusesANonOwner proves who may set this is enforced at
// the boundary, not only in the screen. An Admin or a Doula who can read
// the answer can neither declare her own site nor publish a page (#1968:
// the Website screen no longer draws either button for her, so this is
// the only thing that refuses).
func TestPutHandler_RefusesANonOwner(t *testing.T) {
	bodies := map[string]string{"own": ownBody, "hosted": hostedBody}
	roles := map[string]fixture.StaffOption{"admin": fixture.Admin(), "doula": fixture.Doula()}
	for role, option := range roles {
		for mode, body := range bodies {
			t.Run(role+"/"+mode, func(t *testing.T) {
				e := newEnv(t, signedInAs(option, fixture.StaffName("Ana Reyes")))

				reqtest.ExpectStatus(t, http.MethodPut, e.websiteURL(), http.StatusForbidden, e.SignedIn, rawBody(body))

				count := testdb.Scalar[int](t, e.DB.Admin, `SELECT count(*) FROM practice_websites WHERE practice_id = $1`, e.Practice.ID)
				if count != 0 {
					t.Fatalf("got %d rows, want none written by a non-Owner", count)
				}
			})
		}
	}
}

// TestGetHandler_ADoulaMayRead proves the read is open to every Staff
// member: #442's payments screen has to tell whoever opens it what is
// outstanding rather than show an empty panel.
func TestGetHandler_ADoulaMayRead(t *testing.T) {
	e := newEnv(t)
	e.put(hostedBody)
	doula := e.Colleague(fixture.StaffName("Ana Reyes"))

	got := e.get(e.As(doula))

	if got.Mode != website.ModeHosted {
		t.Fatalf("mode = %q, want %q", got.Mode, website.ModeHosted)
	}
	if got.UpdatedBy != ownerName {
		t.Fatalf("updatedBy = %q, want the Owner who published", got.UpdatedBy)
	}
}

// TestPutHandler_RefusesAMalformedBody proves a body that is not JSON is
// a 400 with the structured shape docs/api-design.md section 7 asks for.
func TestPutHandler_RefusesAMalformedBody(t *testing.T) {
	e := newEnv(t)

	got := apierrtest.Expect(t, http.MethodPut, e.websiteURL(), http.StatusBadRequest, e.SignedIn, rawBody(`not json`))

	if got.Code != apierr.CodeInvalidArgument || got.Message != website.MsgInvalidBody {
		t.Fatalf("error = %+v, want INVALID_ARGUMENT/%q", got, website.MsgInvalidBody)
	}
}

// TestPutHandler_NamesTheFieldThatFailed proves the field-level details
// reach the client, so the screen can put each message beside the input
// it is about rather than in one heap at the top. The character budget is
// enforced server-side and not only counted down in the browser.
func TestPutHandler_NamesTheFieldThatFailed(t *testing.T) {
	overBudget, err := json.Marshal(website.Request{
		Mode:               website.ModeHosted,
		ServiceDescription: strings.Repeat("a", website.MaxFactLength+1),
		CancellationPolicy: policyText,
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	cases := []struct {
		name  string
		body  string
		field string
		want  string
	}{
		{"a malformed URL", `{"mode":"own","ownUrl":"coming soon"}`, "ownUrl", website.MsgURLMalformed},
		{"past the budget", string(overBudget), "serviceDescription", website.MsgTooLong},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)

			got := apierrtest.Expect(t, http.MethodPut, e.websiteURL(), http.StatusBadRequest, e.SignedIn, rawBody(c.body))

			if got.Details[c.field] != c.want {
				t.Fatalf("details = %v, want %s named", got.Details, c.field)
			}
		})
	}
}

// slugOf reads the stored slug directly, because nothing in the API
// returns it: the page's address is #441's build step's business, and
// the screen has no use for it yet.
func slugOf(t *testing.T, db *testdb.DB, practiceID string) string {
	t.Helper()
	slug := testdb.Scalar[sql.NullString](t, db.Admin, `SELECT slug FROM practice_websites WHERE practice_id = $1`, practiceID)
	return slug.String
}

// TestPutHandler_MintsTheSlugOnceAndNeverAgain is the whole reason the
// slug is a stored column rather than a function of the Practice's name.
// Stripe holds doula.cloud/p/<slug> for the life of the connected
// account and #382 established its review of that URL is ongoing, so a
// slug that followed a rename would point a live review at a 404.
func TestPutHandler_MintsTheSlugOnceAndNeverAgain(t *testing.T) {
	e := newEnv(t)
	e.put(hostedBody)

	if got := slugOf(t, e.DB, e.Practice.ID); got != firstSlug {
		t.Fatalf("slug = %q, want %q", got, firstSlug)
	}

	// She renames the Practice, then republishes.
	e.Exec(`UPDATE practices SET name = 'Genesee Birth Collective' WHERE id = $1`, e.Practice.ID)
	e.put(hostedBody)

	if got := slugOf(t, e.DB, e.Practice.ID); got != firstSlug {
		t.Fatalf("slug after a rename = %q, want it unmoved", got)
	}

	// She switches to her own website and back. The slug survives that
	// too, so switching back republishes the address Stripe already has
	// rather than minting a second one.
	e.put(ownBody)
	if got := slugOf(t, e.DB, e.Practice.ID); got != firstSlug {
		t.Fatalf("slug after switching to her own site = %q, want it kept", got)
	}

	e.put(hostedBody)
	if got := slugOf(t, e.DB, e.Practice.ID); got != firstSlug {
		t.Fatalf("slug after switching back = %q, want it kept", got)
	}
}

// TestPutHandler_TwoPracticesOfTheSameNameGetDifferentAddresses proves
// the collision retry. It is a retry rather than a lookup because RLS
// hides the other Practice's row from this transaction entirely -- the
// unique index is the only thing in the system that knows a slug is
// taken, so the write asks it by trying.
func TestPutHandler_TwoPracticesOfTheSameNameGetDifferentAddresses(t *testing.T) {
	first := newEnv(t)
	first.put(hostedBody)

	// A second Practice of the same name is exactly the collision under test.
	secondPractice := fixture.Practice(t, first.DB, fixture.PracticeName(practiceName))
	secondOwner := fixture.Staff(t, first.DB, secondPractice, fixture.Owner(), fixture.StaffName(ownerName))
	reqtest.ExpectStatus(t, http.MethodPut, first.Srv.URL+"/api/practices/"+secondPractice.ID+"/website", http.StatusOK, first.As(secondOwner), rawBody(hostedBody))

	if got := slugOf(t, first.DB, first.Practice.ID); got != firstSlug {
		t.Fatalf("first slug = %q", got)
	}
	if got := slugOf(t, first.DB, secondPractice.ID); got != "rochester-doulas-2" {
		t.Fatalf("second slug = %q, want %q", got, "rochester-doulas-2")
	}

	// The audit row still landed: the retry is a savepoint, not a lost
	// transaction, so the record of who published survives the collision.
	events := testdb.Scalar[int](t, first.DB.Admin, `SELECT count(*) FROM practice_website_events WHERE practice_id = $1`, secondPractice.ID)
	if events != 1 {
		t.Fatalf("got %d events for the second Practice, want 1", events)
	}
}

// #443: publishing a page queues a rebuild, because the deploy workflow
// fires on a push touching site/** and this produces no commit at all.
func TestPutHandler_PublishingQueuesARebuild(t *testing.T) {
	e := newEnv(t)

	body := e.put(hostedBody)

	if got := queuedRebuilds(t, e.DB, e.Practice.ID); got != 1 {
		t.Fatalf("queued %d rebuilds, want 1", got)
	}
	// And the page reads as unproven until something has actually
	// fetched it: absence of a report is never a pass.
	if body.PageState != website.PageStatePending {
		t.Fatalf("pageState = %q, want %q", body.PageState, website.PageStatePending)
	}
	if body.PageURL != website.HostedPageURL(firstSlug) {
		t.Fatalf("pageUrl = %q, want the published address", body.PageURL)
	}
}

// An edit is as much a reason to rebuild as a first publish, and it
// puts the page back to unproven -- whatever a probe found before is
// about words she has just changed.
func TestPutHandler_EditingQueuesAnotherRebuildAndResetsTheState(t *testing.T) {
	e := newEnv(t)
	e.put(hostedBody)
	markPageLive(t, e.DB, e.Practice.ID)

	body := e.put(`{"mode":"hosted","serviceDescription":"Birth and postpartum support.","cancellationPolicy":"` + policyText + `"}`)

	if got := queuedRebuilds(t, e.DB, e.Practice.ID); got != 2 {
		t.Fatalf("queued %d rebuilds, want one per write", got)
	}
	if body.PageState != website.PageStatePending {
		t.Fatalf("pageState = %q, want the earlier probe result cleared", body.PageState)
	}
	if body.PageCheckedAt != "" {
		t.Fatalf("pageCheckedAt = %q, want it cleared with the state", body.PageCheckedAt)
	}
}

// Switching away has to prune her page, which is as much a rebuild as
// publishing was.
func TestPutHandler_SwitchingAwayQueuesARebuildAndDropsTheState(t *testing.T) {
	e := newEnv(t)
	e.put(hostedBody)

	body := e.put(`{"mode":"own","ownUrl":"` + ownSiteURL + `"}`)

	if got := queuedRebuilds(t, e.DB, e.Practice.ID); got != 2 {
		t.Fatalf("queued %d rebuilds, want the prune queued too", got)
	}
	if body.PageState != "" {
		t.Fatalf("pageState = %q, want no state for a Practice with no page here", body.PageState)
	}
	if body.PageURL != "" {
		t.Fatalf("pageUrl = %q, want no link to a page that is no longer built", body.PageURL)
	}
}

// A Practice moving between her own websites has never had a page here,
// so there is nothing to build and nothing to prune.
func TestPutHandler_OwnSiteOnlyQueuesNothing(t *testing.T) {
	e := newEnv(t)

	e.put(ownBody)

	if got := queuedRebuilds(t, e.DB, e.Practice.ID); got != 0 {
		t.Fatalf("queued %d rebuilds, want none", got)
	}
}

// queuedRebuilds counts what #443's outbox holds for one Practice.
func queuedRebuilds(t *testing.T, db *testdb.DB, practiceID string) int {
	t.Helper()
	return testdb.Scalar[int](t, db.Admin, `SELECT count(*) FROM site_build_outbox WHERE practice_id = $1`, practiceID)
}

// markPageLive stands in for a probe having run, so a test can prove the
// next write clears it.
func markPageLive(t *testing.T, db *testdb.DB, practiceID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practice_websites
		    SET page_state = 'live', page_checked_at = now()
		  WHERE practice_id = $1`, practiceID,
	); err != nil {
		t.Fatalf("mark page live: %v", err)
	}
}
