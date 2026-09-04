package authntest_test

import (
	"errors"
	"testing"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/authntest"
)

func TestFakeAccountManager_SeedThenGetAccount(t *testing.T) {
	f := authntest.NewFakeAccountManager()
	f.Seed("uid-1", "person@example.com", false)

	acc, err := f.GetAccount(t.Context(), "uid-1")
	if err != nil {
		t.Fatalf("GetAccount: %v", err)
	}
	if acc.Email != "person@example.com" || acc.EmailVerified {
		t.Fatalf("account = %+v, want unverified person@example.com", acc)
	}
}

func TestFakeAccountManager_GetAccount_UnknownUIDNotFound(t *testing.T) {
	f := authntest.NewFakeAccountManager()
	if _, err := f.GetAccount(t.Context(), "nobody"); !errors.Is(err, authn.ErrAccountNotFound) {
		t.Fatalf("err = %v, want ErrAccountNotFound", err)
	}
}

func TestFakeAccountManager_GetAccountByEmail(t *testing.T) {
	f := authntest.NewFakeAccountManager()
	f.Seed("uid-2", "match@example.com", false)

	uid, err := f.GetAccountByEmail(t.Context(), "match@example.com")
	if err != nil {
		t.Fatalf("GetAccountByEmail: %v", err)
	}
	if uid != "uid-2" {
		t.Fatalf("uid = %q, want uid-2", uid)
	}
}

func TestFakeAccountManager_GetAccountByEmail_UnknownAddressNotFound(t *testing.T) {
	f := authntest.NewFakeAccountManager()
	if _, err := f.GetAccountByEmail(t.Context(), "nobody@example.com"); !errors.Is(err, authn.ErrAccountNotFound) {
		t.Fatalf("err = %v, want ErrAccountNotFound", err)
	}
}

func TestFakeAccountManager_SetEmailVerified(t *testing.T) {
	f := authntest.NewFakeAccountManager()
	f.Seed("uid-3", "person@example.com", false)

	if err := f.SetEmailVerified(t.Context(), "uid-3"); err != nil {
		t.Fatalf("SetEmailVerified: %v", err)
	}
	acc, _ := f.GetAccount(t.Context(), "uid-3")
	if !acc.EmailVerified {
		t.Fatal("EmailVerified = false, want true")
	}
}

func TestFakeAccountManager_SetEmailVerified_UnknownUIDNotFound(t *testing.T) {
	f := authntest.NewFakeAccountManager()
	if err := f.SetEmailVerified(t.Context(), "nobody"); !errors.Is(err, authn.ErrAccountNotFound) {
		t.Fatalf("err = %v, want ErrAccountNotFound", err)
	}
}

func TestFakeAccountManager_SetPassword(t *testing.T) {
	f := authntest.NewFakeAccountManager()
	f.Seed("uid-4", "person@example.com", false)

	if err := f.SetPassword(t.Context(), "uid-4", "new-password"); err != nil {
		t.Fatalf("SetPassword: %v", err)
	}
	if got := f.Password("uid-4"); got != "new-password" {
		t.Fatalf("Password = %q, want new-password", got)
	}
}

func TestFakeAccountManager_SetPassword_UnknownUIDNotFound(t *testing.T) {
	f := authntest.NewFakeAccountManager()
	if err := f.SetPassword(t.Context(), "nobody", "x"); !errors.Is(err, authn.ErrAccountNotFound) {
		t.Fatalf("err = %v, want ErrAccountNotFound", err)
	}
}

func TestFakeAccountManager_Password_UnknownUIDIsEmpty(t *testing.T) {
	f := authntest.NewFakeAccountManager()
	if got := f.Password("nobody"); got != "" {
		t.Fatalf("Password = %q, want empty", got)
	}
}

func TestFakeAccountManager_SetEmail_ClearsVerified(t *testing.T) {
	f := authntest.NewFakeAccountManager()
	f.Seed("uid-5", "old@example.com", true)

	if err := f.SetEmail(t.Context(), "uid-5", "new@example.com"); err != nil {
		t.Fatalf("SetEmail: %v", err)
	}
	acc, _ := f.GetAccount(t.Context(), "uid-5")
	if acc.Email != "new@example.com" {
		t.Fatalf("Email = %q, want new@example.com", acc.Email)
	}
	if acc.EmailVerified {
		t.Fatal("EmailVerified = true, want false after an address change")
	}
}

