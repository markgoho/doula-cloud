package staffauth_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// founderUID is the identity the founder's own `staff` row carries in
// every test here; founderRoutes names both routes the gate stands in
// front of, so a refusal is asserted on each rather than on one.
const founderUID = "founder-uid"

// feedbackRepo is the private repository an issue link is built from
// (GITHUB_FEEDBACK_REPO on Deployed).
const feedbackRepo = "markgoho/doula-cloud-feedback"

// absentFeedbackID is a well-formed id that names no piece of Feedback.
const absentFeedbackID = "00000000-0000-4000-8000-000000000000"

var founderRoutes = []string{
	"/api/staff/feedback",
	"/api/staff/feedback/" + absentFeedbackID,
}

// newFounderServer mounts the founder routes alone, with founderStaffID
// as FOUNDER_STAFF_ID.
func newFounderServer(t *testing.T, db *testdb.DB, founderStaffID string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	staffauth.MountFounderRoutes(g, db.App, staffauth.FounderConfig{StaffID: founderStaffID, FeedbackRepo: feedbackRepo})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// reply is one answer from a founder route, read to the end and closed:
// the status and the whole body.
type reply struct {
	status int
	body   []byte
}

// refusal reads the body as docs/api-design.md section 7's envelope.
func (r reply) refusal(t *testing.T) apierr.APIError {
	t.Helper()
	var out apierr.APIError
	if err := json.Unmarshal(r.body, &out); err != nil {
		t.Fatalf("decode error envelope %q: %v", r.body, err)
	}
	return out
}

func getAsSession(t *testing.T, srv *httptest.Server, session, path string) reply {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+path, http.NoBody)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if session != "" {
		authntest.AddSessionCookie(req, session)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return reply{status: resp.StatusCode, body: body}
}

// A Staff member who is not the founder meets the same 404 an unknown
// piece gives, on both routes, so neither route's existence is revealed
// (#1526) -- even though her own session carries a second factor.
func TestFounderOnly_RefusesAStaffMemberWhoIsNotTheFounder(t *testing.T) {
	db := testdb.New(t)
	founderID := testdb.SeedStaff(t, db, founderUID)
	testdb.SeedStaffAtNewPractice(t, db, "an-owner", []string{ownerRole}, employeeType)
	srv := newFounderServer(t, db, founderID)
	session := authntest.SeedSession(t, db.App, "an-owner")

	for _, path := range founderRoutes {
		resp := getAsSession(t, srv, session, path)
		if resp.status != http.StatusNotFound {
			t.Fatalf("GET %s status = %d, want %d", path, resp.status, http.StatusNotFound)
		}
		if got := resp.refusal(t).Message; got != staffauth.MsgFounderRouteNotFound {
			t.Errorf("GET %s message = %q, want %q", path, got, staffauth.MsgFounderRouteNotFound)
		}
	}
}

// A non-founder with no second factor is told nothing about MFA: the
// refusal that names a second factor would say the route is real.
func TestFounderOnly_RefusesANonFounderBeforeItLooksAtHerSecondFactor(t *testing.T) {
	db := testdb.New(t)
	founderID := testdb.SeedStaff(t, db, founderUID)
	testdb.SeedStaff(t, db, "a-doula")
	srv := newFounderServer(t, db, founderID)
	session := authntest.SeedSessionWithSecondFactor(t, db.App, "a-doula", false)

	for _, path := range founderRoutes {
		resp := getAsSession(t, srv, session, path)
		if resp.status != http.StatusNotFound {
			t.Fatalf("GET %s status = %d, want %d", path, resp.status, http.StatusNotFound)
		}
	}
}

// The founder's own session with no second factor is refused with the
// MFA_REQUIRED code the app already routes into TOTP enrollment.
func TestFounderOnly_RefusesTheFounderWithNoSecondFactor(t *testing.T) {
	db := testdb.New(t)
	founderID := testdb.SeedStaff(t, db, founderUID)
	srv := newFounderServer(t, db, founderID)
	session := authntest.SeedSessionWithSecondFactor(t, db.App, founderUID, false)

	for _, path := range founderRoutes {
		resp := getAsSession(t, srv, session, path)
		if resp.status != http.StatusForbidden {
			t.Fatalf("GET %s status = %d, want %d", path, resp.status, http.StatusForbidden)
		}
		if got := resp.refusal(t).Code; got != apierr.CodeMFARequired {
			t.Errorf("GET %s code = %q, want %q", path, got, apierr.CodeMFARequired)
		}
	}
}

// With FOUNDER_STAFF_ID unset every founder route refuses everybody,
// the person who would otherwise be the founder included.
func TestFounderOnly_RefusesEverybodyWhenNoFounderIsConfigured(t *testing.T) {
	db := testdb.New(t)
	testdb.SeedStaff(t, db, founderUID)
	session := authntest.SeedSession(t, db.App, founderUID)

	for _, configured := range []string{"", "   "} {
		srv := newFounderServer(t, db, configured)
		for _, path := range founderRoutes {
			resp := getAsSession(t, srv, session, path)
			if resp.status != http.StatusNotFound {
				t.Fatalf("FOUNDER_STAFF_ID=%q: GET %s status = %d, want %d", configured, path, resp.status, http.StatusNotFound)
			}
		}
	}
}

// No session at all is authn.Begin's own 401, the same as every other
// session-reading route.
func TestFounderOnly_RefusesACallerWithNoSession(t *testing.T) {
	db := testdb.New(t)
	founderID := testdb.SeedStaff(t, db, founderUID)
	srv := newFounderServer(t, db, founderID)

	for _, path := range founderRoutes {
		resp := getAsSession(t, srv, "", path)
		if resp.status != http.StatusUnauthorized {
			t.Fatalf("GET %s status = %d, want %d", path, resp.status, http.StatusUnauthorized)
		}
	}
}

// The founder, with a second factor, gets in: an empty list, and the
// same 404 as everybody else for a piece that does not exist.
func TestFounderOnly_AdmitsTheFounderWithASecondFactor(t *testing.T) {
	db := testdb.New(t)
	// Upper case on purpose: an id pasted into the env var in the other
	// case still names the same Staff member.
	founderID := testdb.SeedStaff(t, db, founderUID)
	srv := newFounderServer(t, db, " "+strings.ToUpper(founderID)+" ")
	session := authntest.SeedSession(t, db.App, founderUID)

	resp := getAsSession(t, srv, session, "/api/staff/feedback")
	if resp.status != http.StatusOK {
		t.Fatalf("list status = %d, want %d", resp.status, http.StatusOK)
	}

	resp = getAsSession(t, srv, session, "/api/staff/feedback/"+absentFeedbackID)
	if resp.status != http.StatusNotFound {
		t.Fatalf("absent piece status = %d, want %d", resp.status, http.StatusNotFound)
	}
	if got := resp.refusal(t).Message; got != staffauth.MsgFounderRouteNotFound {
		t.Errorf("absent piece message = %q, want %q", got, staffauth.MsgFounderRouteNotFound)
	}
}
