package main

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/testdb"
)

// fakeVerifier answers for the tokens it was given, and refuses every
// other token the way the tenant client refuses a Staff token.
type fakeVerifier map[string]Identity

func (f fakeVerifier) Verify(_ context.Context, idToken string) (Identity, error) {
	identity, ok := f[idToken]
	if !ok {
		return Identity{}, errors.New("not a token of the operator tenant")
	}
	return identity, nil
}

const (
	opName  = "Mark Goho"
	opEmail = "mark@doula.cloud"
	opUID   = "op-uid-1"
)

var (
	testOrigin = "http://localhost:7173"
	opsStart   = time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	justNow    = time.Now() //nolint:forbidigo // #773: an Identity Platform auth_time is real wall time, as staffauth/reauth.go says
	tokens     = fakeVerifier{
		"totp-token":  {UID: opUID, Email: opEmail, Name: opName, SecondFactor: factorTOTP},
		"phone-token": {UID: opUID, Email: opEmail, Name: opName, SecondFactor: "phone"},
		"bare-token":  {UID: opUID, Email: opEmail, Name: opName},
		"other-token": {UID: "op-uid-2", Email: "second@doula.cloud", SecondFactor: factorTOTP},
		// A step-up's tokens: a sign-in just now, its auth_time real wall
		// time as Identity Platform's is.
		"fresh-totp-token":  {UID: opUID, Email: opEmail, Name: opName, SecondFactor: factorTOTP, AuthTime: justNow},
		"fresh-phone-token": {UID: opUID, Email: opEmail, Name: opName, SecondFactor: "phone", AuthTime: justNow},
		"fresh-bare-token":  {UID: opUID, Email: opEmail, Name: opName, AuthTime: justNow},
		"fresh-other-token": {UID: "op-uid-2", Email: "second@doula.cloud", SecondFactor: factorTOTP, AuthTime: justNow},
	}
)

// opsStack is doula-ops's router against a fresh database as ops_runtime,
// with a clock the test moves.
type opsStack struct {
	t       *testing.T
	db      *testdb.DB
	handler http.Handler
	now     time.Time
}

func newOpsStack(t *testing.T, acceptEmulatorFactor bool) *opsStack {
	t.Helper()
	s := &opsStack{t: t, db: testdb.New(t), now: opsStart}
	s.handler, _ = routes(Deps{
		DB:                   s.db.Ops,
		Now:                  func() time.Time { return s.now },
		Verifier:             tokens,
		AcceptEmulatorFactor: acceptEmulatorFactor,
		Origins:              []string{testOrigin},
	})
	return s
}

// call sends method to /api/session, the route every test here drives.
func (s *opsStack) call(method string, header http.Header) *httptest.ResponseRecorder {
	s.t.Helper()
	r := httptest.NewRequestWithContext(s.t.Context(), method, "/api/session", nil)
	maps.Copy(r.Header, header)
	w := httptest.NewRecorder()
	s.handler.ServeHTTP(w, r)
	return w
}

func (s *opsStack) signIn(token string) *httptest.ResponseRecorder {
	s.t.Helper()
	return s.call(http.MethodPost, http.Header{"Authorization": {"Bearer " + token}})
}

func (s *opsStack) signedInCookie() string {
	s.t.Helper()
	w := s.signIn("totp-token")
	if w.Code != http.StatusOK {
		s.t.Fatalf("sign in = %d, want 200; body %s", w.Code, w.Body)
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == authn.SessionCookieName {
			return c.Value
		}
	}
	s.t.Fatalf("sign in set no %s cookie", authn.SessionCookieName)
	return ""
}

func withCookie(token string) http.Header {
	return http.Header{"Cookie": {authn.SessionCookieName + "=" + token}}
}

func (s *opsStack) count(query string, args ...any) int {
	s.t.Helper()
	var n int
	if err := s.db.Admin.QueryRowContext(s.t.Context(), query, args...).Scan(&n); err != nil {
		s.t.Fatalf("count %q: %v", query, err)
	}
	return n
}

func decodeError(t *testing.T, w *httptest.ResponseRecorder) apierr.APIError {
	t.Helper()
	var body apierr.APIError
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %s is not the API error structure: %v", w.Body, err)
	}
	return body
}

