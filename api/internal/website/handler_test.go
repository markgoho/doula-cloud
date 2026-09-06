package website_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/website"
)

// ownerName is the Owner every handler test seeds, named once because
// the response echoes it back and several tests assert on it.
const ownerName = "Maya Chen"

// Two fixture answers, shared because goconst counts repeats and because
// a test reading "the same words as last time" should be looking at the
// same literal.
const (
	policyText = "Two weeks' notice."
	// The slug seedOwner's Practice name mints (00046).
	firstSlug  = "rochester-doulas"
	ownSiteURL = "https://rochesterdoulas.com"
)

func seedPractice(t *testing.T, db *testdb.DB, name string) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO practices (name) VALUES ($1) RETURNING id`, name,
	).Scan(&id); err != nil {
		t.Fatalf("seed practice %q: %v", name, err)
	}
	return id
}

func seedStaff(t *testing.T, db *testdb.DB, identityUID, name string) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff (identity_uid, name, email, work_state)
		 VALUES ($1, $2, $1 || '@example.com', 'NY') RETURNING id`,
		identityUID, name,
	).Scan(&id); err != nil {
		t.Fatalf("seed staff %q: %v", identityUID, err)
	}
	return id
}

func seedMembership(t *testing.T, db *testdb.DB, practiceID, staffID, roles string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_memberships (practice_id, staff_id, roles, employment_type)
		 VALUES ($1, $2, $3::practice_role[], 'employee')`,
		practiceID, staffID, roles,
	); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
}

// newServer mounts this package's whole surface through website.Mount,
// the same call main.go makes on the real GatedRouter and
// idempotency.Router -- so the Owner gate and the role gate are the real
// ones and not a test's approximation.
func newServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	website.Mount(g, ir, &tasknudge.FakeEnqueuer{})
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

func putWebsite(t *testing.T, srv *httptest.Server, session, practiceID, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut,
		srv.URL+"/api/practices/"+practiceID+"/website", bytes.NewBufferString(body))
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

func getWebsite(t *testing.T, srv *httptest.Server, session, practiceID string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet,
		srv.URL+"/api/practices/"+practiceID+"/website", nil)
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

func decodeWebsite(t *testing.T, resp *http.Response) website.Response {
	t.Helper()
	var out website.Response
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}

func decodeError(t *testing.T, resp *http.Response) apierr.APIError {
	t.Helper()
	var out apierr.APIError
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	return out
}

// seedOwner seeds a Practice and an Owner at it -- the only role that
// may write a website declaration.
func seedOwner(t *testing.T, db *testdb.DB, uid string) (practiceID, staffID string) {
	t.Helper()
	practiceID = seedPractice(t, db, "Rochester Doulas")
	staffID = seedStaff(t, db, uid, ownerName)
	seedMembership(t, db, practiceID, staffID, "{owner}")
	return practiceID, staffID
}

// TestGetHandler_UndeclaredIsAnAnswerNotAnError proves a Practice that
// has never answered reads as "undeclared" with a 200. #442's payments
// screen asks this endpoint whether the Stripe button may be offered,
// and a 404 there would be a state to special-case rather than an answer.
func TestGetHandler_UndeclaredIsAnAnswerNotAnError(t *testing.T) {
	db := testdb.New(t)
	const uid = "website-undeclared"
	practiceID, _ := seedOwner(t, db, uid)

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := getWebsite(t, srv, session, practiceID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	got := decodeWebsite(t, resp)
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
	db := testdb.New(t)
	const uid = "website-own"
	practiceID, _ := seedOwner(t, db, uid)

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := putWebsite(t, srv, session, practiceID,
		`{"mode":"own","ownUrl":"facebook.com/RochesterDoulas"}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	got := decodeWebsite(t, resp)
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
	db := testdb.New(t)
	const uid = "website-hosted"
	practiceID, _ := seedOwner(t, db, uid)

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := putWebsite(t, srv, session, practiceID,
		`{"mode":"hosted","serviceDescription":"Birth and postpartum doula support in Monroe County.","cancellationPolicy":"Two weeks' notice for a full refund."}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	got := decodeWebsite(t, resp)
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
	db := testdb.New(t)
	const uid = "website-switch"
	practiceID, _ := seedOwner(t, db, uid)

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	published := putWebsite(t, srv, session, practiceID,
		`{"mode":"hosted","serviceDescription":"Birth support.","cancellationPolicy":"Two weeks' notice."}`)
	_ = published.Body.Close()

	switched := putWebsite(t, srv, session, practiceID,
		`{"mode":"own","ownUrl":"https://rochesterdoulas.com"}`)
	defer switched.Body.Close()
	afterSwitch := decodeWebsite(t, switched)
	if afterSwitch.Mode != website.ModeOwn {
		t.Fatalf("mode = %q, want %q", afterSwitch.Mode, website.ModeOwn)
	}
	if afterSwitch.CancellationPolicy != policyText {
		t.Fatalf("cancellationPolicy = %q, want it carried forward", afterSwitch.CancellationPolicy)
	}

	back := putWebsite(t, srv, session, practiceID,
		`{"mode":"hosted","serviceDescription":"Birth support.","cancellationPolicy":"Two weeks' notice."}`)
	defer back.Body.Close()
	afterBack := decodeWebsite(t, back)
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
	db := testdb.New(t)
	const uid = "website-audit"
	practiceID, staffID := seedOwner(t, db, uid)

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	first := putWebsite(t, srv, session, practiceID,
		`{"mode":"hosted","serviceDescription":"Birth support.","cancellationPolicy":"Two weeks' notice."}`)
	_ = first.Body.Close()
	second := putWebsite(t, srv, session, practiceID,
		`{"mode":"own","ownUrl":"https://rochesterdoulas.com"}`)
	_ = second.Body.Close()

	rows, err := db.Admin.QueryContext(t.Context(),
		`SELECT previous_mode, mode, own_url, service_description, actor_staff_id
		   FROM practice_website_events WHERE practice_id = $1 ORDER BY created_at`, practiceID)
	if err != nil {
		t.Fatalf("query events: %v", err)
	}
	defer rows.Close()

	type event struct {
		previousMode, mode, ownURL, description, actor *string
	}
	var events []event
	for rows.Next() {
		var e event
		if err := rows.Scan(&e.previousMode, &e.mode, &e.ownURL, &e.description, &e.actor); err != nil {
			t.Fatalf("scan event: %v", err)
		}
		events = append(events, e)
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
	if *events[1].actor != staffID {
		t.Fatalf("second event actor = %q, want the Owner %q", *events[1].actor, staffID)
	}
}

// TestPutHandler_RepublishingTheSameWordsIsAnAct proves nothing
// short-circuits: the screen prints the date back to her, and a silent
// no-op would leave her looking at an old one after an act she just
// performed.
func TestPutHandler_RepublishingTheSameWordsIsAnAct(t *testing.T) {
	db := testdb.New(t)
	const uid = "website-republish"
	practiceID, _ := seedOwner(t, db, uid)

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	const body = `{"mode":"hosted","serviceDescription":"Birth support.","cancellationPolicy":"Two weeks' notice."}`
	first := putWebsite(t, srv, session, practiceID, body)
	_ = first.Body.Close()
	second := putWebsite(t, srv, session, practiceID, body)
	_ = second.Body.Close()

	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM practice_website_events WHERE practice_id = $1`, practiceID,
	).Scan(&count); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if count != 2 {
		t.Fatalf("got %d events, want 2 -- a re-assertion is a real act", count)
	}
}

