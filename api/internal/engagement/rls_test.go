package engagement_test

import (
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/testdb"
)

// These tests exercise the RLS policies from
// 00005_client_engagement.sql directly via db.App and set_config,
// bypassing the Go handlers -- proving the SQL policies themselves scope
// visibility correctly, not just that the handlers happen to agree with
// them.

func TestRLS_ClientsFailsClosedWithNoPracticeSet(t *testing.T) {
	db := testdb.New(t)
	fixture.Engagement(t, db, fixture.Practice(t, db))

	if count := testdb.Scalar[int](t, db.App, `SELECT count(*) FROM clients`); count != 0 {
		t.Fatalf("expected 0 rows with no session variables set, got %d", count)
	}
}

// TestRLS_ClientsSelectIsScopedToPracticeID proves the clients_select
// policy narrows to Clients whose own practice_id matches
// app.current_practice_id (00042's plain-column-comparison collapse of
// the old EXISTS-through-engagements shape), not to every Client row
// globally.
func TestRLS_ClientsSelectIsScopedToPracticeID(t *testing.T) {
	db := testdb.New(t)
	practiceA, practiceB := fixture.Practice(t, db), fixture.Practice(t, db)
	atA := fixture.Engagement(t, db, practiceA)
	fixture.Engagement(t, db, practiceB)

	tx := db.BeginAs(t, testdb.Practice(practiceA.ID))

	if visible := testdb.Column[string](t, tx, `SELECT id FROM clients`); len(visible) != 1 || visible[0] != atA.Client.ID {
		t.Fatalf("visible clients = %v, want only %q", visible, atA.Client.ID)
	}
}

// TestRLS_ClientsInsert walks clients_insert's WITH CHECK. It fails closed
// with no app.current_practice_id set: a real Practice is referenced, so
// the only thing distinguishing that row from the allowed one is the
// missing session var -- not a NOT NULL violation instead. It permits an
// INSERT once a Practice context is set and the caller's own Membership
// there is an employee's, even though no Engagement referencing the new
// Client exists yet -- the chicken-and-egg case CreateHandler relies on.
// A contractor Doula's Membership fails the same insert an employee's
// passes, even with both session vars set (ADR-0017: "a contractor
// originates nothing"); and the refusal is role-gated, not bare
// employment_type, so an Owner who also does the work under a contractor
// employment type (ADR-0017's "solo Practice") may still create one.
func TestRLS_ClientsInsert(t *testing.T) {
	cases := []struct {
		name      string
		staff     []fixture.StaffOption
		sessionAs func(p fixture.PracticeRecord, s fixture.StaffRecord) []testdb.SessionVar
		wantErr   bool
	}{
		{"with no Practice context set", nil,
			func(fixture.PracticeRecord, fixture.StaffRecord) []testdb.SessionVar { return nil }, true},
		{"with a Practice context and an employee's Membership", nil,
			func(p fixture.PracticeRecord, s fixture.StaffRecord) []testdb.SessionVar {
				return []testdb.SessionVar{testdb.Practice(p.ID), testdb.Staff(s.ID)}
			}, false},
		{"for a contractor Doula's Membership", []fixture.StaffOption{fixture.Contractor()},
			func(p fixture.PracticeRecord, s fixture.StaffRecord) []testdb.SessionVar {
				return []testdb.SessionVar{testdb.Practice(p.ID), testdb.Staff(s.ID)}
			}, true},
		{"for an Owner with a contractor employment type", []fixture.StaffOption{fixture.Owner(), fixture.Doula(), fixture.Contractor()},
			func(p fixture.PracticeRecord, s fixture.StaffRecord) []testdb.SessionVar {
				return []testdb.SessionVar{testdb.Practice(p.ID), testdb.Staff(s.ID)}
			}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testdb.New(t)
			p := fixture.Practice(t, db)
			s := fixture.Staff(t, db, p, tc.staff...)

			tx := db.BeginAs(t, tc.sessionAs(p, s)...)
			_, err := tx.ExecContext(t.Context(),
				`INSERT INTO clients (practice_id, given_name, email) VALUES ($1, 'New Client', 'new-client@example.com')`, p.ID)

			if (err != nil) != tc.wantErr {
				t.Fatalf("insert error = %v, want an error = %v", err, tc.wantErr)
			}
		})
	}
}