// TestFakeAccountManager_SetEmailErr_TakesPrecedenceOverErr proves
// SetEmailErr, when set, is what SetEmail returns -- without disturbing
// any other method, unlike the generic Err field.
func TestFakeAccountManager_SetEmailErr_TakesPrecedenceOverErr(t *testing.T) {
	f := authntest.NewFakeAccountManager()
	f.Seed("uid-7", "old@example.com", true)
	wantErr := errors.New("admin sdk rejected the write")
	f.SetEmailErr = wantErr

	if err := f.SetEmail(t.Context(), "uid-7", "new@example.com"); !errors.Is(err, wantErr) {
		t.Fatalf("SetEmail err = %v, want %v", err, wantErr)
	}
	if _, err := f.GetAccount(t.Context(), "uid-7"); err != nil {
		t.Fatalf("GetAccount err = %v, want nil -- SetEmailErr must not disturb other methods", err)
	}
}

func TestFakeAccountManager_SetEmail_UnknownUIDNotFound(t *testing.T) {
	f := authntest.NewFakeAccountManager()
	if err := f.SetEmail(t.Context(), "nobody", "x@example.com"); !errors.Is(err, authn.ErrAccountNotFound) {
		t.Fatalf("err = %v, want ErrAccountNotFound", err)
	}
}

// TestFakeAccountManager_ErrShortCircuitsEveryMethod proves f.Err, once
// set, is what every method returns instead of touching its seeded
// state -- how a test simulates the Admin SDK being unreachable.
func TestFakeAccountManager_ErrShortCircuitsEveryMethod(t *testing.T) {
	f := authntest.NewFakeAccountManager()
	f.Seed("uid-6", "person@example.com", false)
	wantErr := errors.New("admin sdk unreachable")
	f.Err = wantErr

	if _, err := f.GetAccount(t.Context(), "uid-6"); !errors.Is(err, wantErr) {
		t.Fatalf("GetAccount err = %v, want %v", err, wantErr)
	}
	if _, err := f.GetAccountByEmail(t.Context(), "person@example.com"); !errors.Is(err, wantErr) {
		t.Fatalf("GetAccountByEmail err = %v, want %v", err, wantErr)
	}
	if err := f.SetEmailVerified(t.Context(), "uid-6"); !errors.Is(err, wantErr) {
		t.Fatalf("SetEmailVerified err = %v, want %v", err, wantErr)
	}
	if err := f.SetPassword(t.Context(), "uid-6", "x"); !errors.Is(err, wantErr) {
		t.Fatalf("SetPassword err = %v, want %v", err, wantErr)
	}
	if err := f.SetEmail(t.Context(), "uid-6", "x@example.com"); !errors.Is(err, wantErr) {
		t.Fatalf("SetEmail err = %v, want %v", err, wantErr)
	}
	if err := f.ClearSecondFactors(t.Context(), "uid-6"); !errors.Is(err, wantErr) {
		t.Fatalf("ClearSecondFactors err = %v, want %v", err, wantErr)
	}
}

func TestFakeAccountManager_EnrollTOTPThenClearSecondFactors(t *testing.T) {
	f := authntest.NewFakeAccountManager()
	f.Seed("uid-8", "person@example.com", true)

	if f.HasSecondFactor("uid-8") {
		t.Fatal("HasSecondFactor = true before EnrollTOTP, want false")
	}
	f.EnrollTOTP("uid-8")
	if !f.HasSecondFactor("uid-8") {
		t.Fatal("HasSecondFactor = false after EnrollTOTP, want true")
	}

	if err := f.ClearSecondFactors(t.Context(), "uid-8"); err != nil {
		t.Fatalf("ClearSecondFactors: %v", err)
	}
	if f.HasSecondFactor("uid-8") {
		t.Fatal("HasSecondFactor = true after ClearSecondFactors, want false")
	}
}

func TestFakeAccountManager_EnrollTOTPAndHasSecondFactor_UnknownUIDNoop(t *testing.T) {
	f := authntest.NewFakeAccountManager()
	f.EnrollTOTP("nobody") // must not panic on a uid with no seeded account
	if f.HasSecondFactor("nobody") {
		t.Fatal("HasSecondFactor = true for an unseeded uid, want false")
	}
}

