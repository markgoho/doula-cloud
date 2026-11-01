package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/testdb"
)

const (
	listPracticeName = "Rooted Birth Collective"
	ownerRole        = "owner"
)

var (
	listSignedUp  = time.Date(2026, 9, 14, 15, 0, 0, 0, time.UTC)
	listGrantedAt = time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
)

type listStack struct {
	*opsStack
	practiceID string
}

// newListStack seeds a Practice with two Owners and a Doula, a Connect
// account, a founding grant, a purchase, two Engagements started and two
// pieces of Feedback, one of them from a Client.
func newListStack(t *testing.T) *listStack {
	t.Helper()
	s := &listStack{opsStack: newOpsStack(t, false)}
	s.practiceID = testdb.SeedPractice(t, s.db, listPracticeName)
	s.exec(`UPDATE practices SET created_at = $2,
	            stripe_connect_account_id = 'acct_list', stripe_connect_card_payments_status = 'active',
	            stripe_connect_payouts_status = 'pending',
	            stripe_connect_requirements_due = '{external_account}',
	            stripe_connect_status_updated_at = $3
	         WHERE id = $1`, s.practiceID, listSignedUp, listGrantedAt)
	owner := testdb.SeedNamedStaffAtPractice(t, s.db, s.practiceID, "list-owner", "Renata Alvarez", []string{ownerRole, doulaRole}, "employee")
	testdb.SeedNamedStaffAtPractice(t, s.db, s.practiceID, "list-co-owner", "Ada Bell", []string{ownerRole}, "employee")
	testdb.SeedContractorAtPractice(t, s.db, s.practiceID, "list-doula")
	actID := testdb.SeedOperatorAct(t, s.db, "Mark Goho")
	s.exec(`INSERT INTO credit_ledger (practice_id, origin, quantity, operator_act_id, created_at)
	        VALUES ($1, 'founding_grant', 9, $2, $3)`, s.practiceID, actID, listGrantedAt)
	s.exec(`INSERT INTO credit_ledger (practice_id, origin, quantity, created_at)
	        VALUES ($1, 'signup_bonus', 5, $2)`, s.practiceID, listSignedUp)
	for _, name := range []string{"Hannah Sorensen", "Camille Boyd"} {
		_, engagementID := testdb.SeedNamedEngagement(t, s.db, s.practiceID, name, strings.ToLower(strings.Fields(name)[0])+"@example.com")
		s.exec(`INSERT INTO credit_ledger (practice_id, origin, quantity, consumed_engagement_id, consumed_at, drawn_lot_id, created_at)
		        SELECT $1, 'consumption', -1, $2, $3, id, $3 FROM credit_ledger WHERE practice_id = $1 AND origin = 'signup_bonus'`,
			s.practiceID, engagementID, listGrantedAt)
	}
	clientID := testdb.SeedNamedClient(t, s.db, s.practiceID, "Nadia", "nadia@example.com")
	testdb.SeedPortalAccount(t, s.db, "portal-nadia", "nadia@example.com")
	s.exec(`INSERT INTO feedback (id, kind, text, page_url, route_id, app_build, screen_width, browser, staff_id, practice_id, roles, sent_at)
	        VALUES (gen_random_uuid(), 'not_working', 'typed', '/x', '/x', 'abc1234', 390, 'Safari 18', $1, $2, '{owner}', $3)`,
		owner, s.practiceID, listGrantedAt)
	s.exec(`INSERT INTO feedback (id, kind, text, page_url, route_id, app_build, screen_width, browser, portal_account, practice_id, client_id, sent_at)
	        VALUES (gen_random_uuid(), 'not_working', 'typed', '/x', '/x', 'abc1234', 390, 'Safari 18', 'portal-nadia', $1, $2, $3)`,
		s.practiceID, clientID, listGrantedAt)
	return s
}

func (s *opsStack) exec(query string, args ...any) {
	s.t.Helper()
	if _, err := s.db.Admin.ExecContext(s.t.Context(), query, args...); err != nil {
		s.t.Fatalf("exec %q: %v", query, err)
	}
}

func (s *opsStack) list(path, cookie string) *httptest.ResponseRecorder {
	s.t.Helper()
	return s.send(http.MethodGet, path, cookie, nil, nil)
}

type listPage struct {
	Items      []map[string]json.RawMessage `json:"items"`
	NextCursor *string                      `json:"nextCursor"`
	HasMore    bool                         `json:"hasMore"`
}

func decodeList(t *testing.T, w *httptest.ResponseRecorder) listPage {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", w.Code, w.Body)
	}
	var page listPage
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode %s: %v", w.Body, err)
	}
	return page
}

func compactJSON(t *testing.T, raw string) string {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	out, _ := json.Marshal(v)
	return string(out)
}

