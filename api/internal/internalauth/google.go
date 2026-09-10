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
// Google's signing certificates itself, so the only thing left here is
// reading the claim -- which is emailFromClaims, kept separate because
// it is the half that can be exercised without reaching Google.
func GoogleValidator(ctx context.Context, token, audience string) (string, error) {
	// coverage:ignore reason: verifies against Google's live signing certificates, not exercised by unit tests
	payload, err := idtoken.Validate(ctx, token, audience)
	// coverage:ignore reason: verifies against Google's live signing certificates, not exercised by unit tests
	if err != nil {
		// coverage:ignore reason: verifies against Google's live signing certificates, not exercised by unit tests
		return "", fmt.Errorf("internalauth: validate id token: %w", err)
	}
	// coverage:ignore reason: verifies against Google's live signing certificates, not exercised by unit tests
	return emailFromClaims(payload.Claims)
}

// emailFromClaims reads the calling service account out of a verified
// token's claims.
//
// An unverified email is refused rather than trusted: a Google service
// account token always carries email_verified true, so a token without
// it is not the caller this boundary is for. The allowlist is compared
// against this value, so a claim that is absent, empty, or the wrong
// type must produce an error and not an empty string that some future
// allowlist entry could match.
func emailFromClaims(claims map[string]any) (string, error) {
	email, _ := claims["email"].(string)
	verified, _ := claims["email_verified"].(bool)
	if email == "" || !verified {
		return "", fmt.Errorf("internalauth: id token carries no verified email claim")
	}
	return email, nil
}
