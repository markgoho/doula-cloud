package testdb_test

import (
	"context"
	"database/sql"
	"slices"
	"testing"

	"doula-cloud/api/internal/testdb"
)

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

func TestSessionVars_NameTheirSettings(t *testing.T) {
	cases := map[string]testdb.SessionVar{
		"app.current_practice_id":  testdb.Practice("p"),
		"app.current_client_id":    testdb.Client("p"),
		"app.current_staff_id":     testdb.Staff("p"),
		"app.current_identity_uid": testdb.Identity("p"),
	}
	db := testdb.New(t)
	tx := db.BeginAs(t, testdb.Practice("p"), testdb.Client("c"), testdb.Staff("s"), testdb.Identity("u"))
	want := map[string]string{
		"app.current_practice_id":  "p",
		"app.current_client_id":    "c",
		"app.current_staff_id":     "s",
		"app.current_identity_uid": "u",
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