func TestFakeAccountManager_ClearSecondFactors_UnknownUIDNotFound(t *testing.T) {
	f := authntest.NewFakeAccountManager()
	if err := f.ClearSecondFactors(t.Context(), "nobody"); !errors.Is(err, authn.ErrAccountNotFound) {
		t.Fatalf("err = %v, want ErrAccountNotFound", err)
	}
}

// TestFakeAccountManager_ClearSecondFactorsErr_TakesPrecedenceOverErr
// mirrors TestFakeAccountManager_SetEmailErr_TakesPrecedenceOverErr: an
// isolated failure of ClearSecondFactors specifically, without disturbing
// GetAccountByEmail -- what #615's spend-handler tests need to prove
// clearEnrolmentAndRecord's own failure branch.
func TestFakeAccountManager_ClearSecondFactorsErr_TakesPrecedenceOverErr(t *testing.T) {
	f := authntest.NewFakeAccountManager()
	f.Seed("uid-9", "person@example.com", true)
	wantErr := errors.New("admin sdk rejected the write")
	f.ClearSecondFactorsErr = wantErr

	if err := f.ClearSecondFactors(t.Context(), "uid-9"); !errors.Is(err, wantErr) {
		t.Fatalf("ClearSecondFactors err = %v, want %v", err, wantErr)
	}
	if _, err := f.GetAccountByEmail(t.Context(), "person@example.com"); err != nil {
		t.Fatalf("GetAccountByEmail err = %v, want nil -- ClearSecondFactorsErr must not disturb other methods", err)
	}
}

// TestFakeAccountManager_DeleteAccount covers #394's account deletion:
// the account goes, a uid the fake never knew is success (matching
// FirebaseVerifier, which swallows Identity Platform's own
// user-not-found so a retried erasure is a no-op), and Exists reports
// what happened.
func TestFakeAccountManager_DeleteAccount(t *testing.T) {
	f := authntest.NewFakeAccountManager()
	f.Seed("uid-10", "erased@example.com", true)

	if !f.Exists("uid-10") {
		t.Fatal("Exists(seeded uid) = false, want true")
	}
	if err := f.DeleteAccount(t.Context(), "uid-10"); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}
	if f.Exists("uid-10") {
		t.Fatal("Exists after DeleteAccount = true, want false")
	}
	if err := f.DeleteAccount(t.Context(), "nobody"); err != nil {
		t.Fatalf("DeleteAccount(unknown uid) = %v, want nil -- a retried erasure must not fail on work already done", err)
	}
}

// TestFakeAccountManager_DeleteAccountErr_TakesPrecedenceOverErr mirrors
// the SetEmailErr and ClearSecondFactorsErr cases: an isolated failure
// of DeleteAccount, for #394's erasure-outbox retry branch.
func TestFakeAccountManager_DeleteAccountErr_TakesPrecedenceOverErr(t *testing.T) {
	f := authntest.NewFakeAccountManager()
	f.Seed("uid-11", "still-here@example.com", true)
	wantErr := errors.New("identity platform is down")
	f.DeleteAccountErr = wantErr

	if err := f.DeleteAccount(t.Context(), "uid-11"); !errors.Is(err, wantErr) {
		t.Fatalf("DeleteAccount err = %v, want %v", err, wantErr)
	}
	if !f.Exists("uid-11") {
		t.Fatal("the account was deleted despite the injected failure")
	}
	if _, err := f.GetAccountByEmail(t.Context(), "still-here@example.com"); err != nil {
		t.Fatalf("GetAccountByEmail err = %v, want nil -- DeleteAccountErr must not disturb other methods", err)
	}
}

// TestFakeAccountManager_DeleteAccount_HonorsErr proves the shared Err
// switch reaches DeleteAccount too.
func TestFakeAccountManager_DeleteAccount_HonorsErr(t *testing.T) {
	f := authntest.NewFakeAccountManager()
	f.Seed("uid-12", "unreachable@example.com", true)
	wantErr := errors.New("admin sdk unreachable")
	f.Err = wantErr

	if err := f.DeleteAccount(t.Context(), "uid-12"); !errors.Is(err, wantErr) {
		t.Fatalf("DeleteAccount err = %v, want %v", err, wantErr)
	}
}