func TestSignIn_AnAuthenticatorAppSignInMintsASessionAndRecordsTheOperatorAndTheAct(t *testing.T) {
	s := newOpsStack(t, false)

	w := s.signIn("totp-token")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", w.Code, w.Body)
	}
	var cookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == authn.SessionCookieName {
			cookie = c
		}
	}
	if cookie == nil || !cookie.HttpOnly || !cookie.Secure || cookie.MaxAge != int(absoluteLimit.Seconds()) {
		t.Fatalf("cookie = %+v, want an HttpOnly Secure %s lasting the absolute limit", cookie, authn.SessionCookieName)
	}
	var name, email string
	if err := s.db.Admin.QueryRowContext(t.Context(),
		`SELECT name, email FROM operators WHERE identity_uid = 'op-uid-1'`).Scan(&name, &email); err != nil {
		t.Fatalf("read operator: %v", err)
	}
	if name != "Mark Goho" || email != opEmail {
		t.Errorf("operator = %q %q, want Mark Goho mark@doula.cloud", name, email)
	}
	if n := s.count(`SELECT count(*) FROM operator_acts a JOIN operators o ON o.id = a.operator_id
		WHERE o.identity_uid = 'op-uid-1' AND a.action = 'sign_in' AND a.created_at = $1`, opsStart); n != 1 {
		t.Errorf("sign_in acts at the sign-in instant = %d, want 1", n)
	}
}

func TestSignIn_ASecondSignInKeepsTheFirstNameAndEmail(t *testing.T) {
	s := newOpsStack(t, false)
	s.signIn("totp-token")
	s.now = s.now.Add(time.Minute)
	if _, err := s.db.Admin.ExecContext(t.Context(), `UPDATE operators SET name = 'First Name' WHERE identity_uid = 'op-uid-1'`); err != nil {
		t.Fatalf("rename: %v", err)
	}

	s.signIn("totp-token")

	if n := s.count(`SELECT count(*) FROM operators WHERE identity_uid = 'op-uid-1' AND name = 'First Name'`); n != 1 {
		t.Errorf("operators named as first recorded = %d, want 1", n)
	}
	if n := s.count(`SELECT count(*) FROM operator_acts WHERE action = 'sign_in'`); n != 2 {
		t.Errorf("sign_in acts = %d, want 2", n)
	}
}

func TestSignIn_AnOperatorWithNoNameIsRecordedByEmail(t *testing.T) {
	s := newOpsStack(t, false)

	if w := s.signIn("other-token"); w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	if n := s.count(`SELECT count(*) FROM operators WHERE name = 'second@doula.cloud'`); n != 1 {
		t.Errorf("operators named by email = %d, want 1", n)
	}
}

func TestSignIn_RefusesAStaffToken(t *testing.T) {
	s := newOpsStack(t, false)

	w := s.signIn("a-staff-token")

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
	if body := decodeError(t, w); body.Code != apierr.CodeUnauthorized {
		t.Errorf("code = %s, want %s", body.Code, apierr.CodeUnauthorized)
	}
	if n := s.count(`SELECT count(*) FROM operator_sessions`); n != 0 {
		t.Errorf("sessions = %d, want 0", n)
	}
}

func TestSignIn_RefusesARequestWithNoToken(t *testing.T) {
	s := newOpsStack(t, false)

	w := s.call(http.MethodPost, nil)

	if w.Code != http.StatusUnauthorized || decodeError(t, w).Code != apierr.CodeUnauthorized {
		t.Fatalf("status = %d body %s, want 401 UNAUTHORIZED", w.Code, w.Body)
	}
}

func TestSignIn_RefusesALoginWithNoSecondFactorAndSendsItToEnroll(t *testing.T) {
	s := newOpsStack(t, true)

	w := s.signIn("bare-token")

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
	if body := decodeError(t, w); body.Code != apierr.CodeMFARequired {
		t.Errorf("code = %s, want %s", body.Code, apierr.CodeMFARequired)
	}
	if n := s.count(`SELECT count(*) FROM operator_sessions`) + s.count(`SELECT count(*) FROM operators`); n != 0 {
		t.Errorf("sessions and operators = %d, want 0", n)
	}
}

func TestSignIn_RefusesTheEmulatorFactorWithoutTheEmulator(t *testing.T) {
	s := newOpsStack(t, false)

	w := s.signIn("phone-token")

	if w.Code != http.StatusForbidden || decodeError(t, w).Code != apierr.CodeMFARequired {
		t.Fatalf("status = %d body %s, want 403 MFA_REQUIRED", w.Code, w.Body)
	}
}

func TestSignIn_AcceptsTheEmulatorFactorWithTheEmulator(t *testing.T) {
	s := newOpsStack(t, true)

	if w := s.signIn("phone-token"); w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", w.Code, w.Body)
	}
}

func TestSignIn_RefusesACrossSiteOrigin(t *testing.T) {
	s := newOpsStack(t, false)

	w := s.call(http.MethodPost, http.Header{
		"Authorization": {"Bearer totp-token"},
		"Origin":        {"https://evil.example"},
	})

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

func TestSession_NamesTheSignedInOperator(t *testing.T) {
	s := newOpsStack(t, false)
	cookie := s.signedInCookie()

	w := s.call(http.MethodGet, withCookie(cookie))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", w.Code, w.Body)
	}
	if got := strings.TrimSpace(w.Body.String()); got != `{"name":"Mark Goho","email":"mark@doula.cloud"}` {
		t.Errorf("body = %s", got)
	}
}

