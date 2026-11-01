package staffauth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/legal"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// The fixture history. The shipped one in api/internal/legal holds no
// material version yet, so these tests give the gate one: the Terms
// change materially on November 1, 2026, and a correction follows on
// November 15 that is not material. The Privacy Policy changes
// materially on December 1, 2026.
const (
	termsMaterial   = "2026-11-01"
	termsCorrection = "2026-11-15"
	privacyMaterial = "2026-12-01"
	firstVersion    = "2026-09-29"
	termsChange     = "Raises the price of a Credit."
	privacyChange   = "Keeps a Visit's notes for seven years."
	termsKey        = "terms"
	privacyKey      = "privacy"
	termsName       = "Terms of Service"
	firstChange     = "First version."
	deletionPath    = "/deletion"
)

// The keys of a terms_accepted diff, and the address an httptest
// request comes from.
const (
	diffTermsVersion   = "termsVersion"
	diffTermsComing    = "termsComing"
	diffPrivacyVersion = "privacyVersion"
	diffPrivacyComing  = "privacyComing"
	diffIPAddress      = "ipAddress"
	loopbackIP         = "127.0.0.1"
)

var fixtureTerms = legal.Document{
	Name: termsName,
	Path: legal.Terms.Path,
	Versions: []legal.Version{
		{Effective: firstVersion, Material: false, Change: firstChange},
		{Effective: termsMaterial, Material: true, Change: termsChange},
		{Effective: termsCorrection, Material: false, Change: "Corrects a typing error."},
	},
}

var fixturePrivacy = legal.Document{
	Name: "Privacy Policy",
	Path: "/privacy",
	Versions: []legal.Version{
		{Effective: firstVersion, Material: false, Change: firstChange},
		{Effective: privacyMaterial, Material: true, Change: privacyChange},
	},
}

// The instants the clock reads in these tests.
var (
	beforeTermsChange = time.Date(2026, 10, 31, 23, 0, 0, 0, time.UTC)
	afterTermsChange  = time.Date(2026, 11, 2, 15, 0, 0, 0, time.UTC)
	afterBothChanges  = time.Date(2026, 12, 2, 15, 0, 0, 0, time.UTC)
)

// useFixtureHistory hands the gate the fixture history for the rest of
// t, and puts the shipped one back after it.
func useFixtureHistory(t *testing.T) {
	t.Helper()
	shipped := staffauth.AgreementDocuments
	staffauth.AgreementDocuments = []legal.Document{fixtureTerms, fixturePrivacy}
	t.Cleanup(func() { staffauth.AgreementDocuments = shipped })
}

// publish records that the operator published a version, the row
// ops's publish act writes (#2486).
func publish(t *testing.T, db *testdb.DB, document, version string, material bool) {
	t.Helper()
	testdb.SeedLegalPublication(t, db, document, version, material, time.Now())
}

// seedAgreement writes a terms_accepted row the way signup does, with
// the diff given.
func seedAgreement(t *testing.T, db *testdb.DB, practiceID, staffID string, diff map[string]any) {
	t.Helper()
	raw, err := json.Marshal(diff)
	if err != nil {
		t.Fatalf("marshal diff: %v", err)
	}
	testdb.SeedActivityWithDiff(t, db, practiceID, activity.SubjectPractice, practiceID,
		staffauth.ActionTermsAccepted, activity.StaffActor(staffID), raw)
}

// agreementServer mounts this package's surface the way main.go does,
// behind a clock a test can move, with a probe that stands for any
// other Practice-scoped request and stubs at the paths the gate leaves
// open.
type agreementServer struct {
	t   *testing.T
	db  *testdb.DB
	srv *httptest.Server
	now time.Time
}

func newAgreementServer(t *testing.T, db *testdb.DB, at time.Time) *agreementServer {
	t.Helper()
	s := &agreementServer{t: t, db: db, now: at}
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	staffauth.Mount(g, ir, db.App, authntest.Verifier{}, authntest.NewFakeAccountManager(), tasknudge.NoOpEnqueuer{}, neverSuppressed)
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	g.Get("/api/practices/{practiceId}/probe", staffauth.AnyStaff, ok)
	g.Get("/api/practices/{practiceId}/export", staffauth.AnyStaff, ok)
	g.Get("/api/practices/{practiceId}/deletion", staffauth.AnyStaff, ok)
	g.GatedWrite("POST /api/practices/{practiceId}/deletion", staffauth.AnyStaff, ok)
	g.GatedWrite("DELETE /api/practices/{practiceId}/deletion", staffauth.AnyStaff, ok)
	g.GatedWrite("DELETE /api/practices/{practiceId}/push-subscriptions", staffauth.AnyStaff, ok)
	g.GatedWrite("POST /api/practices/{practiceId}/push-subscriptions", staffauth.AnyStaff, ok)
	s.srv = httptest.NewServer(clock.Middleware(func() time.Time { return s.now })(mux))
	t.Cleanup(s.srv.Close)
	return s
}

