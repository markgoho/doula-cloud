package testdb_test

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"testing"

	"doula-cloud/api/internal/testdb"
)

const workerTrusted = "true"

func TestBeginAs_ScopesTheTransactionToTheVars(t *testing.T) {
	db := testdb.New(t)
	practiceA := testdb.SeedPractice(t, db, "Practice A")
	_, engagementA := testdb.SeedEngagement(t, db, practiceA)
	testdb.SeedEngagement(t, db, testdb.SeedPractice(t, db, "Practice B"))

	if got := testdb.Scalar[int](t, db.App, `SELECT count(*) FROM engagements`); got != 0 {
		t.Fatalf("unscoped count = %d, want 0", got)
	}

	tx := db.BeginAs(t, testdb.Practice(practiceA))
	if got := testdb.Column[string](t, tx, `SELECT id FROM engagements`); !slices.Equal(got, []string{engagementA}) {
		t.Fatalf("visible engagements = %v, want only %q", got, engagementA)
	}
}

func TestBeginAdmin_ReadsPastRowLevelSecurity(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Admin Begin Practice")

	tx := db.BeginAdmin(t)
	if got := testdb.Scalar[int](t, tx, `SELECT count(*) FROM practices WHERE id = $1`, practiceID); got != 1 {
		t.Fatalf("practices visible to the admin transaction = %d, want 1 with no session variable set", got)
	}
}

func TestRunAsWorker_OpensTheTrustedDoorForThePass(t *testing.T) {
	db := testdb.New(t)
	var door string
	db.RunAsWorker(t, func(ctx context.Context, tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, `SELECT current_setting('app.notification_worker_trusted')`).Scan(&door)
	})

	if door != "true" {
		t.Fatalf("door inside the pass = %q, want true", door)
	}
}