// TestRLS_EngagementsVisibilityIsScopedToCurrentPractice proves the
// engagements policy narrows to rows for app.current_practice_id, not
// every Engagement globally.
func TestRLS_EngagementsVisibilityIsScopedToCurrentPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA, practiceB := fixture.Practice(t, db), fixture.Practice(t, db)
	fixture.Engagement(t, db, practiceA)
	fixture.Engagement(t, db, practiceB)

	tx := db.BeginAs(t, testdb.Practice(practiceA.ID))

	if count := testdb.Scalar[int](t, tx, `SELECT count(*) FROM engagements`); count != 1 {
		t.Fatalf("expected only Practice A's engagement visible, got count = %d", count)
	}
}

// TestRLS_ClientsUpdateFollowsSelectScope proves clients_update (00042)
// follows clients_select's shape, per ADR-0017's "edit follows read": a
// Practice's own session can update its Client's row, and another
// Practice's session affects zero rows attempting the same update rather
// than erroring -- RLS filters rows, it doesn't reject the statement.
func TestRLS_ClientsUpdateFollowsSelectScope(t *testing.T) {
	db := testdb.New(t)
	practiceA, practiceB := fixture.Practice(t, db), fixture.Practice(t, db)
	client := fixture.Client(t, db, practiceA)

	tx := db.BeginAs(t, testdb.Practice(practiceB.ID))
	result, err := tx.ExecContext(t.Context(), `UPDATE clients SET given_name = 'Renamed' WHERE id = $1`, client.ID)
	if err != nil {
		t.Fatalf("update from another practice's session: %v", err)
	}
	if n, _ := result.RowsAffected(); n != 0 {
		t.Fatalf("Practice B's session updated %d rows of Practice A's Client, want 0", n)
	}

	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceA.ID); err != nil {
		t.Fatalf("set_config: %v", err)
	}
	result, err = tx.ExecContext(t.Context(), `UPDATE clients SET given_name = 'Renamed' WHERE id = $1`, client.ID)
	if err != nil {
		t.Fatalf("update from own practice's session: %v", err)
	}
	if n, _ := result.RowsAffected(); n != 1 {
		t.Fatalf("Practice A's session updated %d rows of its own Client, want 1", n)
	}
}

// TestEngagementsCompletedIsExplained_CHECKDemandsBothFacts proves
// engagements_completed_is_explained (00094) is a real database CHECK,
// not only TransitionHandler's own guards: a direct UPDATE to
// 'completed' missing either of ADR-0015's two facts is rejected by
// Postgres itself, superuser connection and all -- a CHECK applies
// regardless of role or RLS. Both halves are asserted separately,
// because #253 landed only the ending_reason one and a CHECK that had
// silently dropped back to it would still pass a single-case test.
func TestEngagementsCompletedIsExplained_CHECKDemandsBothFacts(t *testing.T) {
	db := testdb.New(t)
	e := fixture.Engagement(t, db, fixture.Practice(t, db), fixture.Status(fixture.StatusActive))

	for _, missing := range []struct{ name, statement string }{
		{"neither fact",
			`UPDATE engagements SET status = 'completed' WHERE id = $1`},
		{"no ending_reason",
			`UPDATE engagements SET status = 'completed', birth_outcome = 'unknown' WHERE id = $1`},
		{"no birth_outcome",
			`UPDATE engagements SET status = 'completed', ending_reason = 'care_complete' WHERE id = $1`},
	} {
		if _, err := db.Admin.ExecContext(t.Context(), missing.statement, e.ID); err == nil {
			t.Fatalf("expected the CHECK to reject a completed row with %s, got no error", missing.name)
		}
	}

	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE engagements SET status = 'completed', ending_reason = 'care_complete', birth_outcome = 'unknown'
		  WHERE id = $1`, e.ID,
	); err != nil {
		t.Fatalf("expected a completed row carrying both facts to be accepted, got: %v", err)
	}
}
