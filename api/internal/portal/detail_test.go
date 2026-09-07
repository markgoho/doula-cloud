package portal_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/portal"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// newServer mounts this package's whole surface through portal.Mount, the
// same call main.go makes on the real GatedRouter.
func newServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	portal.Mount(g, db.App)
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

// seedClientAtPracticeWithDueDate inserts a Practice, a Client with an
// Engagement at it, and a client_portal_users row linking identityUID to
// that Client, using the superuser Admin connection. dueDate is passed
// straight to the insert, so "" leaves the nullable column
// (ADR-0017: "nullable because a postpartum-only Engagement has none")
// unset -- the null-due-date branch #505 asks for. Stays local rather
// than moving to testdb: due_date is a fact only this package's own
// detail-view tests care about, and no other package's fixture needs it.
func seedClientAtPracticeWithDueDate(t *testing.T, db *testdb.DB, identityUID, practiceName, dueDate string) (engagementID, status string) {
	t.Helper()
	return seedClientAtPracticeWithDueDateAndKind(t, db, identityUID, practiceName, dueDate, "birth")
}

// seedClientAtPracticeWithDueDateAndKind is
// seedClientAtPracticeWithDueDate with an explicit kind, for #311's own
// offersBirthPlan assertions -- every other call in this file wants the
// 'birth' default, so that one stays the short form.
func seedClientAtPracticeWithDueDateAndKind(t *testing.T, db *testdb.DB, identityUID, practiceName, dueDate, kind string) (engagementID, status string) {
	t.Helper()

	var practiceID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO practices (name) VALUES ($1) RETURNING id`, practiceName,
	).Scan(&practiceID); err != nil {
		t.Fatalf("seed practice: %v", err)
	}

	var clientID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (practice_id, given_name, email) VALUES ($1, 'Test Client', 'client@example.com') RETURNING id`,
		practiceID,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}

	status = "intake"
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, status, kind, due_date) VALUES ($1, $2, $3, $5, nullif($4, '')::date) RETURNING id`,
		clientID, practiceID, status, dueDate, kind,
	).Scan(&engagementID); err != nil {
		t.Fatalf("seed engagement: %v", err)
	}

	testdb.SeedPortalAccount(t, db, identityUID, identityUID+"@example.com")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO client_portal_users (identity_uid, client_id) VALUES ($1, $2)`,
		identityUID, clientID,
	); err != nil {
		t.Fatalf("seed client_portal_users: %v", err)
	}

	return engagementID, status
}

func TestDetailHandler_Success(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "portal-detail-uid"
	engagementID, status := seedClientAtPracticeWithDueDate(t, db, identityUID, "Riverside Doulas", "2027-06-15")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/api/portal/engagements/"+engagementID, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var out portal.Detail
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.EngagementID != engagementID {
		t.Fatalf("engagementId = %q, want %q", out.EngagementID, engagementID)
	}
	// The portal bar's avatar needs a name of the Client's own (#452), and
	// after ADR-0017 that is a display name rather than a column.
	if out.ClientName != "Test Client" {
		t.Fatalf("clientName = %q, want %q", out.ClientName, "Test Client")
	}
	if out.PracticeName != "Riverside Doulas" {
		t.Fatalf("practiceName = %q, want %q", out.PracticeName, "Riverside Doulas")
	}
	if out.Status != status {
		t.Fatalf("status = %q, want %q", out.Status, status)
	}
	// #505: the portal reads the Engagement's due date.
	if out.DueDate == nil || *out.DueDate != "2027-06-15" {
		t.Fatalf("dueDate = %v, want %q", out.DueDate, "2027-06-15")
	}
	// #311: a birth Engagement's own read still resolves the question
	// true, the same as before this field existed.
	if !out.OffersBirthPlan {
		t.Fatalf("offersBirthPlan = %v, want true for a birth Engagement", out.OffersBirthPlan)
	}
	// #310: when the Engagement began, back on this DTO for the chrome's
	// switcher label.
	if out.CreatedAt.IsZero() {
		t.Fatalf("createdAt = %v, want a non-zero time", out.CreatedAt)
	}
}

// TestDetailHandler_NullDueDate covers #505's "show nothing" branch: a
// postpartum-only Engagement has no due date (ADR-0017), and the field
// must be absent from the JSON entirely -- `omitempty` is what the page
// relies on to distinguish "nothing to show" from a fetch that broke.
func TestDetailHandler_NullDueDate(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "portal-detail-null-due-date-uid"
	engagementID, _ := seedClientAtPracticeWithDueDate(t, db, identityUID, "Postpartum Only Doulas", "")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/api/portal/engagements/"+engagementID, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, present := raw["dueDate"]; present {
		t.Fatalf("dueDate key present in response, want omitted: %v", raw["dueDate"])
	}
}

// TestDetailHandler_PostpartumEngagementDoesNotOfferBirthPlan proves
// #311's own AC: the portal's Engagement read resolves the suppression
// question for a postpartum-only Engagement to false, and the raw kind
// never appears in the response at all -- CONTEXT.md's Engagement entry
// gives kind no Client word, so no key on this DTO may carry it.
func TestDetailHandler_PostpartumEngagementDoesNotOfferBirthPlan(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "portal-detail-postpartum-uid"
	engagementID, _ := seedClientAtPracticeWithDueDateAndKind(t, db, identityUID, "Postpartum Only Doulas", "", "postpartum")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/api/portal/engagements/"+engagementID, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	var out portal.Detail
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.OffersBirthPlan {
		t.Fatalf("offersBirthPlan = %v, want false for a postpartum Engagement", out.OffersBirthPlan)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("decode raw response: %v", err)
	}
	if _, present := raw["kind"]; present {
		t.Fatalf("kind key present in response, want omitted entirely: %v", raw["kind"])
	}
}

func TestDetailHandler_NotLinkedToClient(t *testing.T) {
	db := testdb.New(t)
	_, _ = seedClientAtPracticeWithDueDate(t, db, "other-portal-uid", "Other Practice", "")

	srv, session := newServer(t, db, "unrelated-uid")
	defer srv.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/api/portal/engagements/00000000-0000-0000-0000-000000000000", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}
