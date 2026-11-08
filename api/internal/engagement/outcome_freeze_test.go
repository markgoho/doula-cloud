package engagement_test

import (
	"strings"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/testdb"
)

// seedFrozenEngagement returns an Engagement whose birth outcome is
// already recorded, written through the Admin connection so the freeze
// is proved against the database itself rather than against the handler
// that normally writes it.
func seedFrozenEngagement(t *testing.T, db *testdb.DB) fixture.EngagementRecord {
	t.Helper()
	e := fixture.Engagement(t, db, fixture.Practice(t, db), fixture.Status(fixture.StatusActive))
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE engagements SET birth_outcome = 'loss', pregnancy_ended_on = $2::date WHERE id = $1`,
		e.ID, lossOn); err != nil {
		t.Fatalf("seed frozen outcome: %v", err)
	}
	return e
}

// TestFreezeTrigger_RefusesAnOverwriteWithoutTheDoor proves ADR-0015's
// immutability lives in the database, not only in the handler: a plain
// UPDATE of either frozen column raises, whoever runs it -- the Admin
// connection here is the superuser the migrations ran as, which RLS
// would not have stopped at all.
func TestFreezeTrigger_RefusesAnOverwriteWithoutTheDoor(t *testing.T) {
	writes := []struct {
		name string
		sql  string
	}{
		{"the outcome", `UPDATE engagements SET birth_outcome = 'live_birth' WHERE id = $1`},
		{"the date", `UPDATE engagements SET pregnancy_ended_on = '2027-03-05'::date WHERE id = $1`},
		{"back to null", `UPDATE engagements SET birth_outcome = NULL, pregnancy_ended_on = NULL WHERE id = $1`},
	}
	for _, tc := range writes {
		t.Run(tc.name, func(t *testing.T) {
			db := testdb.New(t)
			e := seedFrozenEngagement(t, db)

			_, err := db.Admin.ExecContext(t.Context(), tc.sql, e.ID)
			if err == nil {
				t.Fatal("update succeeded, want the freeze trigger to refuse it")
			}
			if !strings.Contains(err.Error(), "frozen") {
				t.Fatalf("error = %v, want the freeze trigger's own refusal", err)
			}
		})
	}
}

// TestFreezeTrigger_OpensForTheCorrectionDoor proves the one narrow
// escape ADR-0015 names: with app.allow_outcome_correction set for the
// transaction, the same overwrite goes through. This is the idiom
// app.invite_token (00026) already uses -- one writer sets it, one
// trigger reads it.
func TestFreezeTrigger_OpensForTheCorrectionDoor(t *testing.T) {
	db := testdb.New(t)
	e := seedFrozenEngagement(t, db)

	tx, err := db.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(t.Context(),
		`SELECT set_config('app.allow_outcome_correction', 'on', true)`); err != nil {
		t.Fatalf("open the door: %v", err)
	}
	if _, err := tx.ExecContext(t.Context(),
		`UPDATE engagements SET birth_outcome = 'live_birth' WHERE id = $1`, e.ID); err != nil {
		t.Fatalf("corrected update: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

// TestFreezeTrigger_LeavesEveryOtherColumnAlone is the regression this
// trigger most needed: TransitionHandler updates status, ending_reason
// and ending_note on rows whose outcome is already frozen, and a trigger
// written to fire on any UPDATE would break every completion.
func TestFreezeTrigger_LeavesEveryOtherColumnAlone(t *testing.T) {
	db := testdb.New(t)
	e := seedFrozenEngagement(t, db)

	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE engagements SET status = 'completed', ending_reason = 'care_complete', kind = 'postpartum'
		  WHERE id = $1`, e.ID); err != nil {
		t.Fatalf("update of unfrozen columns: %v", err)
	}
}

// TestFreezeTrigger_RefusesARepointedClient covers the other half of
// ADR-0015's freeze rule: re-pointing the row is how a second baby would
// be served on a Credit already spent.
//
// The rule was narrowed on #813 (ADR-0040), never dropped. A merge may
// move an Engagement, and only a merge -- the trigger admits the change
// solely where the old Client is already tombstoned into the new one.
// Neither Client here has been merged into anything, so the refusal
// stands, which is what this test is for.
func TestFreezeTrigger_RefusesARepointedClient(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	e := fixture.Engagement(t, db, p, fixture.Status(fixture.StatusActive))
	other := fixture.Engagement(t, db, p, fixture.Status(fixture.StatusActive))

	_, err := db.Admin.ExecContext(t.Context(),
		`UPDATE engagements SET client_id = (SELECT client_id FROM engagements WHERE id = $2) WHERE id = $1`,
		e.ID, other.ID)
	if err == nil {
		t.Fatal("update succeeded, want client_id to be immutable")
	}
	if !strings.Contains(err.Error(), "moves between Clients only when the record it belongs to has been merged") {
		t.Fatalf("error = %v, want the trigger's own refusal", err)
	}
}

// TestOutcomeIsDatedConstraint is ADR-0015's engagements_outcome_is_dated,
// deliberately not biconditional: a null outcome forbids a date, a dated
// outcome requires one, and 'unknown' -- the honest value for an
// Engagement whose birth happened out of sight -- may carry either.
func TestOutcomeIsDatedConstraint(t *testing.T) {
	cases := []struct {
		name       string
		sql        string
		wantRefuse bool
	}{
		{"a loss with no date",
			`UPDATE engagements SET birth_outcome = 'loss' WHERE id = $1`, true},
		{"a date with no outcome",
			`UPDATE engagements SET pregnancy_ended_on = '2027-03-04'::date WHERE id = $1`, true},
		{"unknown with no date",
			`UPDATE engagements SET birth_outcome = 'unknown' WHERE id = $1`, false},
		{"unknown with a date",
			`UPDATE engagements SET birth_outcome = 'unknown', pregnancy_ended_on = '2027-03-04'::date WHERE id = $1`, false},
		{"a loss with a date",
			`UPDATE engagements SET birth_outcome = 'loss', pregnancy_ended_on = '2027-03-04'::date WHERE id = $1`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testdb.New(t)
			e := fixture.Engagement(t, db, fixture.Practice(t, db), fixture.Status(fixture.StatusActive))

			_, err := db.Admin.ExecContext(t.Context(), tc.sql, e.ID)
			if tc.wantRefuse {
				if err == nil {
					t.Fatal("update succeeded, want engagements_outcome_is_dated to refuse it")
				}
				if !strings.Contains(err.Error(), "engagements_outcome_is_dated") {
					t.Fatalf("error = %v, want the constraint's own refusal", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("update refused: %v", err)
			}
		})
	}
}
