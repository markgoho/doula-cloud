package staffauth_test

import (
	"net/http"
	"strconv"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// signupIPBudget is bootstrapRules' IP ceiling for POST /api/staff/signup
// (mount.go: ratelimit.IPRule(50, time.Hour)). Restated here rather than
// exported from the package under test: this test's whole job is to hold
// the mounted route to a number, so reading the number out of the code it
// is checking would make it pass whatever that code became.
const signupIPBudget = 50

// burstSignupBody is a well-formed signup request. Its contents never
// matter: every request below is refused before the handler reads it,
// either by the missing Bearer token or by the limiter in front of it. A
// function rather than a package-level var so nothing else in
// staffauth_test can mutate the value this test sends.
func burstSignupBody() staffauth.SignupRequest {
	return staffauth.SignupRequest{PracticeName: "Riverside Doulas", StaffName: jamieOwnerName, WorkState: "NY"}
}

// TestSignupRefusesAGenuineBurst is the measurement behind #1138 and the
// guardrail over its fix. It drives the real mounted route -- not
// ratelimit.Wrap around a stand-in handler, which is what
// ratelimit_test.go already covers -- so it answers the question the e2e
// harness actually runs into: how many times may one address reach
// POST /api/staff/signup in an hour before the 429 arrives?
//
// Every request here carries no Bearer token, which is deliberate.
// BearerTokenRule skips a request with no token (rules.go), so the IP
// dimension is the only one counting -- the same dimension a repeated
// Playwright batch spends, since every seeded founding Owner mints a
// fresh Identity Platform token and so never spends the per-token budget
// of 5. Wrap sits outside SignupHandler, so a request under the ceiling
// is refused by authn.BeginBootstrap for its missing credential (401)
// and a request over it never reaches the handler at all (429). Those
// two statuses are what separates "the limiter let this through" from
// "the limiter refused it", with no Identity Platform account needed to
// tell them apart.
//
// app/e2e/staffSignup.ts's escape hatch (#1138) clears this endpoint's
// bucket rows straight out of the e2e stack's own Postgres. This test is
// what keeps that from quietly disabling the protection: the refusal
// still has to be here for a harness to have anything to clear.
func TestSignupRefusesAGenuineBurst(t *testing.T) {
	db := testdb.New(t)
	srv := newSignupServer(authntest.Verifier{}, db)
	t.Cleanup(srv.Close)

	for i := 1; i <= signupIPBudget; i++ {
		resp := postSignup(t, srv, "", burstSignupBody())
		status := resp.StatusCode
		_ = resp.Body.Close()
		// 401, not merely "not 429": a mount that had stopped reaching this
		// handler at all would answer 404 or 500 for fifty requests and
		// then fail below with a message about the ceiling, which is the
		// wrong story about the wrong bug.
		if status != http.StatusUnauthorized {
			t.Fatalf("request %d of %d: status = %d, want 401 -- the limiter let it through to a handler that refuses a missing credential",
				i, signupIPBudget, status)
		}
	}

	resp := postSignup(t, srv, "", burstSignupBody())
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("request %d: status = %d, want 429 -- a genuine burst past %d in an hour must still be refused",
			signupIPBudget+1, resp.StatusCode, signupIPBudget)
	}
	if got := apierrtest.Decode(t, resp).Code; got != apierr.CodeRateLimited {
		t.Errorf("code = %q, want %q", got, apierr.CodeRateLimited)
	}
	// docs/api-design.md section 6 names all three of these on a refusal.
	for header, want := range map[string]string{
		"RateLimit-Limit":     strconv.Itoa(signupIPBudget),
		"RateLimit-Remaining": "0",
	} {
		if got := resp.Header.Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	if resp.Header.Get("Retry-After") == "" {
		t.Error("Retry-After header missing -- a refused caller cannot tell when to come back")
	}
}
