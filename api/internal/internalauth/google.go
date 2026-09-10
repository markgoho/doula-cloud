package internalauth

import (
	"context"
	"fmt"

	"google.golang.org/api/idtoken"
)

// GoogleValidator is the production ValidateFunc: it verifies that the
// token was signed by Google, that it has not expired, and that it names
// audience in `aud`, then returns the verified email claim on it.
//
// idtoken.Validate does the signature and audience work and caches
// Google's signing certificates itself, so this adds only the claim
// reading. An unverified email is refused rather than trusted: a Google
// service account token always carries email_verified true, so a token
// without it is not the caller this boundary is for.
func GoogleValidator(ctx context.Context, token, audience string) (string, error) {
	// coverage:ignore reason: verifies against Google's live signing certificates, not exercised by unit tests
	payload, err := idtoken.Validate(ctx, token, audience)
	// coverage:ignore reason: verifies against Google's live signing certificates, not exercised by unit tests
	if err != nil {
		// coverage:ignore reason: verifies against Google's live signing certificates, not exercised by unit tests
		return "", fmt.Errorf("internalauth: validate id token: %w", err)
	}
	// coverage:ignore reason: verifies against Google's live signing certificates, not exercised by unit tests
	email, _ := payload.Claims["email"].(string)
	// coverage:ignore reason: verifies against Google's live signing certificates, not exercised by unit tests
	verified, _ := payload.Claims["email_verified"].(bool)
	// coverage:ignore reason: verifies against Google's live signing certificates, not exercised by unit tests
	if email == "" || !verified {
		// coverage:ignore reason: verifies against Google's live signing certificates, not exercised by unit tests
		return "", fmt.Errorf("internalauth: id token carries no verified email claim")
	}
	// coverage:ignore reason: verifies against Google's live signing certificates, not exercised by unit tests
	return email, nil
}
