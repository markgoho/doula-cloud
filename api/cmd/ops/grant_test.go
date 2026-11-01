package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/testdb"
)

const (
	grantPracticeName   = "Rooted Birth Collective"
	doulaRole           = "doula"
	unknownPracticeID   = "00000000-0000-0000-0000-000000000000"
	malformedPracticeID = "not-a-uuid"
)

// grantStack is opsStack with a Practice of four Staff and no founding
// grant yet.
type grantStack struct {
	*opsStack
	practiceID string
}

func newGrantStack(t *testing.T) *grantStack {
	t.Helper()
	s := &grantStack{opsStack: newOpsStack(t, false)}
	s.practiceID = testdb.SeedPractice(t, s.db, grantPracticeName)
	testdb.SeedStaffAtPractice(t, s.db, s.practiceID, "grant-owner", []string{"owner"}, "employee")
	for _, uid := range []string{"grant-doula-a", "grant-doula-b", "grant-doula-c"} {
		testdb.SeedStaffAtPractice(t, s.db, s.practiceID, uid, []string{doulaRole}, "contractor")
	}
	return s
}

func (s *grantStack) check(cookie string) *httptest.ResponseRecorder {
	s.t.Helper()
	return s.send(http.MethodGet, "/api/practices/"+s.practiceID+"/founding-grant", cookie, nil, nil)
}

func (s *grantStack) grant(cookie, key string, body any) *httptest.ResponseRecorder {
	s.t.Helper()
	header := http.Header{}
	if key != "" {
		header.Set(headerIdempotencyKey, key)
	}
	return s.send(http.MethodPost, "/api/practices/"+s.practiceID+"/founding-grants", cookie, header, body)
}

func TestFoundingGrantCheck_StatesThePracticeAndTheGrantItsRosterSizes(t *testing.T) {
	s := newGrantStack(t)

	w := s.check(s.signedInCookie())

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", w.Code, w.Body)
	}
	want := `{"practiceName":"` + grantPracticeName + `","staffCount":4,"credits":12,"grantedCredits":null}`
	if got := strings.TrimSpace(w.Body.String()); got != want {
		t.Errorf("body = %s\nwant %s", got, want)
	}
}

func TestFoundingGrantCheck_SaysWhatAGrantedPracticeAlreadyHolds(t *testing.T) {
	s := newGrantStack(t)
	testdb.SeedFoundingGrant(t, s.db, s.practiceID, 9, opsStart.AddDate(0, -1, 0))

	w := s.check(s.signedInCookie())

	if !strings.Contains(w.Body.String(), `"grantedCredits":9`) {
		t.Errorf("body = %s, want the 9 Credits already granted", w.Body)
	}
}

func TestFoundingGrantCheck_AnUnknownPracticeIsNotFound(t *testing.T) {
	s := newGrantStack(t)
	cookie := s.signedInCookie()

	for _, id := range []string{malformedPracticeID, unknownPracticeID} {
		if w := s.send(http.MethodGet, "/api/practices/"+id+"/founding-grant", cookie, nil, nil); w.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", id, w.Code)
		}
	}
}

func TestFoundingGrant_GivesTheRosterSizedGrantAndRecordsWhoGaveIt(t *testing.T) {
	s := newGrantStack(t)
	cookie := s.signedInCookie()
	s.now = s.now.Add(time.Minute)

	w := s.grant(cookie, "grant-key-1", map[string]any{})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", w.Code, w.Body)
	}
	want := `{"practiceName":"` + grantPracticeName + `","staffCount":4,"credits":12}`
	if got := strings.TrimSpace(w.Body.String()); got != want {
		t.Errorf("body = %s\nwant %s", got, want)
	}

	var actID, subjectKind, subjectID string
	var details []byte
	var stepUpAt *time.Time
	var createdAt time.Time
	if err := s.db.Admin.QueryRowContext(t.Context(),
		`SELECT a.id, a.subject_kind, a.subject_id, a.details, a.step_up_at, a.created_at
		   FROM operator_acts a JOIN operators o ON o.id = a.operator_id
		  WHERE o.identity_uid = $1 AND a.action = 'founding_grant'`, opUID,
	).Scan(&actID, &subjectKind, &subjectID, &details, &stepUpAt, &createdAt); err != nil {
		t.Fatalf("read the founding grant act: %v", err)
	}
	if subjectKind != "practice" || subjectID != s.practiceID || stepUpAt != nil || !createdAt.Equal(s.now) {
		t.Errorf("act = (%s/%s, step_up_at %v, created_at %s), want practice/%s with no step-up, taken at %s",
			subjectKind, subjectID, stepUpAt, createdAt, s.practiceID, s.now)
	}
	if string(details) != `{"credits": 12, "requestKey": "grant-key-1", "staffCount": 4}` {
		t.Errorf("act details = %s", details)
	}

	if n := s.count(`SELECT count(*) FROM credit_ledger
		WHERE origin = 'founding_grant' AND practice_id = $1 AND quantity = 12 AND operator_act_id = $2 AND created_at = $3`,
		s.practiceID, actID, s.now); n != 1 {
		t.Errorf("founding grant rows keyed to the act = %d, want 1", n)
	}

	var diff []byte
	if err := s.db.Admin.QueryRowContext(t.Context(),
		`SELECT diff FROM activity
		  WHERE practice_id = $1 AND subject_kind = 'practice' AND subject_id = $1 AND action = 'founding_grant_issued'
		    AND actor_kind = 'operator' AND actor_operator_act_id = $2 AND created_at = $3`,
		s.practiceID, actID, s.now).Scan(&diff); err != nil {
		t.Fatalf("read the Activity row: %v", err)
	}
	if string(diff) != `{"credits": 12, "staffCount": 4}` {
		t.Errorf("activity diff = %s", diff)
	}
}