// TestPutHandler_RefusesANonOwner proves who may set this is enforced at
// the boundary, not only in the screen. A Doula who can read the answer
// cannot write it.
func TestPutHandler_RefusesANonOwner(t *testing.T) {
	db := testdb.New(t)
	const uid = "website-doula"
	practiceID := seedPractice(t, db, "Rochester Doulas")
	staffID := seedStaff(t, db, uid, "Ana Reyes")
	seedMembership(t, db, practiceID, staffID, "{doula}")

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := putWebsite(t, srv, session, practiceID,
		`{"mode":"own","ownUrl":"https://rochesterdoulas.com"}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}

	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM practice_websites WHERE practice_id = $1`, practiceID,
	).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("got %d rows, want none written by a non-Owner", count)
	}
}

// TestGetHandler_ADoulaMayRead proves the read is open to every Staff
// member: #442's payments screen has to tell whoever opens it what is
// outstanding rather than show an empty panel.
func TestGetHandler_ADoulaMayRead(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Rochester Doulas")
	ownerID := seedStaff(t, db, "website-read-owner", ownerName)
	seedMembership(t, db, practiceID, ownerID, "{owner}")
	doulaID := seedStaff(t, db, "website-read-doula", "Ana Reyes")
	seedMembership(t, db, practiceID, doulaID, "{doula}")

	ownerSrv, ownerSession := newServer(t, db, "website-read-owner")
	defer ownerSrv.Close()
	published := putWebsite(t, ownerSrv, ownerSession, practiceID,
		`{"mode":"hosted","serviceDescription":"Birth support.","cancellationPolicy":"Two weeks' notice."}`)
	_ = published.Body.Close()

	doulaSrv, doulaSession := newServer(t, db, "website-read-doula")
	defer doulaSrv.Close()
	resp := getWebsite(t, doulaSrv, doulaSession, practiceID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	got := decodeWebsite(t, resp)
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
	db := testdb.New(t)
	const uid = "website-bad-body"
	practiceID, _ := seedOwner(t, db, uid)

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := putWebsite(t, srv, session, practiceID, `not json`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	got := decodeError(t, resp)
	if got.Code != "INVALID_ARGUMENT" || got.Message != website.MsgInvalidBody {
		t.Fatalf("error = %+v, want INVALID_ARGUMENT/%q", got, website.MsgInvalidBody)
	}
}

// TestPutHandler_NamesTheFieldThatFailed proves the field-level details
// reach the client, so the screen can put each message beside the input
// it is about rather than in one heap at the top.
func TestPutHandler_NamesTheFieldThatFailed(t *testing.T) {
	db := testdb.New(t)
	const uid = "website-field-errors"
	practiceID, _ := seedOwner(t, db, uid)

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := putWebsite(t, srv, session, practiceID,
		`{"mode":"own","ownUrl":"coming soon"}`)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	got := decodeError(t, resp)
	if got.Details["ownUrl"] != website.MsgURLMalformed {
		t.Fatalf("details = %v, want ownUrl named", got.Details)
	}
}

// TestPutHandler_RefusesPastTheBudget proves the character budget is
// enforced server-side and not only counted down in the browser.
func TestPutHandler_RefusesPastTheBudget(t *testing.T) {
	db := testdb.New(t)
	const uid = "website-budget"
	practiceID, _ := seedOwner(t, db, uid)

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	body, err := json.Marshal(website.Request{
		Mode:               website.ModeHosted,
		ServiceDescription: strings.Repeat("a", website.MaxFactLength+1),
		CancellationPolicy: policyText,
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	resp := putWebsite(t, srv, session, practiceID, string(body))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	if got := decodeError(t, resp); got.Details["serviceDescription"] != website.MsgTooLong {
		t.Fatalf("details = %v, want serviceDescription over budget", got.Details)
	}
}

// slugOf reads the stored slug directly, because nothing in the API
// returns it: the page's address is #441's build step's business, and
// the screen has no use for it yet.
func slugOf(t *testing.T, db *testdb.DB, practiceID string) string {
	t.Helper()
	var slug sql.NullString
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT slug FROM practice_websites WHERE practice_id = $1`, practiceID,
	).Scan(&slug); err != nil {
		t.Fatalf("read slug: %v", err)
	}
	return slug.String
}

// TestPutHandler_MintsTheSlugOnceAndNeverAgain is the whole reason the
// slug is a stored column rather than a function of the Practice's name.
// Stripe holds doula.cloud/p/<slug> for the life of the connected
// account and #382 established its review of that URL is ongoing, so a
// slug that followed a rename would point a live review at a 404.
func TestPutHandler_MintsTheSlugOnceAndNeverAgain(t *testing.T) {
	db := testdb.New(t)
	const uid = "website-slug-stable"
	practiceID, _ := seedOwner(t, db, uid)

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	published := putWebsite(t, srv, session, practiceID,
		`{"mode":"hosted","serviceDescription":"Birth support.","cancellationPolicy":"Two weeks' notice."}`)
	_ = published.Body.Close()

	if got := slugOf(t, db, practiceID); got != firstSlug {
		t.Fatalf("slug = %q, want %q", got, firstSlug)
	}

	// She renames the Practice, then republishes.
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practices SET name = 'Genesee Birth Collective' WHERE id = $1`, practiceID,
	); err != nil {
		t.Fatalf("rename practice: %v", err)
	}
	republished := putWebsite(t, srv, session, practiceID,
		`{"mode":"hosted","serviceDescription":"Birth support.","cancellationPolicy":"Two weeks' notice."}`)
	_ = republished.Body.Close()

	if got := slugOf(t, db, practiceID); got != firstSlug {
		t.Fatalf("slug after a rename = %q, want it unmoved", got)
	}

	// She switches to her own website and back. The slug survives that
	// too, so switching back republishes the address Stripe already has
	// rather than minting a second one.
	switched := putWebsite(t, srv, session, practiceID,
		`{"mode":"own","ownUrl":"https://rochesterdoulas.com"}`)
	_ = switched.Body.Close()
	if got := slugOf(t, db, practiceID); got != firstSlug {
		t.Fatalf("slug after switching to her own site = %q, want it kept", got)
	}

	back := putWebsite(t, srv, session, practiceID,
		`{"mode":"hosted","serviceDescription":"Birth support.","cancellationPolicy":"Two weeks' notice."}`)
	_ = back.Body.Close()
	if got := slugOf(t, db, practiceID); got != firstSlug {
		t.Fatalf("slug after switching back = %q, want it kept", got)
	}
}

// TestPutHandler_TwoPracticesOfTheSameNameGetDifferentAddresses proves
// the collision retry. It is a retry rather than a lookup because RLS
// hides the other Practice's row from this transaction entirely -- the
// unique index is the only thing in the system that knows a slug is
// taken, so the write asks it by trying.
func TestPutHandler_TwoPracticesOfTheSameNameGetDifferentAddresses(t *testing.T) {
	db := testdb.New(t)

	const firstUID, secondUID = "website-slug-first", "website-slug-second"
	// seedOwner names every Practice "Rochester Doulas", which is
	// exactly the collision under test.
	firstPractice, _ := seedOwner(t, db, firstUID)
	secondPractice, _ := seedOwner(t, db, secondUID)

	const body = `{"mode":"hosted","serviceDescription":"Birth support.","cancellationPolicy":"Two weeks' notice."}`

	firstSrv, firstSession := newServer(t, db, firstUID)
	defer firstSrv.Close()
	first := putWebsite(t, firstSrv, firstSession, firstPractice, body)
	_ = first.Body.Close()

	secondSrv, secondSession := newServer(t, db, secondUID)
	defer secondSrv.Close()
	second := putWebsite(t, secondSrv, secondSession, secondPractice, body)
	defer second.Body.Close()
	if second.StatusCode != http.StatusOK {
		t.Fatalf("second publish: status = %d, want 200", second.StatusCode)
	}

	if got := slugOf(t, db, firstPractice); got != firstSlug {
		t.Fatalf("first slug = %q", got)
	}
	if got := slugOf(t, db, secondPractice); got != "rochester-doulas-2" {
		t.Fatalf("second slug = %q, want %q", got, "rochester-doulas-2")
	}

	// The audit row still landed: the retry is a savepoint, not a lost
	// transaction, so the record of who published survives the collision.
	var events int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM practice_website_events WHERE practice_id = $1`, secondPractice,
	).Scan(&events); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if events != 1 {
		t.Fatalf("got %d events for the second Practice, want 1", events)
	}
}

// #443: publishing a page queues a rebuild, because the deploy workflow
// fires on a push touching hugo/** and this produces no commit at all.
func TestPutHandler_PublishingQueuesARebuild(t *testing.T) {
	db := testdb.New(t)
	const uid = "website-queues-rebuild"
	practiceID, _ := seedOwner(t, db, uid)

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := putWebsite(t, srv, session, practiceID,
		`{"mode":"hosted","serviceDescription":"Birth support.","cancellationPolicy":"`+policyText+`"}`)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got %d, want 200", resp.StatusCode)
	}

	if got := queuedRebuilds(t, db, practiceID); got != 1 {
		t.Fatalf("queued %d rebuilds, want 1", got)
	}
	// And the page reads as unproven until something has actually
	// fetched it: absence of a report is never a pass.
	body := decodeWebsite(t, resp)
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
	db := testdb.New(t)
	const uid = "website-edit-rebuild"
	practiceID, _ := seedOwner(t, db, uid)

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	first := putWebsite(t, srv, session, practiceID,
		`{"mode":"hosted","serviceDescription":"Birth support.","cancellationPolicy":"`+policyText+`"}`)
	_ = first.Body.Close()
	markPageLive(t, db, practiceID)

	second := putWebsite(t, srv, session, practiceID,
		`{"mode":"hosted","serviceDescription":"Birth and postpartum support.","cancellationPolicy":"`+policyText+`"}`)
	defer func() { _ = second.Body.Close() }()

	if got := queuedRebuilds(t, db, practiceID); got != 2 {
		t.Fatalf("queued %d rebuilds, want one per write", got)
	}
	body := decodeWebsite(t, second)
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
	db := testdb.New(t)
	const uid = "website-switch-away"
	practiceID, _ := seedOwner(t, db, uid)

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	first := putWebsite(t, srv, session, practiceID,
		`{"mode":"hosted","serviceDescription":"Birth support.","cancellationPolicy":"`+policyText+`"}`)
	_ = first.Body.Close()

	second := putWebsite(t, srv, session, practiceID, `{"mode":"own","ownUrl":"`+ownSiteURL+`"}`)
	defer func() { _ = second.Body.Close() }()

	if got := queuedRebuilds(t, db, practiceID); got != 2 {
		t.Fatalf("queued %d rebuilds, want the prune queued too", got)
	}
	body := decodeWebsite(t, second)
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
	db := testdb.New(t)
	const uid = "website-own-only"
	practiceID, _ := seedOwner(t, db, uid)

	srv, session := newServer(t, db, uid)
	defer srv.Close()

	resp := putWebsite(t, srv, session, practiceID, `{"mode":"own","ownUrl":"`+ownSiteURL+`"}`)
	defer func() { _ = resp.Body.Close() }()

	if got := queuedRebuilds(t, db, practiceID); got != 0 {
		t.Fatalf("queued %d rebuilds, want none", got)
	}
}

// queuedRebuilds counts what #443's outbox holds for one Practice.
func queuedRebuilds(t *testing.T, db *testdb.DB, practiceID string) int {
	t.Helper()
	var n int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM site_build_outbox WHERE practice_id = $1`, practiceID,
	).Scan(&n); err != nil {
		t.Fatalf("count queued rebuilds: %v", err)
	}
	return n
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
