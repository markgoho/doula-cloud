package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/testdb"
)

const (
	clearStaffName   = "Priya Raman"
	clearStaffUID    = "clear-staff-uid"
	clearPracticeOne = "Lakeside Birth Collective"
	clearPracticeTwo = "Rooted Doula Care"
)

// secondFactorStack is opsStack with a Staff member who belongs to two
// Practices, holds an authenticator app and is signed in to doula-api.
type secondFactorStack struct {
	*opsStack
	accounts  *authntest.FakeAccountManager
	staffID   string
	practiceA string
	practiceB string
}

func newSecondFactorStack(t *testing.T) *secondFactorStack {
	t.Helper()
	s := &secondFactorStack{opsStack: &opsStack{t: t, db: testdb.New(t), now: opsStart}, accounts: authntest.NewFakeAccountManager()}
	s.handler, _ = routes(Deps{
		DB:       s.db.Ops,
		Now:      func() time.Time { return s.now },
		Verifier: tokens,
		Origins:  []string{testOrigin},
		Accounts: s.accounts,
	})
	s.practiceA = testdb.SeedPractice(t, s.db, clearPracticeOne)
	s.practiceB = testdb.SeedPractice(t, s.db, clearPracticeTwo)
	s.staffID = testdb.SeedNamedStaffAtPractice(t, s.db, s.practiceA, clearStaffUID, clearStaffName, []string{doulaRole}, "employee")
	if _, err := s.db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_memberships (practice_id, staff_id, roles, employment_type) VALUES ($1, $2, '{doula}', 'contractor')`,
		s.practiceB, s.staffID); err != nil {
		t.Fatalf("seed second membership: %v", err)
	}
	if _, err := s.db.Admin.ExecContext(t.Context(),
		`INSERT INTO sessions (token_hash, identity_uid, expires_at) VALUES ('clear-staff-session', $1, $2)`,
		clearStaffUID, opsStart.Add(time.Hour)); err != nil {
		t.Fatalf("seed staff session: %v", err)
	}
	s.accounts.Seed(clearStaffUID, clearStaffUID+"@example.com", true)
	s.accounts.EnrollTOTP(clearStaffUID)
	return s
}

func (s *secondFactorStack) check(cookie, staffID string) *httptest.ResponseRecorder {
	s.t.Helper()
	return s.send(http.MethodGet, "/api/staff/"+staffID+"/second-factor", cookie, nil, nil)
}

func (s *secondFactorStack) clear(cookie, key string, body any) *httptest.ResponseRecorder {
	s.t.Helper()
	return s.clearStaff(cookie, s.staffID, key, body)
}

func (s *secondFactorStack) clearStaff(cookie, staffID, key string, body any) *httptest.ResponseRecorder {
	s.t.Helper()
	header := http.Header{}
	if key != "" {
		header.Set(headerIdempotencyKey, key)
	}
	return s.send(http.MethodPost, "/api/staff/"+staffID+"/second-factor-clears", cookie, header, body)
}

func (s *secondFactorStack) acts() int {
	s.t.Helper()
	return s.count(`SELECT count(*) FROM operator_acts WHERE action = 'second_factor_cleared'`)
}

var noFields = map[string]any{}

const wantSecondFactorBody = `{"staffName":"` + clearStaffName + `","practiceNames":["` + clearPracticeOne + `","` + clearPracticeTwo + `"]}`

func TestSecondFactorCheck_StatesTheStaffMemberAndEachPractice(t *testing.T) {
	s := newSecondFactorStack(t)

	w := s.check(s.signedInCookie(), s.staffID)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", w.Code, w.Body)
	}
	if got := strings.TrimSpace(w.Body.String()); got != wantSecondFactorBody {
		t.Errorf("body = %s\nwant %s", got, wantSecondFactorBody)
	}
}

func TestSecondFactorCheck_AStaffMemberAtNoPracticeListsNone(t *testing.T) {
	s := newSecondFactorStack(t)
	loneID := testdb.SeedStaff(t, s.db, "clear-lone")

	w := s.check(s.signedInCookie(), loneID)

	if !strings.Contains(w.Body.String(), `"practiceNames":[]`) {
		t.Errorf("body = %s, want no Practices", w.Body)
	}
}

func TestSecondFactorCheck_AnUnknownStaffMemberIsNotFound(t *testing.T) {
	s := newSecondFactorStack(t)
	cookie := s.signedInCookie()

	for _, id := range []string{unknownPracticeID, malformedPracticeID} {
		if w := s.check(cookie, id); w.Code != http.StatusNotFound || decodeError(t, w).Message != msgStaffNotFound {
			t.Errorf("staff %q: %d %s, want 404 saying so", id, w.Code, w.Body)
		}
	}
}

func TestSecondFactorCheck_RefusesNoSession(t *testing.T) {
	s := newSecondFactorStack(t)

	if w := s.check("", s.staffID); w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestSecondFactorClear_ClearsTheFactorAndRecordsWhoDidIt(t *testing.T) {
	s := newSecondFactorStack(t)
	cookie := s.steppedUpCookie()
	steppedUp := s.now
	s.now = s.now.Add(time.Minute)

	w := s.clear(cookie, "clear-key-1", noFields)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", w.Code, w.Body)
	}
	if got := strings.TrimSpace(w.Body.String()); got != wantSecondFactorBody {
		t.Errorf("body = %s\nwant %s", got, wantSecondFactorBody)
	}
	if s.accounts.HasSecondFactor(clearStaffUID) {
		t.Error("the Staff member still holds a second factor")
	}

	var actID, subjectKind, subjectID string
	var stepUpAt, createdAt time.Time
	if err := s.db.Admin.QueryRowContext(t.Context(),
		`SELECT a.id, a.subject_kind, a.subject_id, a.step_up_at, a.created_at
		   FROM operator_acts a JOIN operators o ON o.id = a.operator_id
		  WHERE o.identity_uid = $1 AND a.action = 'second_factor_cleared'`, opUID,
	).Scan(&actID, &subjectKind, &subjectID, &stepUpAt, &createdAt); err != nil {
		t.Fatalf("read the act: %v", err)
	}
	if subjectKind != "staff" || subjectID != s.staffID || !stepUpAt.Equal(steppedUp) || !createdAt.Equal(s.now) {
		t.Errorf("act = (%s/%s, step_up_at %s, created_at %s), want staff/%s stepped up at %s, taken at %s",
			subjectKind, subjectID, stepUpAt, createdAt, s.staffID, steppedUp, s.now)
	}

	if n := s.count(`SELECT count(*) FROM staff_auth_events
		WHERE staff_id = $1 AND reason = 'support' AND actor_operator_act_id = $2 AND actor_staff_id IS NULL AND created_at = $3`,
		s.staffID, actID, s.now); n != 1 {
		t.Errorf("support rows keyed to the act = %d, want 1", n)
	}

	for _, practiceID := range []string{s.practiceA, s.practiceB} {
		if n := s.count(`SELECT count(*) FROM activity
			WHERE practice_id = $1 AND subject_kind = 'membership' AND subject_id = $2 AND action = 'second_factor_cleared'
			  AND actor_kind = 'operator' AND actor_operator_act_id = $3 AND created_at = $4`,
			practiceID, s.staffID, actID, s.now); n != 1 {
			t.Errorf("Activity rows at %s = %d, want 1", practiceID, n)
		}
	}

	if n := s.count(`SELECT count(*) FROM sessions WHERE identity_uid = $1`, clearStaffUID); n != 0 {
		t.Errorf("the Staff member's sessions = %d, want every one ended", n)
	}
	if n := s.count(`SELECT count(*) FROM session_notice_outbox
		WHERE identity_uid = $1 AND kind IN ('session_revoked', 'mfa_recovery_cleared')`, clearStaffUID); n != 2 {
		t.Errorf("notices queued = %d, want both", n)
	}
	if at := s.stepUpAt(cookie); at != nil {
		t.Errorf("step_up_at = %v after the act, want it spent", at)
	}
}

func TestSecondFactorClear_ARetryUnderTheSameKeyIsTheSameClear(t *testing.T) {
	s := newSecondFactorStack(t)
	cookie := s.steppedUpCookie()
	first := s.clear(cookie, "clear-key-retry", noFields)
	if first.Code != http.StatusOK {
		t.Fatalf("first = %d; body %s", first.Code, first.Body)
	}
	if w := s.stepUp(cookie, "fresh-totp-token"); w.Code != http.StatusNoContent {
		t.Fatalf("step up again = %d", w.Code)
	}
	s.accounts.EnrollTOTP(clearStaffUID)

	second := s.clear(cookie, "clear-key-retry", noFields)

	if second.Code != http.StatusOK || second.Body.String() != first.Body.String() {
		t.Fatalf("retry = %d %s, want the first answer %s", second.Code, second.Body, first.Body)
	}
	if !s.accounts.HasSecondFactor(clearStaffUID) {
		t.Error("the retry cleared the factor enrolled since the first clear")
	}
	if n := s.acts(); n != 1 {
		t.Errorf("acts = %d, want 1", n)
	}
	if n := s.count(`SELECT count(*) FROM activity WHERE action = 'second_factor_cleared'`); n != 2 {
		t.Errorf("Activity rows = %d, want the first clear's 2", n)
	}
}

func TestSecondFactorClear_ANewKeyClearsAgain(t *testing.T) {
	s := newSecondFactorStack(t)
	cookie := s.steppedUpCookie()
	if w := s.clear(cookie, "clear-key-a", noFields); w.Code != http.StatusOK {
		t.Fatalf("first = %d; body %s", w.Code, w.Body)
	}
	if w := s.stepUp(cookie, "fresh-totp-token"); w.Code != http.StatusNoContent {
		t.Fatalf("step up again = %d", w.Code)
	}

	if w := s.clear(cookie, "clear-key-b", noFields); w.Code != http.StatusOK {
		t.Fatalf("second = %d; body %s", w.Code, w.Body)
	}
	if n := s.acts(); n != 2 {
		t.Errorf("acts = %d, want 2", n)
	}
}

func TestSecondFactorClear_RefusesNoSession(t *testing.T) {
	s := newSecondFactorStack(t)

	if w := s.clear("", "clear-key", noFields); w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
	if !s.accounts.HasSecondFactor(clearStaffUID) {
		t.Error("the factor was cleared without a session")
	}
}

func TestSecondFactorClear_RefusesASessionThatHasNotSteppedUp(t *testing.T) {
	s := newSecondFactorStack(t)

	w := s.clear(s.signedInCookie(), "clear-key", noFields)

	if w.Code != http.StatusForbidden || decodeError(t, w).Code != apierr.CodeSecondFactorRequired {
		t.Fatalf("status = %d, body %s, want 403 SECOND_FACTOR_REQUIRED", w.Code, w.Body)
	}
}

func TestSecondFactorClear_RefusesAStaleStepUp(t *testing.T) {
	s := newSecondFactorStack(t)
	cookie := s.steppedUpCookie()
	s.now = s.now.Add(stepUpLimit)

	w := s.clear(cookie, "clear-key", noFields)

	if w.Code != http.StatusForbidden || decodeError(t, w).Code != apierr.CodeSecondFactorRequired {
		t.Fatalf("status = %d, body %s, want 403 SECOND_FACTOR_REQUIRED", w.Code, w.Body)
	}
	if !s.accounts.HasSecondFactor(clearStaffUID) || s.acts() != 0 {
		t.Error("a stale step-up cleared the factor or recorded an act")
	}
}

func TestSecondFactorClear_OneStepUpAdmitsOneClear(t *testing.T) {
	s := newSecondFactorStack(t)
	cookie := s.steppedUpCookie()
	if w := s.clear(cookie, "clear-key-a", noFields); w.Code != http.StatusOK {
		t.Fatalf("first = %d; body %s", w.Code, w.Body)
	}

	if w := s.clear(cookie, "clear-key-b", noFields); w.Code != http.StatusForbidden {
		t.Fatalf("second clear on one step-up = %d, want 403", w.Code)
	}
}

func TestSecondFactorClear_RefusesARequestWithNoKey(t *testing.T) {
	s := newSecondFactorStack(t)

	w := s.clear(s.steppedUpCookie(), "", noFields)

	if w.Code != http.StatusBadRequest || decodeError(t, w).Message != msgClearKeyRequired {
		t.Fatalf("status = %d, body %s, want 400 asking for the key", w.Code, w.Body)
	}
}

func TestSecondFactorClear_TakesNoActorFromTheRequest(t *testing.T) {
	s := newSecondFactorStack(t)

	w := s.clear(s.steppedUpCookie(), "clear-key", map[string]string{"operator": "Someone Else"})

	if w.Code != http.StatusBadRequest || decodeError(t, w).Message != msgNoClearFields {
		t.Fatalf("status = %d, body %s, want 400 refusing the field", w.Code, w.Body)
	}
	if !s.accounts.HasSecondFactor(clearStaffUID) || s.acts() != 0 {
		t.Error("a request naming its actor cleared the factor or recorded an act")
	}
}

func TestSecondFactorClear_AnUnknownStaffMemberIsNotFound(t *testing.T) {
	s := newSecondFactorStack(t)

	for _, id := range []string{unknownPracticeID, malformedPracticeID} {
		w := s.clearStaff(s.steppedUpCookie(), id, "clear-key-"+id, noFields)
		if w.Code != http.StatusNotFound || decodeError(t, w).Message != msgStaffNotFound {
			t.Errorf("staff %q: %d %s, want 404 saying so", id, w.Code, w.Body)
		}
	}
	if s.acts() != 0 {
		t.Error("an unknown Staff member recorded an act")
	}
}

func TestSecondFactorClear_AnIdentityPlatformRefusalChangesNothing(t *testing.T) {
	s := newSecondFactorStack(t)
	s.accounts.ClearSecondFactorsErr = errors.New("identity platform unreachable")

	w := s.clear(s.steppedUpCookie(), "clear-key", noFields)

	if w.Code != http.StatusBadGateway || decodeError(t, w).Message != msgIdentityRefused {
		t.Fatalf("status = %d, body %s, want 502 saying nothing changed", w.Code, w.Body)
	}
	if s.acts() != 0 || s.count(`SELECT count(*) FROM sessions WHERE identity_uid = $1`, clearStaffUID) != 1 {
		t.Error("a refused clear recorded an act or ended the Staff member's sessions")
	}
}