func TestFoundingGrant_ARetryUnderTheSameKeyIsTheSameGrant(t *testing.T) {
	s := newGrantStack(t)
	cookie := s.signedInCookie()
	first := s.grant(cookie, "grant-key-retry", map[string]any{})
	testdb.SeedStaffAtPractice(t, s.db, s.practiceID, "grant-late-joiner", []string{doulaRole}, "employee")

	w := s.grant(cookie, "grant-key-retry", map[string]any{})

	if w.Code != http.StatusOK || w.Body.String() != first.Body.String() {
		t.Fatalf("retry = %d %s, want 200 and the first answer %s", w.Code, w.Body, first.Body)
	}
	if n := s.count(`SELECT count(*) FROM operator_acts WHERE action = 'founding_grant'`); n != 1 {
		t.Errorf("founding grant acts = %d, want 1", n)
	}
	if n := s.count(`SELECT count(*) FROM activity WHERE action = 'founding_grant_issued'`); n != 1 {
		t.Errorf("founding grant Activity rows = %d, want 1", n)
	}
}

func TestFoundingGrant_RefusesASecondGrant(t *testing.T) {
	s := newGrantStack(t)
	cookie := s.signedInCookie()
	if w := s.grant(cookie, "grant-key-first", map[string]any{}); w.Code != http.StatusOK {
		t.Fatalf("first grant = %d %s", w.Code, w.Body)
	}

	w := s.grant(cookie, "grant-key-second", map[string]any{})

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body %s", w.Code, w.Body)
	}
	if got := decodeError(t, w).Message; got != "This Practice already holds its founding grant of 12 Credits. A founding grant is given once." {
		t.Errorf("message = %q", got)
	}
	if n := s.count(`SELECT count(*) FROM credit_ledger WHERE origin = 'founding_grant'`); n != 1 {
		t.Errorf("founding grant rows = %d, want 1", n)
	}
}

func TestFoundingGrant_RefusesAGrantGivenBeforeOps(t *testing.T) {
	s := newGrantStack(t)
	testdb.SeedFoundingGrant(t, s.db, s.practiceID, 9, opsStart.AddDate(0, -1, 0))

	w := s.grant(s.signedInCookie(), "grant-key-after", map[string]any{})

	if w.Code != http.StatusConflict || !strings.Contains(decodeError(t, w).Message, "founding grant of 9 Credits") {
		t.Fatalf("status = %d, body %s, want 409 naming the 9 Credits held", w.Code, w.Body)
	}
}

func TestFoundingGrant_RefusesAPracticeWithNoStaff(t *testing.T) {
	s := newGrantStack(t)
	empty := testdb.SeedPractice(t, s.db, "Nobody Here")

	w := s.send(http.MethodPost, "/api/practices/"+empty+"/founding-grants", s.signedInCookie(),
		http.Header{headerIdempotencyKey: {"grant-key-empty"}}, map[string]any{})

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body %s", w.Code, w.Body)
	}
	if got := decodeError(t, w).Message; got != "This Practice has no Staff, so there is nothing to size a founding grant from." {
		t.Errorf("message = %q", got)
	}
	if n := s.count(`SELECT count(*) FROM operator_acts WHERE action = 'founding_grant'`); n != 0 {
		t.Errorf("founding grant acts = %d, want none", n)
	}
}

func TestFoundingGrant_Refusals(t *testing.T) {
	s := newGrantStack(t)
	cookie := s.signedInCookie()

	cases := []struct {
		name, cookie, key string
		body              any
		status            int
		message           string
	}{
		{"no session", "", "k", map[string]any{}, http.StatusUnauthorized, ""},
		{"no Idempotency-Key", cookie, "", map[string]any{}, http.StatusBadRequest, "Send an Idempotency-Key header with the founding grant."},
		{"an actor in the body", cookie, "k", map[string]any{"grantedBy": "someone"}, http.StatusBadRequest, msgNoGrantFields},
		{"an amount in the body", cookie, "k", map[string]any{fieldCredits: 100}, http.StatusBadRequest, msgNoGrantFields},
		{"not JSON", cookie, "k", "not json", http.StatusBadRequest, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := s.grant(tc.cookie, tc.key, tc.body)
			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d; body %s", w.Code, tc.status, w.Body)
			}
			if tc.message != "" {
				if got := decodeError(t, w).Message; got != tc.message {
					t.Errorf("message = %q, want %q", got, tc.message)
				}
			}
		})
	}
	if n := s.count(`SELECT count(*) FROM credit_ledger WHERE origin = 'founding_grant'`); n != 0 {
		t.Errorf("founding grant rows after every refusal = %d, want none", n)
	}
}

func TestFoundingGrant_AnUnknownPracticeIsNotFound(t *testing.T) {
	s := newGrantStack(t)
	cookie := s.signedInCookie()

	for _, id := range []string{malformedPracticeID, unknownPracticeID} {
		w := s.send(http.MethodPost, "/api/practices/"+id+"/founding-grants", cookie, http.Header{headerIdempotencyKey: {"k"}}, map[string]any{})
		if w.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", id, w.Code)
		}
	}
}

func TestFoundingGrantCheck_RefusesNoSession(t *testing.T) {
	s := newGrantStack(t)

	if w := s.check(""); w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestFoundingGrant_ResultDecodes(t *testing.T) {
	s := newGrantStack(t)

	var got foundingGrantResult
	if err := json.Unmarshal(s.grant(s.signedInCookie(), "grant-key-decode", map[string]any{}).Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got != (foundingGrantResult{PracticeName: grantPracticeName, StaffCount: 4, Credits: 12}) {
		t.Errorf("result = %+v", got)
	}
}
