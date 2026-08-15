// Package authn verifies GCP Identity Platform ID tokens. It defines a
// small Verifier interface so HTTP middleware can be tested against a
// fake implementation instead of a live Identity Platform project.
package authn

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
)

// BearerToken extracts the token from an HTTP Authorization header of the
// form "Bearer <token>". It returns ("", false) if the header is missing,
// malformed, or contains an empty token.
func BearerToken(r *http.Request) (string, bool) {
	const prefix = "Bearer "
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	token := strings.TrimPrefix(header, prefix)
	if token == "" {
		return "", false
	}
	return token, true
}

// Begin extracts the Bearer token from r, verifies it with verifier, and
// opens a transaction on db. It writes the appropriate HTTP error (401 for
// missing or invalid token, 500 for DB connection failure) and returns
// ok=false if any step fails. On success it returns the open *sql.Tx and
// the verified caller's UID. Callers must ensure the returned transaction is
// rolled back or committed.
func Begin(w http.ResponseWriter, r *http.Request, verifier Verifier, db *sql.DB) (*sql.Tx, string, bool) {
	idToken, ok := BearerToken(r)
	if !ok {
		http.Error(w, "missing bearer token", http.StatusUnauthorized)
		return nil, "", false
	}

	verified, err := verifier.VerifyIDToken(r.Context(), idToken)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return nil, "", false
	}

	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		// coverage:ignore reason: DB connection failure, not exercised by unit tests
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}

	return tx, verified.UID, true
}

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