func TestRunAsAdminWorker_CommitsThePassOutsideEveryPolicy(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Admin Worker Practice")

	db.RunAsAdminWorker(t, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE practices SET name = 'Worked' WHERE id = $1`, practiceID)
		if err != nil {
			return fmt.Errorf("rename: %w", err)
		}
		return nil
	})

	if got := testdb.Scalar[string](t, db.Admin, `SELECT name FROM practices WHERE id = $1`, practiceID); got != "Worked" {
		t.Fatalf("name after the pass = %q, want Worked", got)
	}
}

func TestSessionVars_NameTheirSettings(t *testing.T) {
	cases := map[string]testdb.SessionVar{
		"app.current_practice_id":  testdb.Practice("p"),
		"app.current_client_id":    testdb.Client("p"),
		"app.current_staff_id":     testdb.Staff("p"),
		"app.current_identity_uid": testdb.Identity("p"),
		"app.invite_token":         testdb.InviteToken("p"),
		"app.invite_token_digest":  testdb.InviteTokenDigest("p"),

		"app.notification_worker_trusted": testdb.NotificationWorker(),
	}
	db := testdb.New(t)
	tx := db.BeginAs(t, testdb.Practice("p"), testdb.Client("c"), testdb.Staff("s"), testdb.Identity("u"), testdb.InviteToken("t"), testdb.InviteTokenDigest("d"), testdb.NotificationWorker())
	want := map[string]string{
		"app.current_practice_id":  "p",
		"app.current_client_id":    "c",
		"app.current_staff_id":     "s",
		"app.current_identity_uid": "u",
		"app.invite_token":         "t",
		"app.invite_token_digest":  "d",

		"app.notification_worker_trusted": workerTrusted,
	}
	for name, v := range cases {
		if v.Name != name {
			t.Fatalf("var name = %q, want %q", v.Name, name)
		}
		if got := testdb.Scalar[string](t, tx, `SELECT current_setting($1)`, name); got != want[name] {
			t.Fatalf("%s = %q, want %q", name, got, want[name])
		}
	}
}

func TestExecCorrectingOutcome_MovesAFrozenOutcome(t *testing.T) {
	db := testdb.New(t)
	_, engagement := testdb.SeedEngagement(t, db, testdb.SeedPractice(t, db, "Practice A"))
	db.ExecCorrectingOutcome(t, `UPDATE engagements SET birth_outcome = 'loss', pregnancy_ended_on = '2027-03-04'::date WHERE id = $1`, engagement)

	db.ExecCorrectingOutcome(t, `UPDATE engagements SET birth_outcome = 'live_birth' WHERE id = $1`, engagement)

	if got := testdb.Scalar[string](t, db.Admin, `SELECT birth_outcome::text FROM engagements WHERE id = $1`, engagement); got != "live_birth" {
		t.Fatalf("birth_outcome = %q, want live_birth", got)
	}
}

func TestColumn_ReturnsNilForNoRows(t *testing.T) {
	db := testdb.New(t)

	if got := testdb.Column[string](t, db.Admin, `SELECT id FROM engagements`); got != nil {
		t.Fatalf("rows = %v, want none", got)
	}
}

func TestRowsAffected_CountsTheRowsAWriteTouched(t *testing.T) {
	db := testdb.New(t)
	practice := testdb.SeedPractice(t, db, "Practice A")
	_, engagement := testdb.SeedEngagement(t, db, practice)

	tx := db.BeginAs(t, testdb.Practice(practice))
	if got := testdb.RowsAffected(t, tx, `UPDATE engagements SET kind = kind WHERE id = $1`, engagement); got != 1 {
		t.Fatalf("rows affected = %d, want 1", got)
	}
}

func TestRowsAffected_IsZeroForRowsTheSessionCannotSee(t *testing.T) {
	db := testdb.New(t)
	_, engagement := testdb.SeedEngagement(t, db, testdb.SeedPractice(t, db, "Practice A"))
	other := testdb.SeedPractice(t, db, "Practice B")

	tx := db.BeginAs(t, testdb.Practice(other))
	if got := testdb.RowsAffected(t, tx, `UPDATE engagements SET kind = kind WHERE id = $1`, engagement); got != 0 {
		t.Fatalf("rows affected = %d, want 0", got)
	}
}

func TestRequireFailsClosed_PassesWhenNoRowIsVisible(t *testing.T) {
	db := testdb.New(t)
	testdb.SeedEngagement(t, db, testdb.SeedPractice(t, db, "Practice A"))

	testdb.RequireFailsClosed(t, db, "engagements")
}

func TestReadAs_ReadsInATransactionScopedToWho(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Practice A")
	clientID, engagementID := testdb.SeedEngagement(t, db, practiceID)
	read := func(ctx context.Context, tx *sql.Tx, id string) (string, bool, error) {
		var found string
		err := tx.QueryRowContext(ctx, `SELECT id FROM engagements WHERE id = $1`, id).Scan(&found)
		return found, err == nil, nil
	}

	if got, found := testdb.ReadAs(t, db, testdb.Client(clientID), engagementID, read); !found || got != engagementID {
		t.Fatalf("the Client read %q, found = %v, want her own Engagement", got, found)
	}
	if _, found := testdb.ReadAs(t, db, testdb.Client("00000000-0000-0000-0000-000000000000"), engagementID, read); found {
		t.Fatalf("a stranger found the Engagement, want it hidden")
	}
}

func TestBeginAdmin_KeepsWhatItCommitsAndRollsBackTheRestWhenTheTestEnds(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Admin Commit Practice")
	nameOf := func() string {
		return testdb.Scalar[string](t, db.Admin, `SELECT name FROM practices WHERE id = $1`, practiceID)
	}
	rename := func(t *testing.T, tx *sql.Tx, name string) {
		t.Helper()
		if _, err := tx.ExecContext(t.Context(), `UPDATE practices SET name = $1 WHERE id = $2`, name, practiceID); err != nil {
			t.Fatalf("rename: %v", err)
		}
	}

	t.Run("a committed write survives the test", func(t *testing.T) {
		tx := db.BeginAdmin(t)
		rename(t, tx, "Committed Name")
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
	})
	if got := nameOf(); got != "Committed Name" {
		t.Fatalf("name after the committing test = %q, want %q", got, "Committed Name")
	}

	t.Run("an uncommitted write is rolled back", func(t *testing.T) {
		rename(t, db.BeginAdmin(t), "Uncommitted Name")
	})
	if got := nameOf(); got != "Committed Name" {
		t.Fatalf("name after the uncommitted test = %q, want %q", got, "Committed Name")
	}
}
