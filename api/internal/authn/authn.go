// Package authn verifies GCP Identity Platform ID tokens. It defines a
// small Verifier interface so HTTP middleware can be tested against a
// fake implementation instead of a live Identity Platform project.
package authn

import (
	"context"
	"fmt"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
)

// VerifiedToken is the identity a Verifier extracts from a valid ID
// token. Identity Platform provides identity only -- no custom claims --
// so this is deliberately just a uid; the BFF resolves authorization
// (which Practice/Client the caller may act as) from the database on
// every request.
type VerifiedToken struct {
	UID string
}

// Verifier checks a raw ID token and returns the identity it carries, or
// an error if the token is missing, expired, malformed, or not signed by
// the configured Identity Platform project.
type Verifier interface {
	VerifyIDToken(ctx context.Context, idToken string) (*VerifiedToken, error)
}

// FirebaseVerifier verifies tokens via the GCP Identity Platform Admin
// SDK. Identity Platform is built on the Firebase Auth backend, so the
// Firebase Admin SDK for Go is Google's supported client for verifying
// its tokens.
type FirebaseVerifier struct {
	client *auth.Client
}

// NewFirebaseVerifier creates a FirebaseVerifier scoped to the given GCP
// project, using Application Default Credentials to authenticate to the
// Admin SDK.
func NewFirebaseVerifier(ctx context.Context, projectID string) (*FirebaseVerifier, error) {
	// coverage:ignore reason: requires real GCP credentials and network access, not exercised by unit tests
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: projectID})
	if err != nil {
		// coverage:ignore reason: requires real GCP credentials and network access, not exercised by unit tests
		return nil, fmt.Errorf("authn: init firebase app: %w", err)
	}

	// coverage:ignore reason: requires real GCP credentials and network access, not exercised by unit tests
	client, err := app.Auth(ctx)
	if err != nil {
		// coverage:ignore reason: requires real GCP credentials and network access, not exercised by unit tests
		return nil, fmt.Errorf("authn: init auth client: %w", err)
	}

	// coverage:ignore reason: requires real GCP credentials and network access, not exercised by unit tests
	return &FirebaseVerifier{client: client}, nil
}

// VerifyIDToken verifies idToken against Identity Platform and returns
// the caller's uid.
func (v *FirebaseVerifier) VerifyIDToken(ctx context.Context, idToken string) (*VerifiedToken, error) {
	// coverage:ignore reason: requires a real Identity Platform token, not exercised by unit tests
	token, err := v.client.VerifyIDToken(ctx, idToken)
	if err != nil {
		// coverage:ignore reason: requires a real Identity Platform token, not exercised by unit tests
		return nil, fmt.Errorf("authn: verify id token: %w", err)
	}
	// coverage:ignore reason: requires a real Identity Platform token, not exercised by unit tests
	return &VerifiedToken{UID: token.UID}, nil
}
