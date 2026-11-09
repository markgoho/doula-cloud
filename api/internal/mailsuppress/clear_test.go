package mailsuppress_test

import (
	"errors"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/mailsuppress"
	"doula-cloud/api/internal/testdb"
)

// clearingStaff is a Practice with an Owner who clears suppressions.
func clearingStaff(t *testing.T) (*testdb.DB, fixture.PracticeRecord, fixture.StaffRecord) {
	t.Helper()
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	return db, p, fixture.Staff(t, db, p, fixture.Owner())
}

func TestClear_LiftsABounceLocallyAndAtMailgun(t *testing.T) {
	db, p, staff := clearingStaff(t)
	suppress(t, db, testAddress, mailsuppress.CauseBounce)
	clearer := &mail.FakeSender{}

	// The transaction carries app.current_practice_id the way
	// staffauth.Middleware sets it, because clients and
	// practice_invitations are both RLS-scoped by it.
	tx := db.BeginAs(t, testdb.Practice(p.ID))
	if err := mailsuppress.Clear(t.Context(), tx, clearer, testAddress, staff.ID); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// Mailgun keeps its own bounce list; a clear that skipped it would
	// leave the address refused server-side no matter what the local row
	// says.
	if got := clearer.Deleted(); len(got) != 1 || got[0] != testAddress {
		t.Fatalf("Mailgun deletes = %v, want [%s]", got, testAddress)
	}
	requireActive(t, db, testAddress, false)
	if clearedBy := testdb.Scalar[string](t, db.Admin, `SELECT cleared_by FROM email_suppressions WHERE address = $1`, testAddress); clearedBy != staff.ID {
		t.Fatalf("cleared_by = %q, want the acting Staff member %q", clearedBy, staff.ID)
	}
}

// TestClear_Refusals table-drives the errors Clear gives and that each
// leaves the suppression as it was and Mailgun untouched. ADR-0029: a
// complaint is never cleared, and the refusal belongs here rather than in
// whatever screen happens to be calling. The vendor call runs first
// precisely so the Mailgun case can exist: Mailgun refuses, and the local
// row is untouched rather than reporting an address as usable that
// Mailgun still blocks.
func TestClear_Refusals(t *testing.T) {
	cases := []struct {
		name       string
		cause      string
		deleteErr  error
		wantErr    error
		wantActive bool
	}{
		{"a complaint", mailsuppress.CauseComplaint, nil, mailsuppress.ErrNotClearable, true},
		{"an unsuppressed address", "", nil, mailsuppress.ErrNotSuppressed, false},
		{"a Mailgun failure", mailsuppress.CauseBounce, errBoom, errBoom, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, p, staff := clearingStaff(t)
			if tc.cause != "" {
				suppress(t, db, testAddress, tc.cause)
			}
			clearer := &mail.FakeSender{DeleteErr: tc.deleteErr}

			err := mailsuppress.Clear(t.Context(), db.BeginAs(t, testdb.Practice(p.ID)), clearer, testAddress, staff.ID)

			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Clear error = %v, want %v", err, tc.wantErr)
			}
			if len(clearer.Deleted()) != 0 {
				t.Fatal("a refused clear still reached Mailgun")
			}
			requireActive(t, db, testAddress, tc.wantActive)
		})
	}
}

func TestList_CarriesOnlyThisPracticesAddresses(t *testing.T) {
	db := testdb.New(t)
	mine := minePractice(t, db)
	rosterMember := fixture.Staff(t, db, mine, fixture.StaffUID("roster-member"))

	for _, a := range []string{"client@example.test", "stranger@example.test", rosterMember.UID + "@example.com"} {
		suppress(t, db, a, mailsuppress.CauseBounce)
	}

	items, err := mailsuppress.List(t.Context(), db.BeginAs(t, testdb.Practice(mine.ID)), mine.ID)
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
	if !got[rosterMember.UID+"@example.com"] {
		t.Fatal("a roster member's suppressed address is missing from the list")
	}
	if got["stranger@example.test"] {
		t.Fatal("another Practice's Client leaked into this Practice's suppression list")
	}
}

// A cleared row is history, not a live block, so it drops off the list.
func TestList_OmitsAClearedSuppression(t *testing.T) {
	db, p, staff := clearingStaff(t)
	clientAt(t, db, p, testAddress)
	suppress(t, db, testAddress, mailsuppress.CauseBounce)
	tx := db.BeginAs(t, testdb.Practice(p.ID))
	if err := mailsuppress.Clear(t.Context(), tx, &mail.FakeSender{}, testAddress, staff.ID); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	items, err := mailsuppress.List(t.Context(), db.BeginAs(t, testdb.Practice(p.ID)), p.ID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("List returned %d rows after a clear, want 0", len(items))
	}
}

func TestAttachedToPractice(t *testing.T) {
	db := testdb.New(t)
	mine := minePractice(t, db)
	inviter := fixture.Staff(t, db, mine, fixture.Owner())
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_invitations (practice_id, address, roles, employment_type, token_digest, invited_by, expires_at)
		 VALUES ($1, 'invitee@example.test', '{doula}'::practice_role[], 'employee', 'digest', $2, now() + interval '7 days')`,
		mine.ID, inviter.ID,
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
			got, err := mailsuppress.AttachedToPractice(t.Context(), db.BeginAs(t, testdb.Practice(mine.ID)), mine.ID, tc.address)
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
