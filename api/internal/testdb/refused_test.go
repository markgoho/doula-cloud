package testdb_test

import (
	"testing"

	"doula-cloud/api/internal/testdb"
)

func TestRequireRefused_HoldsTheSchemaToItsConstraint(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Refusals")

	testdb.RequireRefused(t, db, "name", map[string]testdb.Write{
		"a practice with no name": {Query: `UPDATE practices SET name = NULL WHERE id = $1`, Args: []any{practiceID}},
	})
	testdb.RequireAccepted(t, db, map[string]testdb.Write{
		"a rename": {Query: `UPDATE practices SET name = 'Renamed' WHERE id = $1`, Args: []any{practiceID}},
	})
	if got := testdb.Scalar[string](t, db.Admin, `SELECT name FROM practices WHERE id = $1`, practiceID); got != "Renamed" {
		t.Errorf("name = %q, want the accepted write to land", got)
	}
}
