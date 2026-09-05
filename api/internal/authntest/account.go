package authntest

import (
	"context"
	"sync"

	"doula-cloud/api/internal/authn"
)

// account is one FakeAccountManager entry -- what a real Identity
// Platform record would hold that #613's flows ever touch, plus the
// last password SetPassword recorded, so a test can assert on it without
// this package growing a real credential store.
type account struct {
	email         string
	emailVerified bool
	password      string
	mfaEnrolled   bool
}

// FakeAccountManager is the test double for authn.AccountManager --
// real Identity Platform accounts cannot be created in a unit test, so
// this holds an in-memory set instead, mirroring mail.FakeSender's
// shape.
type FakeAccountManager struct {
	mu       sync.Mutex
	accounts map[string]*account // uid -> account

	// Err, when set, is returned by every method instead of doing
	// anything -- how a test simulates the Admin SDK being unreachable.
	Err error

	// SetEmailErr, when set, is returned by SetEmail specifically,
	// without disturbing GetAccount -- how a test proves
	// ChangeEmailHandler's own SetEmail failure branch, distinct from a
	// failure of the GetAccount call that precedes it.
	SetEmailErr error

	// ClearSecondFactorsErr, when set, is returned by ClearSecondFactors
	// specifically, without disturbing GetAccountByEmail -- how #615's
	// spend-handler tests prove clearEnrolmentAndRecord's own failure
	// branch, distinct from the account-lookup failure that precedes it.
	ClearSecondFactorsErr error
}

var _ authn.AccountManager = (*FakeAccountManager)(nil)

// NewFakeAccountManager returns an empty FakeAccountManager.
func NewFakeAccountManager() *FakeAccountManager {
	return &FakeAccountManager{accounts: map[string]*account{}}
}

// Seed adds an account for uid, as though Identity Platform already held
// it.
func (f *FakeAccountManager) Seed(uid, email string, emailVerified bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.accounts[uid] = &account{email: email, emailVerified: emailVerified}
}

// Password returns the password SetPassword last recorded for uid, so a
// test can assert a reset actually changed it.
func (f *FakeAccountManager) Password(uid string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if a, ok := f.accounts[uid]; ok {
		return a.password
	}
	return ""
}

// EnrollTOTP marks uid as holding a TOTP enrolment, as though she had
// completed Identity Platform's client-side enrolment flow -- #615's
// tests seed this so a recovery spend has something to clear.
func (f *FakeAccountManager) EnrollTOTP(uid string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if a, ok := f.accounts[uid]; ok {
		a.mfaEnrolled = true
	}
}

// HasSecondFactor reports whether uid currently holds an enrolled
// factor, so a test can assert ClearSecondFactors actually cleared it.
func (f *FakeAccountManager) HasSecondFactor(uid string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if a, ok := f.accounts[uid]; ok {
		return a.mfaEnrolled
	}
	return false
}

// GetAccount implements authn.AccountManager.
func (f *FakeAccountManager) GetAccount(_ context.Context, uid string) (authn.Account, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Err != nil {
		return authn.Account{}, f.Err
	}
	a, ok := f.accounts[uid]
	if !ok {
		return authn.Account{}, authn.ErrAccountNotFound
	}
	return authn.Account{Email: a.email, EmailVerified: a.emailVerified}, nil
}

// GetAccountByEmail implements authn.AccountManager.
func (f *FakeAccountManager) GetAccountByEmail(_ context.Context, email string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Err != nil {
		return "", f.Err
	}
	for uid, a := range f.accounts {
		if a.email == email {
			return uid, nil
		}
	}
	return "", authn.ErrAccountNotFound
}

// SetEmailVerified implements authn.AccountManager.
func (f *FakeAccountManager) SetEmailVerified(_ context.Context, uid string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Err != nil {
		return f.Err
	}
	a, ok := f.accounts[uid]
	if !ok {
		return authn.ErrAccountNotFound
	}
	a.emailVerified = true
	return nil
}

// SetPassword implements authn.AccountManager.
func (f *FakeAccountManager) SetPassword(_ context.Context, uid, password string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Err != nil {
		return f.Err
	}
	a, ok := f.accounts[uid]
	if !ok {
		return authn.ErrAccountNotFound
	}
	a.password = password
	return nil
}

// SetEmail implements authn.AccountManager.
func (f *FakeAccountManager) SetEmail(_ context.Context, uid, email string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.SetEmailErr != nil {
		return f.SetEmailErr
	}
	if f.Err != nil {
		return f.Err
	}
	a, ok := f.accounts[uid]
	if !ok {
		return authn.ErrAccountNotFound
	}
	a.email = email
	a.emailVerified = false
	return nil
}

// CountWithoutSecondFactor implements authn.AccountManager. A uid this
// fake holds no account for counts as "without a second factor", the
// same treatment FirebaseVerifier gives an Identity Platform NotFound.
func (f *FakeAccountManager) CountWithoutSecondFactor(_ context.Context, uids []string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Err != nil {
		return 0, f.Err
	}
	count := 0
	for _, uid := range uids {
		a, ok := f.accounts[uid]
		if !ok || !a.mfaEnrolled {
			count++
		}
	}
	return count, nil
}

// ClearSecondFactors implements authn.AccountManager.
func (f *FakeAccountManager) ClearSecondFactors(_ context.Context, uid string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.ClearSecondFactorsErr != nil {
		return f.ClearSecondFactorsErr
	}
	if f.Err != nil {
		return f.Err
	}
	a, ok := f.accounts[uid]
	if !ok {
		return authn.ErrAccountNotFound
	}
	a.mfaEnrolled = false
	return nil
}