func TestPracticeList_ShowsEachPracticesFields(t *testing.T) {
	s := newListStack(t)

	page := decodeList(t, s.list("/api/practices", s.signedInCookie()))

	if len(page.Items) != 1 || page.HasMore || page.NextCursor != nil {
		t.Fatalf("page = %+v, want one Practice and no more", page)
	}
	got, _ := json.Marshal(page.Items[0])
	want := compactJSON(t, `{
		"id": "`+s.practiceID+`",
		"name": "Rooted Birth Collective",
		"signedUpAt": "2026-09-14T15:00:00Z",
		"owners": [
			{"name": "Renata Alvarez", "email": "list-owner@example.com"},
			{"name": "Ada Bell", "email": "list-co-owner@example.com"}
		],
		"staffCount": 3,
		"stripeConnect": {
			"connected": true,
			"cardPayments": "active",
			"payouts": "pending",
			"requirementsDue": ["external_account"],
			"updatedAt": "2026-09-15T10:00:00Z"
		},
		"creditsHeld": 12,
		"engagementsStarted": 2,
		"foundingGrant": {"credits": 9, "givenBy": "Mark Goho", "givenAt": "2026-09-15T10:00:00Z"},
		"deletion": null,
		"source": null,
		"openFeedback": 2
	}`)
	if compactJSON(t, string(got)) != want {
		t.Errorf("item =\n%s\nwant\n%s", got, want)
	}
}