func TestSession_RefusesNoCookie(t *testing.T) {
	s := newOpsStack(t, false)

	w := s.call(http.MethodGet, nil)

	if w.Code != http.StatusUnauthorized || decodeError(t, w).Code != apierr.CodeUnauthorized {
		t.Fatalf("status = %d body %s, want 401 UNAUTHORIZED", w.Code, w.Body)
	}
}

func TestSession_RefusesAnUnknownCookie(t *testing.T) {
	s := newOpsStack(t, false)

	w := s.call(http.MethodGet, withCookie("not-a-session"))

	if w.Code != http.StatusUnauthorized || decodeError(t, w).Code != apierr.CodeUnauthorized {
		t.Fatalf("status = %d body %s, want 401 UNAUTHORIZED", w.Code, w.Body)
	}
}

func TestSession_EndsAfterAnHourIdle(t *testing.T) {
	s := newOpsStack(t, false)
	cookie := s.signedInCookie()

	s.now = opsStart.Add(idleLimit)
	w := s.call(http.MethodGet, withCookie(cookie))

	if w.Code != http.StatusUnauthorized || decodeError(t, w).Code != apierr.CodeUnauthorized {
		t.Fatalf("after an hour idle: status = %d body %s, want 401 UNAUTHORIZED", w.Code, w.Body)
	}
}

func TestSession_UseKeepsItAlivePastAnHour(t *testing.T) {
	s := newOpsStack(t, false)
	cookie := s.signedInCookie()

	for elapsed := 50 * time.Minute; elapsed <= 2*time.Hour+30*time.Minute; elapsed += 50 * time.Minute {
		s.now = opsStart.Add(elapsed)
		if w := s.call(http.MethodGet, withCookie(cookie)); w.Code != http.StatusOK {
			t.Fatalf("used at %s: status = %d, want 200", elapsed, w.Code)
		}
	}
}

func TestSession_EndsAfterEightHoursWhateverItsUse(t *testing.T) {
	s := newOpsStack(t, false)
	cookie := s.signedInCookie()

	for elapsed := 50 * time.Minute; elapsed < absoluteLimit; elapsed += 50 * time.Minute {
		s.now = opsStart.Add(elapsed)
		if w := s.call(http.MethodGet, withCookie(cookie)); w.Code != http.StatusOK {
			t.Fatalf("used at %s: status = %d, want 200", elapsed, w.Code)
		}
	}
	s.now = opsStart.Add(absoluteLimit)
	w := s.call(http.MethodGet, withCookie(cookie))

	if w.Code != http.StatusUnauthorized || decodeError(t, w).Code != apierr.CodeUnauthorized {
		t.Fatalf("at eight hours: status = %d body %s, want 401 UNAUTHORIZED", w.Code, w.Body)
	}
}

func TestSignOut_EndsTheSessionAndRecordsTheAct(t *testing.T) {
	s := newOpsStack(t, false)
	cookie := s.signedInCookie()
	s.now = opsStart.Add(10 * time.Minute)

	w := s.call(http.MethodDelete, withCookie(cookie))

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body %s", w.Code, w.Body)
	}
	cleared := false
	for _, c := range w.Result().Cookies() {
		cleared = cleared || (c.Name == authn.SessionCookieName && c.MaxAge < 0)
	}
	if !cleared {
		t.Error("sign out did not clear the cookie")
	}
	if n := s.count(`SELECT count(*) FROM operator_acts a JOIN operators o ON o.id = a.operator_id
		WHERE o.identity_uid = 'op-uid-1' AND a.action = 'sign_out' AND a.created_at = $1`, s.now); n != 1 {
		t.Errorf("sign_out acts at the sign-out instant = %d, want 1", n)
	}
	if w := s.call(http.MethodGet, withCookie(cookie)); w.Code != http.StatusUnauthorized {
		t.Errorf("session after sign out = %d, want 401", w.Code)
	}
}

func TestSignOut_RefusesAnExpiredSession(t *testing.T) {
	s := newOpsStack(t, false)
	cookie := s.signedInCookie()
	s.now = opsStart.Add(absoluteLimit)

	w := s.call(http.MethodDelete, withCookie(cookie))

	if w.Code != http.StatusUnauthorized || decodeError(t, w).Code != apierr.CodeUnauthorized {
		t.Fatalf("status = %d body %s, want 401 UNAUTHORIZED", w.Code, w.Body)
	}
	if n := s.count(`SELECT count(*) FROM operator_acts WHERE action = 'sign_out'`); n != 0 {
		t.Errorf("sign_out acts = %d, want 0", n)
	}
}

func TestNoSignupRoute(t *testing.T) {
	_, mounted := routes(Deps{})
	for _, route := range mounted {
		if strings.Contains(strings.ToLower(route), "signup") || strings.Contains(route, "accounts") {
			t.Errorf("doula-ops mounts %s; operator logins are made by hand (ADR-0062)", route)
		}
	}
}
