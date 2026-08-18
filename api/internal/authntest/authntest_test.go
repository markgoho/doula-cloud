package authntest_test

import (
	"errors"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
)

const testUID = "staff-uid"

func TestVerifyIDToken_ReturnsUID(t *testing.T) {
	verified, err := authntest.Verifier{UID: testUID}.VerifyIDToken(t.Context(), "any-token")
	if err != nil {
		t.Fatalf("VerifyIDToken: %v", err)
	}
	if verified.UID != testUID {
		t.Fatalf("UID = %q, want %q", verified.UID, testUID)
	}
}

func TestVerifyIDToken_ReturnsErr(t *testing.T) {
	wantErr := errors.New("bad token")

	verified, err := authntest.Verifier{UID: testUID, Err: wantErr}.VerifyIDToken(t.Context(), "any-token")
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
	if verified != nil {
		t.Fatalf("verified = %v, want nil", verified)
	}
}

func TestMintSessionCookie_ReturnsCookie(t *testing.T) {
	cookie, err := authntest.Verifier{UID: testUID}.MintSessionCookie(t.Context(), "any-id-token", time.Hour)
	if err != nil {
		t.Fatalf("MintSessionCookie: %v", err)
	}
	if cookie == "" {
		t.Fatal("cookie = \"\", want non-empty")
	}
}

func TestMintSessionCookie_ReturnsMintErr(t *testing.T) {
	wantErr := errors.New("mint failed")

	cookie, err := authntest.Verifier{UID: testUID, MintErr: wantErr}.MintSessionCookie(t.Context(), "any-id-token", time.Hour)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
	if cookie != "" {
		t.Fatalf("cookie = %q, want \"\"", cookie)
	}
}

func TestVerifySessionCookie_ReturnsUID(t *testing.T) {
	verified, err := authntest.Verifier{UID: testUID}.VerifySessionCookie(t.Context(), "any-cookie")
	if err != nil {
		t.Fatalf("VerifySessionCookie: %v", err)
	}
	if verified.UID != testUID {
		t.Fatalf("UID = %q, want %q", verified.UID, testUID)
	}
}

func TestVerifySessionCookie_ReturnsErr(t *testing.T) {
	wantErr := errors.New("bad cookie")

	verified, err := authntest.Verifier{UID: testUID, Err: wantErr}.VerifySessionCookie(t.Context(), "any-cookie")
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
	if verified != nil {
		t.Fatalf("verified = %v, want nil", verified)
	}
}

func TestVerifySessionCookie_ReturnsErrRevoked(t *testing.T) {
	verified, err := authntest.Verifier{UID: testUID, Err: authntest.ErrRevoked}.VerifySessionCookie(t.Context(), "any-cookie")
	if !errors.Is(err, authntest.ErrRevoked) {
		t.Fatalf("err = %v, want %v", err, authntest.ErrRevoked)
	}
	if verified != nil {
		t.Fatalf("verified = %v, want nil", verified)
	}
}

func TestRevokeRefreshTokens_Succeeds(t *testing.T) {
	if err := (authntest.Verifier{UID: testUID}).RevokeRefreshTokens(t.Context(), testUID); err != nil {
		t.Fatalf("RevokeRefreshTokens: %v", err)
	}
}