// session signs uid in at the server's instant, with a second factor,
// so the five acts that need one do not refuse first.
func (s *agreementServer) session(uid string) string {
	s.t.Helper()
	return authntest.SeedSessionAt(s.t, s.db.App, uid, s.now)
}

func (s *agreementServer) do(method, session, practiceID, suffix string) *http.Response {
	s.t.Helper()
	req, err := http.NewRequestWithContext(s.t.Context(), method, s.srv.URL+"/api/practices/"+practiceID+suffix, nil)
	if err != nil {
		s.t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.t.Fatalf("request: %v", err)
	}
	return resp
}

// refused reports whether the probe was refused for want of an
// agreement, and fails on any other answer.
func (s *agreementServer) refused(session, practiceID string) bool {
	s.t.Helper()
	resp := s.do(http.MethodGet, session, practiceID, "/probe")
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return false
	}
	if resp.StatusCode != http.StatusForbidden {
		s.t.Fatalf("probe status = %d, want 200 or 403", resp.StatusCode)
	}
	if got := apierrtest.Decode(s.t, resp).Code; got != apierr.CodeTermsAgreementRequired {
		s.t.Fatalf("probe code = %q, want %q", got, apierr.CodeTermsAgreementRequired)
	}
	return true
}

// due reads what the session read says is due.
func (s *agreementServer) due(session, practiceID string) []staffauth.DueVersion {
	s.t.Helper()
	resp := s.do(http.MethodGet, session, practiceID, "/session")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		s.t.Fatalf("session status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var out staffauth.PracticeSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		s.t.Fatalf("decode session: %v", err)
	}
	return out.AgreementDue
}

// agree presses Agree and continue.
func (s *agreementServer) agree(session, practiceID string) {
	s.t.Helper()
	resp := s.do(http.MethodPost, session, practiceID, "/terms-agreement")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		s.t.Fatalf("agree status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
}

// signedUpBefore is a Practice whose Owner agreed at signup to the
// first versions, before anything below was in the history.
func signedUpBefore(t *testing.T, db *testdb.DB, uid string) (practiceID, ownerID string) {
	t.Helper()
	practiceID, ownerID = testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole, adminRole, doulaRole}, employeeType)
	seedAgreement(t, db, practiceID, ownerID, map[string]any{
		diffTermsVersion: firstVersion, diffPrivacyVersion: firstVersion, diffIPAddress: loopbackIP,
	})
	return practiceID, ownerID
}

// #1549's first box: after the date a published material version
// starts, an Owner's Practice-scoped request is refused with the code
// the app sends to the screen, and the session read says what is due,
// from the version's own record. Before the date nothing is refused.
func TestAgreementGate_RefusesAnOwnerAfterAMaterialVersionStarts(t *testing.T) {
	useFixtureHistory(t)
	db := testdb.New(t)
	practiceID, _ := signedUpBefore(t, db, "owner-after-change")
	publish(t, db, termsKey, termsMaterial, true)
	s := newAgreementServer(t, db, beforeTermsChange)
	session := s.session("owner-after-change")

	if s.refused(session, practiceID) {
		t.Fatal("refused an hour before the version starts, want served")
	}
	if due := s.due(session, practiceID); len(due) != 0 {
		t.Fatalf("due before the date = %v, want nothing", due)
	}

	// She signs in again after the date: a Staff session lasts 12 hours.
	s.now = afterTermsChange
	session = s.session("owner-after-change")
	if !s.refused(session, practiceID) {
		t.Fatal("served after the version started, want refused")
	}
	due := s.due(session, practiceID)
	want := staffauth.DueVersion{Document: termsKey, Name: termsName, Version: termsMaterial, Change: termsChange}
	if len(due) != 1 || due[0] != want {
		t.Fatalf("due = %v, want [%v]", due, want)
	}
}

