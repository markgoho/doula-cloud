package internalauth_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"doula-cloud/api/internal/internalauth"
)

const (
	audience  = "https://doula-api-850855848778.us-central1.run.app"
	scheduler = "internal-caller@doula-cloud.iam.gserviceaccount.com"

	authorization  = "Authorization"
	internalHeader = "X-Internal-Secret"
	bearerToken    = "Bearer signed-token"
)

// acceptToken is a ValidateFunc that stands in for Google: it answers
// with the caller the test names, and records what it was asked, so a
// test can assert the audience actually reached the validator.
func acceptToken(email string, gotAudience *string) internalauth.ValidateFunc {
	return func(_ context.Context, _, aud string) (string, error) {
		if gotAudience != nil {
			*gotAudience = aud
		}
		return email, nil
	}
}

func rejectToken(_ context.Context, _, _ string) (string, error) {
	return "", errors.New("token rejected")
}

func request(t *testing.T, headers map[string]string) *http.Request {
	t.Helper()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/internal/outboxes/drain", nil)
	for name, value := range headers {
		r.Header.Set(name, value)
	}
	return r
}

func TestAllow_AcceptsAnAllowlistedCallersToken(t *testing.T) {
	var got string
	g := internalauth.New(internalauth.Config{
		Audience: audience,
		Callers:  []string{scheduler},
		Validate: acceptToken(scheduler, &got),
	})

	if !g.Allow(request(t, map[string]string{authorization: bearerToken})) {
		t.Fatal("Allow() = false for an allowlisted caller's token, want true")
	}
	if got != audience {
		t.Errorf("validator saw audience %q, want %q", got, audience)
	}
}

func TestAllow_AcceptsBearerInAnyCase(t *testing.T) {
	g := internalauth.New(internalauth.Config{
		Audience: audience,
		Callers:  []string{scheduler},
		Validate: acceptToken(scheduler, nil),
	})

	if !g.Allow(request(t, map[string]string{authorization: "bearer signed-token"})) {
		t.Error("Allow() = false for a lowercase bearer scheme, want true")
	}
}

// The allowlist is compared case-insensitively and trimmed, because it
// arrives as a hand-edited configuration string.
func TestAllow_NormalizesTheAllowlistAndTheClaim(t *testing.T) {
	g := internalauth.New(internalauth.Config{
		Audience: audience,
		Callers:  []string{"  " + scheduler + "  ", "", "Other@doula-cloud.iam.gserviceaccount.com"},
		Validate: acceptToken("OTHER@doula-cloud.iam.gserviceaccount.com", nil),
	})

	if !g.Allow(request(t, map[string]string{authorization: bearerToken})) {
		t.Error("Allow() = false for an allowlisted caller in a different case, want true")
	}
}

func TestAllow_RefusesTokens(t *testing.T) {
	tests := []struct {
		name   string
		cfg    internalauth.Config
		header string
	}{
		{
			name: "a caller who is not on the allowlist",
			cfg: internalauth.Config{
				Audience: audience,
				Callers:  []string{scheduler},
				Validate: acceptToken("someone-else@example.com", nil),
			},
			header: bearerToken,
		},
		{
			name: "a token the validator refuses",
			cfg: internalauth.Config{
				Audience: audience,
				Callers:  []string{scheduler},
				Validate: rejectToken,
			},
			header: bearerToken,
		},
		{
			name: "no audience configured",
			cfg: internalauth.Config{
				Callers:  []string{scheduler},
				Validate: acceptToken(scheduler, nil),
			},
			header: bearerToken,
		},
		{
			name: "an empty allowlist",
			cfg: internalauth.Config{
				Audience: audience,
				Validate: acceptToken(scheduler, nil),
			},
			header: bearerToken,
		},
		{
			name:   "no validator configured",
			cfg:    internalauth.Config{Audience: audience, Callers: []string{scheduler}},
			header: bearerToken,
		},
		{
			name: "a scheme that is not bearer",
			cfg: internalauth.Config{
				Audience: audience,
				Callers:  []string{scheduler},
				Validate: acceptToken(scheduler, nil),
			},
			header: "Basic signed-token",
		},
		{
			name: "an empty bearer token",
			cfg: internalauth.Config{
				Audience: audience,
				Callers:  []string{scheduler},
				Validate: acceptToken(scheduler, nil),
			},
			header: "Bearer   ",
		},
		{
			name: "a header shorter than the scheme itself",
			cfg: internalauth.Config{
				Audience: audience,
				Callers:  []string{scheduler},
				Validate: acceptToken(scheduler, nil),
			},
			header: "Bear",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := internalauth.New(tc.cfg)
			if g.Allow(request(t, map[string]string{authorization: tc.header})) {
				t.Error("Allow() = true, want false")
			}
		})
	}
}

func TestAllow_TheSecretHeaderIsTheLocalMechanism(t *testing.T) {
	g := internalauth.FromSecret("e2e-worker-secret")

	if !g.Allow(request(t, map[string]string{internalHeader: "e2e-worker-secret"})) {
		t.Error("Allow() = false for the configured secret, want true")
	}
	if g.Allow(request(t, map[string]string{internalHeader: "wrong"})) {
		t.Error("Allow() = true for the wrong secret, want false")
	}
	if g.Allow(request(t, nil)) {
		t.Error("Allow() = true for a request carrying nothing, want false")
	}
}

// Production configures no secret. An empty configured secret must
// refuse the header rather than match a request that also sends nothing,
// which is the behavior outbox.ProcessHandler carried before #1052.
func TestAllow_NoSecretConfiguredRefusesTheHeader(t *testing.T) {
	g := internalauth.New(internalauth.Config{
		Audience: audience,
		Callers:  []string{scheduler},
		Validate: acceptToken(scheduler, nil),
	})

	if g.Allow(request(t, map[string]string{internalHeader: ""})) {
		t.Error("Allow() = true for an empty secret against an empty configured secret, want false")
	}
}

// Require is the refusal every internal endpoint shares, so that the
// five of them cannot drift into answering an unauthenticated caller
// differently from each other.
func TestRequire_WritesOneRefusal(t *testing.T) {
	g := internalauth.FromSecret("e2e-worker-secret")

	allowed := httptest.NewRecorder()
	if !g.Require(allowed, request(t, map[string]string{internalHeader: "e2e-worker-secret"})) {
		t.Fatal("Require() = false for the configured secret, want true")
	}
	if allowed.Code != http.StatusOK {
		t.Errorf("an allowed request was written %d, want the handler left untouched", allowed.Code)
	}

	refused := httptest.NewRecorder()
	if g.Require(refused, request(t, nil)) {
		t.Fatal("Require() = true for a request carrying nothing, want false")
	}
	if refused.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", refused.Code, http.StatusUnauthorized)
	}
	if body := refused.Body.String(); !strings.Contains(body, "UNAUTHORIZED") {
		t.Errorf("body = %q, want the shared unauthorized error", body)
	}
}

// A handler wired with no guard at all fails closed. It is a
// programming error rather than a configuration one, but a 401 is a far
// better answer to it than a panic on a public endpoint.
func TestAllow_NilGuardRefusesEverything(t *testing.T) {
	var g *internalauth.Guard

	if g.Allow(request(t, map[string]string{internalHeader: "anything"})) {
		t.Error("Allow() = true on a nil guard, want false")
	}
}
