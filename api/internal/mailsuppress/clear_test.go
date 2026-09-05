package mailsuppress_test

import (
	"database/sql"
	"errors"
	"testing"

	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/mailsuppress"
	"doula-cloud/api/internal/testdb"
)

// beginTx hands back the request-scoped transaction Clear and List take:
// the low-privilege App connection, with app.current_practice_id set the
// way staffauth.Middleware sets it, because clients and
// practice_invitations are both RLS-scoped by it and read as empty
// without it.
func beginTx(t *testing.T, db *testdb.DB, practiceID string) *sql.Tx {
	t.Helper()
	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		// coverage:ignore reason: fixture failure, not exercised by the happy-path test
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	if _, err := tx.ExecContext(t.Context(),
		`SELECT set_config('app.current_practice_id', $1, true)`, practiceID,
	); err != nil {
		// coverage:ignore reason: fixture failure, not exercised by the happy-path test
		t.Fatalf("set practice: %v", err)
	}
	return tx
}

// Named because goconst counts the literals across the package's tests,
// not because the role vocabulary lives here -- ADR-0008 owns that.
const (
	roleOwner = "owner"
	roleAdmin = "admin"
	roleDoula = "doula"
)

func commit(t *testing.T, tx *sql.Tx) {
	t.Helper()
	if err := tx.Commit(); err != nil {
		// coverage:ignore reason: fixture failure, not exercised by the happy-path test
		t.Fatalf("commit: %v", err)
	}
}

// seedClientAt gives practiceID a Client with address, so the address
// counts as one that Practice is responsible for.
func seedClientAt(t *testing.T, db *testdb.DB, practiceID, address string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO clients (practice_id, given_name, email) VALUES ($1, 'Test', $2)`,
		practiceID, address,
	); err != nil {
		// coverage:ignore reason: fixture insert failure, not exercised by the happy-path test
		t.Fatalf("seed client: %v", err)
	}
}

func TestClear_LiftsABounceLocallyAndAtMailgun(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Clearing Practice")
	staffID := testdb.SeedStaffAtPractice(t, db, practiceID, "clearer", []string{roleOwner}, "employee")
	if err := mailsuppress.Record(t.Context(), db.App, testAddress, mailsuppress.CauseBounce, "evt-1"); err != nil {
		t.Fatalf("Record: %v", err)
	}
	clearer := &mail.FakeSender{}

	tx := beginTx(t, db, practiceID)
	if err := mailsuppress.Clear(t.Context(), tx, clearer, testAddress, staffID); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	commit(t, tx)

	// Mailgun keeps its own bounce list; a clear that skipped it would
	// leave the address refused server-side no matter what the local row
	// says.
	if got := clearer.Deleted(); len(got) != 1 || got[0] != testAddress {
		t.Fatalf("Mailgun deletes = %v, want [%s]", got, testAddress)
	}

	active, err := mailsuppress.Active(t.Context(), db.App, testAddress)
	if err != nil {
		t.Fatalf("Active: %v", err)
	}
	if active {
		t.Fatal("a cleared bounce still refuses the address")
	}

	var clearedBy string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT cleared_by FROM email_suppressions WHERE address = $1`, testAddress,
	).Scan(&clearedBy); err != nil {
		t.Fatalf("read cleared_by: %v", err)
	}
	if clearedBy != staffID {
		t.Fatalf("cleared_by = %q, want the acting Staff member %q", clearedBy, staffID)
	}
}

// ADR-0029: a complaint is never cleared, and the refusal belongs here
// rather than in whatever screen happens to be calling.
func TestClear_RefusesAComplaint(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Complaint Practice")
	staffID := testdb.SeedStaffAtPractice(t, db, practiceID, "clearer", []string{roleOwner}, "employee")
	if err := mailsuppress.Record(t.Context(), db.App, testAddress, mailsuppress.CauseComplaint, "evt-1"); err != nil {
		t.Fatalf("Record: %v", err)
	}
	clearer := &mail.FakeSender{}

	err := mailsuppress.Clear(t.Context(), beginTx(t, db, practiceID), clearer, testAddress, staffID)
	if !errors.Is(err, mailsuppress.ErrNotClearable) {
		t.Fatalf("Clear error = %v, want ErrNotClearable", err)
	}
	if len(clearer.Deleted()) != 0 {
		t.Fatal("a refused complaint still reached Mailgun")
	}
	active, err := mailsuppress.Active(t.Context(), db.App, testAddress)
	if err != nil {
		t.Fatalf("Active: %v", err)
	}
	if !active {
		t.Fatal("a refused clear un-suppressed the address anyway")
	}
}

func TestClear_UnsuppressedAddressIsNotFound(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Empty Practice")
	staffID := testdb.SeedStaffAtPractice(t, db, practiceID, "clearer", []string{roleOwner}, "employee")

	err := mailsuppress.Clear(t.Context(), beginTx(t, db, practiceID), &mail.FakeSender{}, testAddress, staffID)
	if !errors.Is(err, mailsuppress.ErrNotSuppressed) {
		t.Fatalf("Clear error = %v, want ErrNotSuppressed", err)
	}
}

