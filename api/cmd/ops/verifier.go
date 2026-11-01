package main

import (
	"context"
	"fmt"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
)

// The SDK refuses a token from another tenant (a Staff token carries none) and checks freshness against wall time inside itself, the #773 exemption.
type tenantVerifier struct {
	client *auth.TenantClient
}

func newTenantVerifier(ctx context.Context, projectID, tenantID string) (Verifier, error) {
	// coverage:ignore reason: requires GCP credentials or the Auth emulator, not exercised by unit tests
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: projectID})
	if err != nil {
		// coverage:ignore reason: requires GCP credentials or the Auth emulator, not exercised by unit tests
		return nil, fmt.Errorf("init firebase app: %w", err)
	}
	// coverage:ignore reason: requires GCP credentials or the Auth emulator, not exercised by unit tests
	client, err := app.Auth(ctx)
	if err != nil {
		// coverage:ignore reason: requires GCP credentials or the Auth emulator, not exercised by unit tests
		return nil, fmt.Errorf("init auth client: %w", err)
	}
	// coverage:ignore reason: requires GCP credentials or the Auth emulator, not exercised by unit tests
	tenant, err := client.TenantManager.AuthForTenant(tenantID)
	if err != nil {
		// coverage:ignore reason: requires GCP credentials or the Auth emulator, not exercised by unit tests
		return nil, fmt.Errorf("tenant client: %w", err)
	}
	// coverage:ignore reason: requires GCP credentials or the Auth emulator, not exercised by unit tests
	return &tenantVerifier{client: tenant}, nil
}

func (v *tenantVerifier) Verify(ctx context.Context, idToken string) (Identity, error) {
	// coverage:ignore reason: requires a real operator token, exercised by ops/'s e2e suite
	token, err := v.client.VerifyIDToken(ctx, idToken)
	if err != nil {
		// coverage:ignore reason: requires a real operator token, exercised by ops/'s e2e suite
		return Identity{}, fmt.Errorf("verify operator token: %w", err)
	}
	// coverage:ignore reason: requires a real operator token, exercised by ops/'s e2e suite
	identity := identityOf(token.UID, token.Claims)
	// coverage:ignore reason: requires a real operator token, exercised by ops/'s e2e suite
	identity.AuthTime = time.Unix(token.AuthTime, 0)
	// coverage:ignore reason: requires a real operator token, exercised by ops/'s e2e suite
	return identity, nil
}

func identityOf(uid string, claims map[string]any) Identity {
	email, _ := claims["email"].(string)
	name, _ := claims["name"].(string)
	fb, _ := claims["firebase"].(map[string]any)
	factor, _ := fb["sign_in_second_factor"].(string)
	return Identity{UID: uid, Email: email, Name: name, SecondFactor: factor}
}