// Both documents can be due at once, the Terms first.
func TestAgreementGate_BothDocumentsDue(t *testing.T) {
	useFixtureHistory(t)
	db := testdb.New(t)
	practiceID, _ := signedUpBefore(t, db, "owner-both")
	publish(t, db, termsKey, termsMaterial, true)
	publish(t, db, privacyKey, privacyMaterial, true)
	s := newAgreementServer(t, db, afterBothChanges)
	session := s.session("owner-both")

	due := s.due(session, practiceID)
	if len(due) != 2 || due[0].Document != termsKey || due[1].Document != privacyKey {
		t.Fatalf("due = %v, want the Terms then the Privacy Policy", due)
	}
	if due[1].Change != privacyChange {
		t.Fatalf("privacy change = %q, want %q", due[1].Change, privacyChange)
	}
}

// #1549's second box: the export, every deletion route, and the push
// unsubscribe that signing out sends are never refused for this
// reason. (Signing out itself and the Credit refund are not
// Practice-scoped; main's TestAgreementGate_RealRoutes walks them.)
func TestAgreementGate_LeavesExportDeletionAndSignOutOpen(t *testing.T) {
	useFixtureHistory(t)
	db := testdb.New(t)
	practiceID, _ := signedUpBefore(t, db, "owner-exempt")
	publish(t, db, termsKey, termsMaterial, true)
	s := newAgreementServer(t, db, afterTermsChange)
	session := s.session("owner-exempt")

	for _, route := range []struct{ method, suffix string }{
		{http.MethodGet, "/session"},
		{http.MethodGet, "/export"},
		{http.MethodGet, deletionPath},
		{http.MethodPost, deletionPath},
		{http.MethodDelete, deletionPath},
		{http.MethodDelete, "/push-subscriptions"},
	} {
		resp := s.do(route.method, session, practiceID, route.suffix)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s %s status = %d, want %d", route.method, route.suffix, resp.StatusCode, http.StatusOK)
		}
	}

	// Subscribing to push is not signing out: it waits like the rest.
	resp := s.do(http.MethodPost, session, practiceID, "/push-subscriptions")
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("POST push-subscriptions status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

// #1549's third box: a Staff member who is not an Owner is never
// refused for this reason and is told nothing is due, an Admin
// included. Before an Owner agrees, the Practice works as usual for her.
func TestAgreementGate_NeverStopsAStaffMemberWhoIsNotAnOwner(t *testing.T) {
	useFixtureHistory(t)
	db := testdb.New(t)
	practiceID, _ := signedUpBefore(t, db, "owner-not-yet")
	testdb.SeedStaffAtPractice(t, db, practiceID, "doula-at-work", []string{doulaRole}, employeeType)
	testdb.SeedStaffAtPractice(t, db, practiceID, "admin-at-work", []string{adminRole, doulaRole}, employeeType)
	publish(t, db, termsKey, termsMaterial, true)
	s := newAgreementServer(t, db, afterTermsChange)

	for _, uid := range []string{"doula-at-work", "admin-at-work"} {
		session := s.session(uid)
		if s.refused(session, practiceID) {
			t.Fatalf("%s refused, want served", uid)
		}
		if due := s.due(session, practiceID); len(due) != 0 {
			t.Fatalf("%s told %v is due, want nothing", uid, due)
		}
	}
	if !s.refused(s.session("owner-not-yet"), practiceID) {
		t.Fatal("the Owner served, want refused: the Practice is still waiting on her")
	}
}

// #1549's fourth box: the press writes one terms_accepted row, the same
// shape signup writes, with the versions in force at the server's
// instant and the address the request came from; the Owner then enters
// her Practice.
func TestAgreeToTerms_RecordsTheAgreementAndOpensThePractice(t *testing.T) {
	useFixtureHistory(t)
	db := testdb.New(t)
	practiceID, ownerID := signedUpBefore(t, db, "owner-agrees")
	publish(t, db, termsKey, termsMaterial, true)
	s := newAgreementServer(t, db, afterTermsChange)
	session := s.session("owner-agrees")

	s.agree(session, practiceID)

	rows := termsAcceptedRows(t, db)
	if len(rows) != 2 {
		t.Fatalf("terms_accepted rows = %d, want the signup's and this one", len(rows))
	}
	r := rows[1]
	if r.subjectKind != activity.SubjectPractice || r.subjectID != practiceID {
		t.Fatalf("subject = %s/%s, want practice/%s", r.subjectKind, r.subjectID, practiceID)
	}
	if r.actorKind != "staff" || r.actorStaffID != ownerID {
		t.Fatalf("actor = %s/%s, want staff/%s", r.actorKind, r.actorStaffID, ownerID)
	}
	assertDiff(t, r.diff, map[string]any{
		diffTermsVersion:   termsMaterial,
		diffTermsComing:    []any{termsCorrection},
		diffPrivacyVersion: firstVersion,
		diffPrivacyComing:  []any{privacyMaterial},
		diffIPAddress:      loopbackIP,
	})
	if !r.createdAt.Equal(afterTermsChange) {
		t.Fatalf("created_at = %s, want the server's clock, %s", r.createdAt, afterTermsChange)
	}
	if s.refused(session, practiceID) {
		t.Fatal("refused after she agreed, want served")
	}

	// A second press writes nothing more.
	s.agree(session, practiceID)
	if rows := termsAcceptedRows(t, db); len(rows) != 2 {
		t.Fatalf("terms_accepted rows after a second press = %d, want 2", len(rows))
	}
}

// Only an Owner may press it.
func TestAgreeToTerms_RefusesAStaffMemberWhoIsNotAnOwner(t *testing.T) {
	useFixtureHistory(t)
	db := testdb.New(t)
	practiceID, _ := signedUpBefore(t, db, "owner-elsewhere")
	testdb.SeedStaffAtPractice(t, db, practiceID, "doula-presses", []string{doulaRole}, employeeType)
	publish(t, db, termsKey, termsMaterial, true)
	s := newAgreementServer(t, db, afterTermsChange)

	resp := s.do(http.MethodPost, s.session("doula-presses"), practiceID, "/terms-agreement")
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
	if rows := termsAcceptedRows(t, db); len(rows) != 1 {
		t.Fatalf("terms_accepted rows = %d, want only the signup's", len(rows))
	}
}

// #1549's fifth box: after one Owner agrees, a second Owner of the same
// Practice is not asked, and her press writes nothing.
func TestAgreementGate_TheFirstOwnerSettlesItForThePractice(t *testing.T) {
	useFixtureHistory(t)
	db := testdb.New(t)
	practiceID, _ := signedUpBefore(t, db, "first-owner")
	testdb.SeedStaffAtPractice(t, db, practiceID, "second-owner", []string{ownerRole, doulaRole}, employeeType)
	publish(t, db, termsKey, termsMaterial, true)
	s := newAgreementServer(t, db, afterTermsChange)
	second := s.session("second-owner")

	if !s.refused(second, practiceID) {
		t.Fatal("the second Owner served before anyone agreed, want refused")
	}
	s.agree(s.session("first-owner"), practiceID)

	if s.refused(second, practiceID) {
		t.Fatal("the second Owner refused after the first agreed, want served")
	}
	if due := s.due(second, practiceID); len(due) != 0 {
		t.Fatalf("the second Owner told %v is due, want nothing", due)
	}
	s.agree(second, practiceID)
	if rows := termsAcceptedRows(t, db); len(rows) != 2 {
		t.Fatalf("terms_accepted rows = %d, want the signup's and the first Owner's", len(rows))
	}
}

// An Owner of two Practices is asked for each: agreeing for one leaves
// the other waiting.
func TestAgreementGate_AnOwnerOfTwoPracticesIsAskedForEach(t *testing.T) {
	useFixtureHistory(t)
	db := testdb.New(t)
	const uid = "owner-of-two"
	first, ownerID := signedUpBefore(t, db, uid)
	second := testdb.SeedPractice(t, db, "Second Practice")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_memberships (practice_id, staff_id, roles, employment_type)
		 VALUES ($1, $2, ARRAY['owner', 'doula']::practice_role[], 'employee')`, second, ownerID,
	); err != nil {
		t.Fatalf("seed second Membership: %v", err)
	}
	seedAgreement(t, db, second, ownerID, map[string]any{
		diffTermsVersion: firstVersion, diffPrivacyVersion: firstVersion, diffIPAddress: loopbackIP,
	})
	publish(t, db, termsKey, termsMaterial, true)
	s := newAgreementServer(t, db, afterTermsChange)
	session := s.session(uid)

	s.agree(session, first)
	if s.refused(session, first) {
		t.Fatal("refused at the Practice she agreed for, want served")
	}
	if !s.refused(session, second) {
		t.Fatal("served at the other Practice, want refused until she agrees for it too")
	}
}

// #1549's sixth box: a version that is not material asks nobody, and
// neither does a material one the operator never published, because
// then no email went before it (ADR-0053 rejects the screen with no
// email before it).
func TestAgreementGate_AsksNothingForAChangeThatIsNotMaterialOrNotPublished(t *testing.T) {
	useFixtureHistory(t)

	t.Run("not material", func(t *testing.T) {
		db := testdb.New(t)
		practiceID, _ := signedUpBefore(t, db, "owner-correction")
		staffauth.AgreementDocuments = []legal.Document{{
			Name: termsName, Path: legal.Terms.Path,
			Versions: []legal.Version{
				{Effective: firstVersion, Material: false, Change: firstChange},
				{Effective: termsCorrection, Material: false, Change: "Corrects a typing error."},
			},
		}}
		publish(t, db, termsKey, termsCorrection, false)
		s := newAgreementServer(t, db, afterBothChanges)
		if s.refused(s.session("owner-correction"), practiceID) {
			t.Fatal("refused for a change that is not material, want served")
		}
	})

	t.Run("not published", func(t *testing.T) {
		db := testdb.New(t)
		practiceID, _ := signedUpBefore(t, db, "owner-unannounced")
		s := newAgreementServer(t, db, afterTermsChange)
		if s.refused(s.session("owner-unannounced"), practiceID) {
			t.Fatal("refused for a version nobody was emailed about, want served")
		}
	})
}

// A newer material version the operator never published is passed over
// for the older one that was: the Practice is still asked about that one,
// and the screen names it.
func TestAgreementGate_AnUnpublishedNewerVersionDoesNotHideAPublishedOne(t *testing.T) {
	useFixtureHistory(t)
	staffauth.AgreementDocuments = []legal.Document{{
		Name: termsName, Path: legal.Terms.Path,
		Versions: []legal.Version{
			{Effective: firstVersion, Material: false, Change: firstChange},
			{Effective: termsMaterial, Material: true, Change: termsChange},
			{Effective: termsCorrection, Material: true, Change: "Changes the refund window."},
		},
	}}
	db := testdb.New(t)
	practiceID, _ := signedUpBefore(t, db, "owner-unpublished-newer")
	publish(t, db, termsKey, termsMaterial, true)
	s := newAgreementServer(t, db, afterBothChanges)
	session := s.session("owner-unpublished-newer")

	due := s.due(session, practiceID)
	if len(due) != 1 || due[0].Version != termsMaterial {
		t.Fatalf("due = %v, want the published %s", due, termsMaterial)
	}
}

// #1549's seventh box, as #1816 reads it: a Practice that signed up on
// the version, during its 30 days of notice, or after a later version,
// has agreed to it and is not asked. A Practice whose only row names an
// older version is.
func TestAgreementGate_APracticeThatAgreedAtSignupIsNotAsked(t *testing.T) {
	useFixtureHistory(t)
	cases := []struct {
		name  string
		diff  map[string]any
		asked bool
	}{
		{"signed up on the version", map[string]any{diffTermsVersion: termsMaterial, diffPrivacyVersion: firstVersion}, false},
		{"signed up during the notice", map[string]any{diffTermsVersion: firstVersion, diffTermsComing: []string{termsMaterial, termsCorrection}, diffPrivacyVersion: firstVersion}, false},
		{"signed up after the correction", map[string]any{diffTermsVersion: termsCorrection, diffPrivacyVersion: firstVersion}, false},
		{"signed up before the version", map[string]any{diffTermsVersion: firstVersion, diffPrivacyVersion: firstVersion, diffPrivacyComing: []string{privacyMaterial}}, true},
		// A later version still coming is not this one: that row was
		// written before this version was in the history.
		{"named only a later version as coming", map[string]any{diffTermsVersion: firstVersion, diffTermsComing: []string{termsCorrection}, diffPrivacyVersion: firstVersion}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testdb.New(t)
			practiceID, ownerID := testdb.SeedStaffAtNewPractice(t, db, "owner-signup-case", []string{ownerRole}, employeeType)
			seedAgreement(t, db, practiceID, ownerID, tc.diff)
			publish(t, db, termsKey, termsMaterial, true)
			s := newAgreementServer(t, db, afterTermsChange)
			if got := s.refused(s.session("owner-signup-case"), practiceID); got != tc.asked {
				t.Fatalf("refused = %v, want %v", got, tc.asked)
			}
		})
	}
}
