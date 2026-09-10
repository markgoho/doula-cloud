package migrations

import "testing"

// TestRowDependentLeavesAFreshTableAlone proves the one exemption the
// classifier makes on its own: a table this Up section just created
// holds no rows, so nothing it does to that table can meet one.
func TestRowDependentLeavesAFreshTableAlone(t *testing.T) {
	fresh := `CREATE TABLE new_thing (id uuid, slug text);
	          CREATE UNIQUE INDEX new_thing_slug_key ON new_thing (slug);
	          ALTER TABLE new_thing ADD CONSTRAINT new_thing_id_key UNIQUE (id);`
	if got := RowDependent(fresh); len(got) != 0 {
		t.Errorf("RowDependent over a table created in the same migration = %+v, want none", got)
	}

	old := `CREATE TABLE new_thing (id uuid);
	        CREATE UNIQUE INDEX old_thing_slug_key ON old_thing (slug);`
	if got := RowDependent(old); len(got) != 1 || got[0].Class != "CREATE UNIQUE INDEX" {
		t.Errorf("RowDependent over a table that predates the migration = %+v, want one CREATE UNIQUE INDEX finding", got)
	}
}

// TestSafeFormOnlyExemptsASingleAction proves a marker belongs to the
// action it is written in: a NOT VALID beside an action without one
// exempts nothing.
func TestSafeFormOnlyExemptsASingleAction(t *testing.T) {
	lone := `ALTER TABLE t ADD CONSTRAINT t_amount_positive CHECK (amount > 0) NOT VALID;`
	if got := RowDependent(lone); len(got) != 0 {
		t.Errorf("RowDependent over a lone NOT VALID check = %+v, want none", got)
	}

	beside := `ALTER TABLE t ADD CONSTRAINT t_id_positive CHECK (id > 0) NOT VALID,
	                         ADD CONSTRAINT t_amount_positive CHECK (amount > 0);`
	if got := RowDependent(beside); len(got) != 1 || got[0].Class != "ADD CONSTRAINT ... CHECK" {
		t.Errorf("RowDependent over two CHECK actions sharing one NOT VALID = %+v, want one ADD CONSTRAINT ... CHECK finding", got)
	}
}

// TestADefaultOnAnInlineConstraintIsNotSafe proves the one place DEFAULT
// stops being the remedy and becomes the hazard: it fills every existing
// row, and a constraint written on the column is then checked against
// what it filled in.
func TestADefaultOnAnInlineConstraintIsNotSafe(t *testing.T) {
	const class = "ADD COLUMN ... DEFAULT with an inline constraint"

	after := `ALTER TABLE t ADD COLUMN parent_id int NOT NULL DEFAULT 99 REFERENCES parent (id);`
	if got := RowDependent(after); len(got) != 1 || got[0].Class != class {
		t.Errorf("RowDependent(%s) = %+v, want one %q finding", after, got, class)
	}

	before := `ALTER TABLE t ADD COLUMN slug text UNIQUE DEFAULT 'x';`
	if got := RowDependent(before); len(got) != 1 || got[0].Class != class {
		t.Errorf("RowDependent(%s) = %+v, want one %q finding", before, got, class)
	}

	plain := `ALTER TABLE t ADD COLUMN amount bigint NOT NULL DEFAULT 0;`
	if got := RowDependent(plain); len(got) != 0 {
		t.Errorf("RowDependent(%s) = %+v, want none -- this is the prescribed safe form", plain, got)
	}
}
