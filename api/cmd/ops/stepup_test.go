package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
)

const stepUpPath = "/api/session/step-up"

func (s *opsStack) stepUp(cookie, token string) *httptest.ResponseRecorder {
	s.t.Helper()
	header := withCookie(cookie)
	if token != "" {
		header.Set("Authorization", "Bearer "+token)
	}
	r := httptest.NewRequestWithContext(s.t.Context(), http.MethodPost, stepUpPath, nil)
	r.Header = header
	w := httptest.NewRecorder()
	s.handler.ServeHTTP(w, r)
	return w
}

func (s *opsStack) stepUpAt(cookie string) *time.Time {
	s.t.Helper()
	var at *time.Time
	if err := s.db.Admin.QueryRowContext(s.t.Context(),
		`SELECT step_up_at FROM operator_sessions WHERE token_hash = $1`, tokenHash(cookie)).Scan(&at); err != nil {
		s.t.Fatalf("read step_up_at: %v", err)
	}
	return at
}

// guarded answers through requireStepUp, as an act's route will, so the
// guard is proven before any act exists.
func (s *opsStack) guarded(cookie string) *httptest.ResponseRecorder {
	s.t.Helper()
	act := requireStepUp(s.db.Ops, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, err := s.db.Ops.BeginTx(r.Context(), nil)
		if err != nil {
			s.t.Fatalf("begin: %v", err)
		}
		defer func() { _ = tx.Rollback() }()
		if err := spendStepUp(r.Context(), tx, signedIn(r)); err != nil {
			writeStepUpRequired(w)
			return
		}
		if err := tx.Commit(); err != nil {
			s.t.Fatalf("commit: %v", err)
		}
		apierr.WriteJSON(w, http.StatusOK, map[string]time.Time{"stepUpAt": *signedIn(r).StepUpAt})
	}))
	r := httptest.NewRequestWithContext(s.t.Context(), http.MethodPost, "/api/an-act", nil)
	r.Header = withCookie(cookie)
	w := httptest.NewRecorder()
	clock.Middleware(func() time.Time { return s.now })(act).ServeHTTP(w, r)
	return w
}

func TestStepUp_AFreshCodeStampsTheSession(t *testing.T) {
	s := newOpsStack(t, false)
	cookie := s.signedInCookie()
	s.now = opsStart.Add(20 * time.Minute)

	w := s.stepUp(cookie, "fresh-totp-token")

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body %s", w.Code, w.Body)
	}
	if at := s.stepUpAt(cookie); at == nil || !at.Equal(s.now) {
		t.Errorf("step_up_at = %v, want %s", at, s.now)
	}
}

// steppedUpCookie is a signed-in session that has just given a fresh
// code, for an act's handler spec.
func (s *opsStack) steppedUpCookie() string {
	s.t.Helper()
	cookie := s.signedInCookie()
	if w := s.stepUp(cookie, "fresh-totp-token"); w.Code != http.StatusNoContent {
		s.t.Fatalf("step up = %d, want 204; body %s", w.Code, w.Body)
	}
	return cookie
}

func (s *opsStack) refusals(reason string) int {
	s.t.Helper()
	return s.count(`SELECT count(*) FROM operator_acts a JOIN operators o ON o.id = a.operator_id
		WHERE o.identity_uid = 'op-uid-1' AND a.action = 'step_up_refused'
		  AND a.details->>'reason' = $1 AND a.created_at = $2`, reason, s.now)
}

func TestStepUp_TheEmulatorFactorStampsTheSessionOnlyWithTheEmulator(t *testing.T) {
	for _, emulator := range []bool{true, false} {
		s := newOpsStack(t, emulator)
		cookie := s.signedInCookie()

		w := s.stepUp(cookie, "fresh-phone-token")

		if emulator && w.Code != http.StatusNoContent {
			t.Errorf("with the emulator: status = %d, want 204; body %s", w.Code, w.Body)
		}
		if !emulator && (w.Code != http.StatusUnauthorized || s.refusals(refusedNoCode) != 1) {
			t.Errorf("without the emulator: status = %d, refusals %d; want 401 and one recorded", w.Code, s.refusals(refusedNoCode))
		}
	}
}

func TestStepUp_ARefusedCodeIsRecordedAndStampsNothing(t *testing.T) {
	cases := []struct {
		name, token, reason string
	}{
		{"a code Identity Platform refused leaves no token", "", refusedNoSignIn},
		{"a token the operator tenant does not verify", "a-staff-token", refusedNotVerified},
		{"another operator's sign-in", "fresh-other-token", refusedOtherOperator},
		{"a sign-in with no authenticator code", "fresh-bare-token", refusedNoCode},
		{"the session's own sign-in, long past", "totp-token", refusedStaleSignIn},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newOpsStack(t, false)
			cookie := s.signedInCookie()
			s.now = opsStart.Add(20 * time.Minute)

			w := s.stepUp(cookie, tc.token)

			if w.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401; body %s", w.Code, w.Body)
			}
			if body := decodeError(t, w); body.Code != apierr.CodeUnauthorized || body.Message != msgStepUpRefused {
				t.Errorf("body = %+v, want UNAUTHORIZED %q", body, msgStepUpRefused)
			}
			if n := s.refusals(tc.reason); n != 1 {
				t.Errorf("step_up_refused acts for %s by the operator at the try = %d, want 1", tc.reason, n)
			}
			if at := s.stepUpAt(cookie); at != nil {
				t.Errorf("step_up_at = %v, want none", at)
			}
		})
	}
}

