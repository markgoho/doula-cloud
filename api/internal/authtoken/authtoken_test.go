package authtoken_test

import (
	"errors"
	"testing"
	"time"

	"doula-cloud/api/internal/authtoken"
	"doula-cloud/api/internal/testdb"
)

func countTokens(t *testing.T, db *testdb.DB, identityUID string, purpose authtoken.Purpose) int {
	t.Helper()
	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM auth_tokens WHERE identity_uid = $1 AND purpose = $2`, identityUID, purpose,
	).Scan(&count); err != nil {
		t.Fatalf("count tokens: %v", err)
	}
	return count
}

func TestMint_ThenSpend_ResolvesTheMintedIdentity(t *testing.T) {
	db := testdb.New(t)
	now := time.Now()

	token, err := authtoken.Mint(t.Context(), db.App, "uid-1", authtoken.PurposeStaffEmailVerification, 24*time.Hour, now)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}

	uid, err := authtoken.Spend(t.Context(), db.App, token, authtoken.PurposeStaffEmailVerification, now)
	if err != nil {
		t.Fatalf("Spend: %v", err)
	}
	if uid != "uid-1" {
		t.Fatalf("uid = %q, want uid-1", uid)
	}
}

func TestSpend_IsSingleUse(t *testing.T) {
	db := testdb.New(t)
	now := time.Now()

	token, err := authtoken.Mint(t.Context(), db.App, "uid-2", authtoken.PurposeStaffPasswordReset, time.Hour, now)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if _, err := authtoken.Spend(t.Context(), db.App, token, authtoken.PurposeStaffPasswordReset, now); err != nil {
		t.Fatalf("first Spend: %v", err)
	}

	if _, err := authtoken.Spend(t.Context(), db.App, token, authtoken.PurposeStaffPasswordReset, now); !errors.Is(err, authtoken.ErrInvalid) {
		t.Fatalf("second Spend err = %v, want ErrInvalid", err)
	}
}

func TestSpend_RejectsAnExpiredToken(t *testing.T) {
	db := testdb.New(t)
	now := time.Now()

	token, err := authtoken.Mint(t.Context(), db.App, "uid-3", authtoken.PurposeStaffEmailVerification, time.Hour, now)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}

	past := now.Add(2 * time.Hour)
	if _, err := authtoken.Spend(t.Context(), db.App, token, authtoken.PurposeStaffEmailVerification, past); !errors.Is(err, authtoken.ErrInvalid) {
		t.Fatalf("Spend err = %v, want ErrInvalid", err)
	}
}

func TestSpend_RejectsTheWrongPurpose(t *testing.T) {
	db := testdb.New(t)
	now := time.Now()

	token, err := authtoken.Mint(t.Context(), db.App, "uid-4", authtoken.PurposeStaffEmailVerification, time.Hour, now)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}

	if _, err := authtoken.Spend(t.Context(), db.App, token, authtoken.PurposeStaffPasswordReset, now); !errors.Is(err, authtoken.ErrInvalid) {
		t.Fatalf("Spend err = %v, want ErrInvalid", err)
	}
}

func TestSpend_RejectsAnUnknownToken(t *testing.T) {
	db := testdb.New(t)
	if _, err := authtoken.Spend(t.Context(), db.App, "never-minted", authtoken.PurposeStaffPasswordReset, time.Now()); !errors.Is(err, authtoken.ErrInvalid) {
		t.Fatalf("Spend err = %v, want ErrInvalid", err)
	}
}

// TestMint_ReMintingInvalidatesThePriorToken proves the re-request AC:
// a second Mint for the same identity+purpose kills the first token
// outright, and leaves exactly one live row behind.
func TestMint_ReMintingInvalidatesThePriorToken(t *testing.T) {
	db := testdb.New(t)
	now := time.Now()

	first, err := authtoken.Mint(t.Context(), db.App, "uid-5", authtoken.PurposeStaffEmailVerification, 24*time.Hour, now)
	if err != nil {
		t.Fatalf("first Mint: %v", err)
	}
	second, err := authtoken.Mint(t.Context(), db.App, "uid-5", authtoken.PurposeStaffEmailVerification, 24*time.Hour, now)
	if err != nil {
		t.Fatalf("second Mint: %v", err)
	}

	if _, err := authtoken.Spend(t.Context(), db.App, first, authtoken.PurposeStaffEmailVerification, now); !errors.Is(err, authtoken.ErrInvalid) {
		t.Fatalf("spending the superseded token err = %v, want ErrInvalid", err)
	}
	if _, err := authtoken.Spend(t.Context(), db.App, second, authtoken.PurposeStaffEmailVerification, now); err != nil {
		t.Fatalf("spending the fresh token: %v", err)
	}
	if count := countTokens(t, db, "uid-5", authtoken.PurposeStaffEmailVerification); count != 1 {
		t.Fatalf("token rows = %d, want 1", count)
	}
}

// TestMint_DoesNotDisturbAnotherPurposesToken proves the shared table's
// per-purpose isolation: re-minting one purpose leaves a different
// purpose's live token for the same identity untouched.
func TestMint_DoesNotDisturbAnotherPurposesToken(t *testing.T) {
	db := testdb.New(t)
	now := time.Now()

	resetToken, err := authtoken.Mint(t.Context(), db.App, "uid-6", authtoken.PurposeStaffPasswordReset, time.Hour, now)
	if err != nil {
		t.Fatalf("Mint reset: %v", err)
	}
	if _, err := authtoken.Mint(t.Context(), db.App, "uid-6", authtoken.PurposeStaffEmailVerification, 24*time.Hour, now); err != nil {
		t.Fatalf("Mint verification: %v", err)
	}

	if _, err := authtoken.Spend(t.Context(), db.App, resetToken, authtoken.PurposeStaffPasswordReset, now); err != nil {
		t.Fatalf("Spend reset token: %v", err)
	}
}

// TestSpend_RollsBackWithItsTransaction proves a caller that spends a
// token on its own request-scoped transaction and then rolls back never
// loses the token -- it stays spendable, same as sessions' MintSession
// riding a caller's transaction.
func TestSpend_RollsBackWithItsTransaction(t *testing.T) {
	db := testdb.New(t)
	now := time.Now()

	token, err := authtoken.Mint(t.Context(), db.App, "uid-7", authtoken.PurposeStaffPasswordReset, time.Hour, now)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if _, err := authtoken.Spend(t.Context(), tx, token, authtoken.PurposeStaffPasswordReset, now); err != nil {
		t.Fatalf("Spend: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	if _, err := authtoken.Spend(t.Context(), db.App, token, authtoken.PurposeStaffPasswordReset, now); err != nil {
		t.Fatalf("Spend after rollback: %v", err)
	}
}

func TestMintCode_ThenSpend_ResolvesTheMintedIdentity(t *testing.T) {
	db := testdb.New(t)
	now := time.Now()

	code, err := authtoken.MintCode(t.Context(), db.App, "uid-8", authtoken.PurposeStaffMFARecovery, 24*time.Hour, now)
	if err != nil {
		t.Fatalf("MintCode: %v", err)
	}
	if len(code) != 8 {
		t.Fatalf("code = %q, want 8 digits", code)
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			t.Fatalf("code = %q, want all-decimal", code)
		}
	}

	uid, err := authtoken.Spend(t.Context(), db.App, code, authtoken.PurposeStaffMFARecovery, now)
	if err != nil {
		t.Fatalf("Spend: %v", err)
	}
	if uid != "uid-8" {
		t.Fatalf("uid = %q, want uid-8", uid)
	}
}

// TestMintCode_ReMintingInvalidatesThePriorCode mirrors Mint's own
// re-request rule: a second MintCode for the same identity+purpose kills
// the first code outright.
func TestMintCode_ReMintingInvalidatesThePriorCode(t *testing.T) {
	db := testdb.New(t)
	now := time.Now()

	first, err := authtoken.MintCode(t.Context(), db.App, "uid-9", authtoken.PurposeStaffMFARecovery, 24*time.Hour, now)
	if err != nil {
		t.Fatalf("first MintCode: %v", err)
	}
	second, err := authtoken.MintCode(t.Context(), db.App, "uid-9", authtoken.PurposeStaffMFARecovery, 24*time.Hour, now)
	if err != nil {
		t.Fatalf("second MintCode: %v", err)
	}

	if _, err := authtoken.Spend(t.Context(), db.App, first, authtoken.PurposeStaffMFARecovery, now); !errors.Is(err, authtoken.ErrInvalid) {
		t.Fatalf("spending the superseded code err = %v, want ErrInvalid", err)
	}
	if _, err := authtoken.Spend(t.Context(), db.App, second, authtoken.PurposeStaffMFARecovery, now); err != nil {
		t.Fatalf("spending the fresh code: %v", err)
	}
}

// TestDigest_MatchesWhatSpendLooksUp proves Digest is the same digest
// Mint/Spend key auth_tokens.token_hash on -- staff_mfa_recovery_vouches
// (00062) has to compute this independently to look a mint back up by
// its token_hash, and a mismatched algorithm would silently never match.
func TestDigest_MatchesWhatSpendLooksUp(t *testing.T) {
	db := testdb.New(t)
	now := time.Now()

	token, err := authtoken.Mint(t.Context(), db.App, "uid-10", authtoken.PurposeStaffMFARecovery, time.Hour, now)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}

	var storedHash string
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT token_hash FROM auth_tokens WHERE identity_uid = $1`, "uid-10").Scan(&storedHash); err != nil {
		t.Fatalf("query token_hash: %v", err)
	}
	if got := authtoken.Digest(token); got != storedHash {
		t.Fatalf("Digest(token) = %q, want %q (the stored token_hash)", got, storedHash)
	}
}
