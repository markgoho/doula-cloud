package testdb_test

import (
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