// The vendor call runs first precisely so this case can exist: Mailgun
// refuses, and the local row is untouched rather than reporting an
// address as usable that Mailgun still blocks.
func TestClear_MailgunFailureLeavesTheSuppressionStanding(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Unreachable Practice")
	staffID := testdb.SeedStaffAtPractice(t, db, practiceID, "clearer", []string{roleOwner}, "employee")
	if err := mailsuppress.Record(t.Context(), db.App, testAddress, mailsuppress.CauseBounce, "evt-1"); err != nil {
		t.Fatalf("Record: %v", err)
	}

	err := mailsuppress.Clear(t.Context(), beginTx(t, db, practiceID), &mail.FakeSender{DeleteErr: errBoom}, testAddress, staffID)
	if !errors.Is(err, errBoom) {
		t.Fatalf("Clear error = %v, want errBoom", err)
	}
	active, err := mailsuppress.Active(t.Context(), db.App, testAddress)
	if err != nil {
		t.Fatalf("Active: %v", err)
	}
	if !active {
		t.Fatal("a failed Mailgun delete cleared the local row anyway")
	}
}

func TestList_CarriesOnlyThisPracticesAddresses(t *testing.T) {
	db := testdb.New(t)
	mine := testdb.SeedPractice(t, db, "Mine")
	theirs := testdb.SeedPractice(t, db, "Theirs")
	seedClientAt(t, db, mine, "client@example.test")
	seedClientAt(t, db, theirs, "stranger@example.test")
	const staffAddress = "roster-member@example.com"
	testdb.SeedStaffAtPractice(t, db, mine, "roster-member", []string{roleDoula}, "employee")

	for _, a := range []string{"client@example.test", "stranger@example.test", staffAddress} {
		if err := mailsuppress.Record(t.Context(), db.App, a, mailsuppress.CauseBounce, "evt"); err != nil {
			t.Fatalf("Record %s: %v", a, err)
		}
	}

	items, err := mailsuppress.List(t.Context(), beginTx(t, db, mine), mine)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	got := map[string]bool{}
	for _, it := range items {
		got[it.Address] = true
	}
	if !got["client@example.test"] {
		t.Fatal("this Practice's own Client is missing from its suppression list")
	}
	// A Staff member's own address is one the Practice is responsible
	// for: an Owner whose out-of-credits mail bounces must be able to see
	// it, which was invisible everywhere before #744.
	if !got[staffAddress] {
		t.Fatal("a roster member's suppressed address is missing from the list")
	}
	if got["stranger@example.test"] {
		t.Fatal("another Practice's Client leaked into this Practice's suppression list")
	}
}

// A cleared row is history, not a live block, so it drops off the list.
func TestList_OmitsAClearedSuppression(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Cleared")
	staffID := testdb.SeedStaffAtPractice(t, db, practiceID, "clearer", []string{roleOwner}, "employee")
	seedClientAt(t, db, practiceID, testAddress)
	if err := mailsuppress.Record(t.Context(), db.App, testAddress, mailsuppress.CauseBounce, "evt-1"); err != nil {
		t.Fatalf("Record: %v", err)
	}
	tx := beginTx(t, db, practiceID)
	if err := mailsuppress.Clear(t.Context(), tx, &mail.FakeSender{}, testAddress, staffID); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	commit(t, tx)

	items, err := mailsuppress.List(t.Context(), beginTx(t, db, practiceID), practiceID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("List returned %d rows after a clear, want 0", len(items))
	}
}

func TestAttachedToPractice(t *testing.T) {
	db := testdb.New(t)
	mine := testdb.SeedPractice(t, db, "Mine")
	theirs := testdb.SeedPractice(t, db, "Theirs")
	seedClientAt(t, db, mine, "client@example.test")
	seedClientAt(t, db, theirs, "stranger@example.test")
	inviter := testdb.SeedStaffAtPractice(t, db, mine, "inviter", []string{roleOwner}, "employee")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_invitations (practice_id, address, roles, employment_type, token_digest, invited_by, expires_at)
		 VALUES ($1, 'invitee@example.test', '{doula}'::practice_role[], 'employee', 'digest', $2, now() + interval '7 days')`,
		mine, inviter,
	); err != nil {
		t.Fatalf("seed invitation: %v", err)
	}

	for _, tc := range []struct {
		name    string
		address string
		want    bool
	}{
		{"this Practice's Client", "client@example.test", true},
		// An invited address is pre-account and has no Staff row yet, but
		// a bounced Staff invitation is exactly the case with nowhere
		// else to surface.
		{"an outstanding Invitation", "invitee@example.test", true},
		{"differently cased", "CLIENT@Example.Test", true},
		{"another Practice's Client", "stranger@example.test", false},
		{"nobody's address", "nobody@example.test", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := mailsuppress.AttachedToPractice(t.Context(), beginTx(t, db, mine), mine, tc.address)
			if err != nil {
				t.Fatalf("AttachedToPractice: %v", err)
			}
			if got != tc.want {
				t.Fatalf("AttachedToPractice(%q) = %v, want %v", tc.address, got, tc.want)
			}
		})
	}
}

var _ mailsuppress.BounceClearer = &mail.FakeSender{}
