package authntest_test

import (
	"errors"
	"testing"

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