func TestPracticeList_CarriesNoClientPerStaffSignInBillingModeOrSubscriptionField(t *testing.T) {
	s := newListStack(t)
	cookie := s.signedInCookie()
	s.exec(`UPDATE practices SET deleted_at = $2 WHERE id = $1`, testdb.SeedPractice(t, s.db, "Gone"), listGrantedAt)

	for _, path := range []string{"/api/practices", "/api/practices/deleted"} {
		page := decodeList(t, s.list(path, cookie))
		if len(page.Items) != 1 {
			t.Fatalf("%s: %d items, want 1", path, len(page.Items))
		}
		keys := []string{}
		for k := range page.Items[0] {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		want := []string{"creditsHeld", "deletion", "engagementsStarted", "foundingGrant", "id", "name", "openFeedback", "owners", "signedUpAt", "source", "staffCount", "stripeConnect"}
		if !slices.Equal(keys, want) {
			t.Errorf("%s fields = %v\nwant %v", path, keys, want)
		}
		raw, _ := json.Marshal(page.Items[0])
		for _, banned := range []string{"client", "lastactive", "lastsignin", "billingmode", "subscription"} {
			if strings.Contains(strings.ToLower(string(raw)), banned) {
				t.Errorf("%s carries %q: %s", path, banned, raw)
			}
		}
	}
}

func TestPracticeList_AFinalizedDeletionMovesToTheDeletedList(t *testing.T) {
	s := newListStack(t)
	cookie := s.signedInCookie()
	pending := testdb.SeedPractice(t, s.db, "Pending Deletion")
	s.exec(`UPDATE practices SET created_at = $2, deletion_requested_at = $3, deletion_finalize_at = $4 WHERE id = $1`,
		pending, listSignedUp.Add(time.Hour), listGrantedAt, listGrantedAt.AddDate(0, 0, 30))
	gone := testdb.SeedPractice(t, s.db, "Gone")
	s.exec(`UPDATE practices SET deletion_requested_at = $2, deletion_finalize_at = $3, deleted_at = $3 WHERE id = $1`,
		gone, listSignedUp, listGrantedAt)

	live := decodeList(t, s.list("/api/practices", cookie))
	names := []string{}
	for _, item := range live.Items {
		var name string
		_ = json.Unmarshal(item["name"], &name)
		names = append(names, name)
	}
	if !slices.Equal(names, []string{"Pending Deletion", listPracticeName}) {
		t.Errorf("live list = %v, want the pending Practice, newest first, and no deleted one", names)
	}
	if got := string(live.Items[0]["deletion"]); compactJSON(t, got) != compactJSON(t, `{"requestedAt":"2026-09-15T10:00:00Z","finalizesAt":"2026-10-15T10:00:00Z","deletedAt":null}`) {
		t.Errorf("pending deletion = %s", got)
	}

	deleted := decodeList(t, s.list("/api/practices/deleted", cookie))
	if len(deleted.Items) != 1 || string(deleted.Items[0]["id"]) != `"`+gone+`"` {
		t.Fatalf("deleted list = %+v, want only %s", deleted.Items, gone)
	}
	if got := string(deleted.Items[0]["deletion"]); compactJSON(t, got) != compactJSON(t, `{"requestedAt":"2026-09-14T15:00:00Z","finalizesAt":"2026-09-15T10:00:00Z","deletedAt":"2026-09-15T10:00:00Z"}`) {
		t.Errorf("finalized deletion = %s", got)
	}
}

func TestPracticeList_APracticeWithNothingYetShowsEmptyFields(t *testing.T) {
	s := &listStack{opsStack: newOpsStack(t, false)}
	testdb.SeedPractice(t, s.db, "Brand New")

	page := decodeList(t, s.list("/api/practices", s.signedInCookie()))

	item := page.Items[0]
	for field, want := range map[string]string{
		"owners":             `[]`,
		"staffCount":         `0`,
		"creditsHeld":        `0`,
		"engagementsStarted": `0`,
		"foundingGrant":      `null`,
		"openFeedback":       `0`,
		"stripeConnect":      `{"connected":false,"cardPayments":"unsupported","payouts":"unsupported","requirementsDue":[],"updatedAt":null}`,
	} {
		if got := string(item[field]); got != want {
			t.Errorf("%s = %s, want %s", field, got, want)
		}
	}
}

func TestPracticeList_AGrantGivenBeforeOpsNamesNobody(t *testing.T) {
	s := &listStack{opsStack: newOpsStack(t, false)}
	id := testdb.SeedPractice(t, s.db, "Early")
	s.exec(`ALTER TABLE credit_ledger DROP CONSTRAINT credit_ledger_founding_grant_operator_act`)
	s.exec(`INSERT INTO credit_ledger (practice_id, origin, quantity, created_at) VALUES ($1, 'founding_grant', 6, $2)`, id, listGrantedAt)

	page := decodeList(t, s.list("/api/practices", s.signedInCookie()))

	if got := string(page.Items[0]["foundingGrant"]); got != `{"credits":6,"givenBy":null,"givenAt":"2026-09-15T10:00:00Z"}` {
		t.Errorf("foundingGrant = %s", got)
	}
}

func TestPracticeList_OpeningAPageRecordsWhoReadWhichOwnersContact(t *testing.T) {
	s := newListStack(t)
	cookie := s.signedInCookie()
	s.now = s.now.Add(time.Minute)

	decodeList(t, s.list("/api/practices", cookie))

	var subjectKind *string
	var details []byte
	var stepUpAt *time.Time
	if err := s.db.Admin.QueryRowContext(t.Context(),
		`SELECT a.subject_kind, a.details, a.step_up_at FROM operator_acts a JOIN operators o ON o.id = a.operator_id
		  WHERE o.identity_uid = $1 AND a.action = 'owner_contact_read' AND a.created_at = $2`, opUID, s.now,
	).Scan(&subjectKind, &details, &stepUpAt); err != nil {
		t.Fatalf("read the read row: %v", err)
	}
	if subjectKind != nil || stepUpAt != nil {
		t.Errorf("read row subject %v, step_up_at %v, want neither", subjectKind, stepUpAt)
	}
	if want := `{"list": "practices", "practiceIds": ["` + s.practiceID + `"]}`; string(details) != want {
		t.Errorf("details = %s, want %s", details, want)
	}
}

func TestPracticeList_TheDeletedListRecordsItsReadToo(t *testing.T) {
	s := &listStack{opsStack: newOpsStack(t, false)}
	cookie := s.signedInCookie()

	decodeList(t, s.list("/api/practices/deleted", cookie))

	if n := s.count(`SELECT count(*) FROM operator_acts WHERE action = 'owner_contact_read' AND details = '{"list": "deleted", "practiceIds": []}'`); n != 1 {
		t.Errorf("deleted-list read rows = %d, want 1", n)
	}
}

func TestPracticeList_PagesNewestFirstByCursor(t *testing.T) {
	s := &listStack{opsStack: newOpsStack(t, false)}
	cookie := s.signedInCookie()
	for i := range practicePageSize + 1 {
		id := testdb.SeedPractice(t, s.db, "Practice")
		s.exec(`UPDATE practices SET created_at = $2 WHERE id = $1`, id, listSignedUp.Add(time.Duration(i)*time.Hour))
	}

	first := decodeList(t, s.list("/api/practices", cookie))
	if len(first.Items) != practicePageSize || !first.HasMore || first.NextCursor == nil {
		t.Fatalf("first page: %d items, hasMore %v, cursor %v", len(first.Items), first.HasMore, first.NextCursor)
	}
	second := decodeList(t, s.list("/api/practices?cursor="+*first.NextCursor, cookie))
	if len(second.Items) != 1 || second.HasMore {
		t.Fatalf("second page: %d items, hasMore %v", len(second.Items), second.HasMore)
	}
	if got := string(second.Items[0]["signedUpAt"]); got != `"2026-09-14T15:00:00Z"` {
		t.Errorf("last Practice signed up %s, want the oldest", got)
	}
}

func TestPracticeList_RefusesAMalformedCursor(t *testing.T) {
	s := &listStack{opsStack: newOpsStack(t, false)}

	w := s.list("/api/practices?cursor=not-a-cursor", s.signedInCookie())

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body %s", w.Code, w.Body)
	}
	if n := s.count(`SELECT count(*) FROM operator_acts WHERE action = 'owner_contact_read'`); n != 0 {
		t.Errorf("read rows = %d, want none for a refused request", n)
	}
}

func TestPracticeList_RefusedWithoutASession(t *testing.T) {
	s := &listStack{opsStack: newOpsStack(t, false)}

	for _, path := range []string{"/api/practices", "/api/practices/deleted"} {
		if w := s.list(path, ""); w.Code != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want 401", path, w.Code)
		}
	}
	if n := s.count(`SELECT count(*) FROM operator_acts WHERE action = 'owner_contact_read'`); n != 0 {
		t.Errorf("read rows = %d, want none", n)
	}
}