func TestStepUp_RefusesNoSession(t *testing.T) {
	s := newOpsStack(t, false)

	w := s.stepUp("not-a-session", "fresh-totp-token")

	if w.Code != http.StatusUnauthorized || s.count(`SELECT count(*) FROM operator_acts`) != 0 {
		t.Fatalf("status = %d, want 401 with nothing recorded", w.Code)
	}
}

func TestRequireStepUp_AdmitsAnActJustSteppedUp(t *testing.T) {
	s := newOpsStack(t, false)
	cookie := s.steppedUpCookie()
	steppedUp := s.now
	s.now = s.now.Add(stepUpLimit - time.Second)

	w := s.guarded(cookie)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", w.Code, w.Body)
	}
	if want := `{"stepUpAt":"` + steppedUp.Format(time.RFC3339) + `"}`; strings.TrimSpace(w.Body.String()) != want {
		t.Errorf("body = %s, want %s: the act reads the stamp it was admitted on", w.Body, want)
	}
}

func TestSpendStepUp_AnActSpendsTheStampSoTheNextActNeedsANewCode(t *testing.T) {
	s := newOpsStack(t, false)
	cookie := s.steppedUpCookie()

	if w := s.guarded(cookie); w.Code != http.StatusOK {
		t.Fatalf("first act = %d, want 200; body %s", w.Code, w.Body)
	}
	if at := s.stepUpAt(cookie); at != nil {
		t.Errorf("step_up_at after the act = %v, want spent", at)
	}

	assertStepUpRequired(t, s.guarded(cookie))
}

func TestSpendStepUp_RefusesAStampAnotherActSpentFirst(t *testing.T) {
	s := newOpsStack(t, false)
	cookie := s.steppedUpCookie()
	at := s.stepUpAt(cookie)
	op := operator{StepUpAt: at, token: cookie}

	first, err := s.db.Ops.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = first.Rollback() }()
	if err := spendStepUp(t.Context(), first, op); err != nil {
		t.Fatalf("first spend: %v", err)
	}

	// The second act was admitted on the same stamp; its spend waits on
	// the first act's row lock and finds the stamp gone once it commits.
	second := make(chan error, 1)
	go func() {
		tx, err := s.db.Ops.BeginTx(t.Context(), nil)
		if err != nil {
			second <- err
			return
		}
		defer func() { _ = tx.Rollback() }()
		second <- spendStepUp(t.Context(), tx, op)
	}()
	if err := first.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if err := <-second; !errors.Is(err, errStepUpSpent) {
		t.Errorf("second spend = %v, want errStepUpSpent", err)
	}
}

func TestSpendStepUp_RefusesAStampNoLongerTheSessions(t *testing.T) {
	s := newOpsStack(t, false)
	cookie := s.steppedUpCookie()
	admitted := s.stepUpAt(cookie)
	s.now = s.now.Add(time.Minute)
	s.stepUp(cookie, "fresh-totp-token")

	tx, err := s.db.Ops.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := spendStepUp(t.Context(), tx, operator{StepUpAt: admitted, token: cookie}); !errors.Is(err, errStepUpSpent) {
		t.Errorf("spend on a replaced stamp = %v, want errStepUpSpent", err)
	}
}

func TestRequireStepUp_RefusesASessionNeverSteppedUp(t *testing.T) {
	s := newOpsStack(t, false)
	cookie := s.signedInCookie()

	w := s.guarded(cookie)

	assertStepUpRequired(t, w)
}

func TestRequireStepUp_RefusesAStaleStepUp(t *testing.T) {
	s := newOpsStack(t, false)
	cookie := s.steppedUpCookie()
	s.now = s.now.Add(stepUpLimit)

	w := s.guarded(cookie)

	assertStepUpRequired(t, w)
}

func TestRequireStepUp_RefusesNoSession(t *testing.T) {
	s := newOpsStack(t, false)

	w := s.guarded("not-a-session")

	if w.Code != http.StatusUnauthorized || decodeError(t, w).Code != apierr.CodeUnauthorized {
		t.Fatalf("status = %d body %s, want 401 UNAUTHORIZED", w.Code, w.Body)
	}
}

func assertStepUpRequired(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body %s", w.Code, w.Body)
	}
	if body := decodeError(t, w); body.Code != apierr.CodeSecondFactorRequired || body.Message != msgStepUpRequired {
		t.Errorf("body = %+v, want SECOND_FACTOR_REQUIRED %q", body, msgStepUpRequired)
	}
}
