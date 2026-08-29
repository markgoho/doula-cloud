package website_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/staffauth"
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

// newServer mounts both routes the way main.go really does -- the GET
// through GatedRouter with the AnyStaff declaration, the PUT behind
// staffauth.Middleware -- so the Owner gate and the role gate are the
// real ones and not a test's approximation.
func newServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("PUT /practices/{practiceId}/website",
		staffauth.Middleware(db.App)(website.PutHandler()))
	g := staffauth.NewGatedRouter(mux, db.App)
	g.Get("/practices/{practiceId}/website", staffauth.AnyStaff, website.GetHandler())
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

func putWebsite(t *testing.T, srv *httptest.Server, session, practiceID, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut,
		srv.URL+"/practices/"+practiceID+"/website", bytes.NewBufferString(body))
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
		srv.URL+"/practices/"+practiceID+"/website", nil)
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

func decodeError(t *testing.T, resp *http.Response) website.APIError {
	t.Helper()
	var out website.APIError
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
