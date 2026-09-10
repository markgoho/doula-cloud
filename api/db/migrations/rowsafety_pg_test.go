// The proof behind rowsafety.go's rowClasses. Every class asserted
// there is asserted here against a real Postgres instead of against the
// Postgres documentation: each case builds the same schema twice, once
// empty (what a pull request's testdb gives every migration) and once
// loaded (what trunk's migrate job finds on doula-cloud-pg -- one row,
// or the fewest rows the class needs), and requires the statement to
// succeed in the first and fail in the second. That difference is the
// whole defect class of #1021, and the reason a green PR proves nothing
// about migrate, deploy-api or deploy-app -- all three run only on a
// push to trunk.
//
// Each case also records what the guardrail did before #1139 widened it:
// legacyCaught is the pattern the guardrail shipped with, and a case
// that does not set legacy must be one it never matched.

package migrations_test

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"doula-cloud/api/db/migrations"
	"doula-cloud/api/internal/testdb"
)

// legacyCaught reports whether the guardrail as #1022 shipped it would
// have failed this statement: an ADD COLUMN carrying NOT NULL and no
// DEFAULT, and nothing else.
func legacyCaught(stmt string) bool {
	legacy := regexp.MustCompile(`(?i)ADD\s+COLUMN\s+[^;]*?\bNOT\s+NULL\b[^;]*;`)
	for _, m := range legacy.FindAllString(stmt+";", -1) {
		if !strings.Contains(strings.ToUpper(m), "DEFAULT") {
			return true
		}
	}
	return false
}

// The fixtures more than one case shares, named so the same schema and
// the same offending row are one thing rather than several.
const (
	oneRow         = `INSERT INTO t VALUES (1);`
	amountSchema   = `CREATE TABLE t (id int, amount bigint);`
	negativeAmount = `INSERT INTO t VALUES (1, -1);`
)

// rowCase is one statement shape, the schema it needs, and the rows
// that make trunk refuse it.
type rowCase struct {
	// name is the case, and the class rowsafety.go must report unless
	// class says otherwise.
	name string
	// class overrides name where two cases prove the same class by
	// different routes.
	class string
	// schema is the DDL both worlds get. It never depends on rows.
	schema string
	// row populates the loaded world, and only that one.
	row string
	// pre runs in both worlds after row, for setup the statement needs
	// that would itself have refused the row had it run first.
	pre string
	// legacy says the guardrail as #1022 shipped it already caught this
	// class. Exactly one case sets it.
	legacy bool
	// stmt is the statement a migration's Up section would carry.
	stmt string
	// safe, when set, is the same intent written so no existing row can
	// refuse it; it must succeed in both worlds and pass the guardrail.
	safe string
}

// rowCases covers every class in rowsafety.go's rowClasses.
var rowCases = []rowCase{
	{
		name:   "ADD COLUMN ... NOT NULL without DEFAULT",
		schema: `CREATE TABLE t (id int);`,
		row:    oneRow,
		stmt:   `ALTER TABLE t ADD COLUMN amount_cents bigint NOT NULL;`,
		safe: `ALTER TABLE t ADD COLUMN amount_cents bigint NOT NULL DEFAULT 0;
		       ALTER TABLE t ALTER COLUMN amount_cents DROP DEFAULT;`,
		legacy: true,
	},
	{
		// The hole a per-statement reading of DEFAULT leaves: DEFAULT
		// makes NOT NULL safe and is exactly what makes an inline
		// constraint unsafe, since every existing row is handed the
		// same value to be checked.
		name: "ADD COLUMN ... DEFAULT with an inline constraint",
		schema: `CREATE TABLE parent (id int PRIMARY KEY);
		         CREATE TABLE t (id int);`,
		row:  oneRow,
		stmt: `ALTER TABLE t ADD COLUMN parent_id int NOT NULL DEFAULT 99 REFERENCES parent (id);`,
	},
	{
		// The hole a per-statement reading of NOT VALID leaves: one
		// action's marker must not cover the action beside it.
		name:   "a NOT VALID action beside one without it",
		class:  "ADD CONSTRAINT ... CHECK",
		schema: amountSchema,
		row:    negativeAmount,
		stmt: `ALTER TABLE t ADD CONSTRAINT t_id_positive CHECK (id > 0) NOT VALID,
		                     ADD CONSTRAINT t_amount_positive CHECK (amount > 0);`,
	},
	{
		name:   "ALTER COLUMN ... SET NOT NULL",
		schema: `CREATE TABLE t (id int, note text);`,
		row:    `INSERT INTO t VALUES (1, NULL);`,
		stmt:   `ALTER TABLE t ALTER COLUMN note SET NOT NULL;`,
	},
	{
		name:   "ALTER COLUMN ... TYPE",
		schema: `CREATE TABLE t (id int, amount text);`,
		row:    `INSERT INTO t VALUES (1, 'not a number');`,
		stmt:   `ALTER TABLE t ALTER COLUMN amount TYPE bigint USING amount::bigint;`,
	},
	{
		name:   "ADD CONSTRAINT ... UNIQUE / PRIMARY KEY / EXCLUDE",
		schema: `CREATE TABLE t (id int, slug text);`,
		row:    `INSERT INTO t VALUES (0, 'taken'), (1, 'taken');`,
		stmt:   `ALTER TABLE t ADD CONSTRAINT t_slug_key UNIQUE (slug);`,
	},
	{
		name:   "ADD CONSTRAINT ... CHECK",
		schema: amountSchema,
		row:    negativeAmount,
		stmt:   `ALTER TABLE t ADD CONSTRAINT t_amount_positive CHECK (amount > 0);`,
		safe:   `ALTER TABLE t ADD CONSTRAINT t_amount_positive CHECK (amount > 0) NOT VALID;`,
	},
	{
		name: "ADD CONSTRAINT ... FOREIGN KEY",
		schema: `CREATE TABLE parent (id int PRIMARY KEY);
		         CREATE TABLE t (id int, parent_id int);`,
		row:  `INSERT INTO t VALUES (1, 99);`,
		stmt: `ALTER TABLE t ADD CONSTRAINT t_parent_fk FOREIGN KEY (parent_id) REFERENCES parent (id);`,
		safe: `ALTER TABLE t ADD CONSTRAINT t_parent_fk FOREIGN KEY (parent_id) REFERENCES parent (id) NOT VALID;`,
	},
	{
		name:   "VALIDATE CONSTRAINT",
		schema: amountSchema,
		row:    negativeAmount,
		pre:    `ALTER TABLE t ADD CONSTRAINT t_amount_positive CHECK (amount > 0) NOT VALID;`,
		stmt:   `ALTER TABLE t VALIDATE CONSTRAINT t_amount_positive;`,
	},
	{
		name:   "CREATE UNIQUE INDEX",
		schema: `CREATE TABLE t (id int, slug text);`,
		row:    `INSERT INTO t VALUES (0, 'taken'), (1, 'taken');`,
		stmt:   `CREATE UNIQUE INDEX t_slug_uidx ON t (slug);`,
	},
	{
		name:   "ADD COLUMN ... GENERATED ALWAYS AS",
		schema: `CREATE TABLE t (id int, divisor bigint);`,
		row:    `INSERT INTO t VALUES (1, 0);`,
		stmt:   `ALTER TABLE t ADD COLUMN ratio bigint GENERATED ALWAYS AS (100 / divisor) STORED;`,
	},
	{
		name: "DML (UPDATE / DELETE / INSERT)",
		schema: `CREATE TABLE parent (id int PRIMARY KEY);
		         CREATE TABLE t (id int, parent_id int REFERENCES parent (id));`,
		row:  `INSERT INTO parent VALUES (1); INSERT INTO t VALUES (1, 1);`,
		stmt: `DELETE FROM parent WHERE id = 1;`,
	},
	{
		name:   "DO block",
		schema: `CREATE TABLE t (id int);`,
		row:    oneRow,
		stmt: `DO $$ BEGIN
		         IF EXISTS (SELECT 1 FROM t) THEN
		           RAISE EXCEPTION 'a row was already here';
		         END IF;
		       END $$;`,
	},
}

// TestEachRowClassIsRealAndCaught is the proof: for every class, the
// statement succeeds on an empty database and fails on a populated one,
// the guardrail as it shipped in #1022 missed it, and the guardrail as
// #1139 leaves it catches it.
func TestEachRowClassIsRealAndCaught(t *testing.T) {
	for _, c := range rowCases {
		t.Run(c.name, func(t *testing.T) {
			class := c.class
			if class == "" {
				class = c.name
			}
			if got := migrations.RowDependent(upWrap(c.stmt)); len(got) != 1 || got[0].Class != class {
				t.Fatalf("RowDependent(%s) = %+v, want exactly one finding of class %q", c.stmt, got, class)
			}
			if got := legacyCaught(c.stmt); got != c.legacy {
				t.Errorf("the pre-#1139 guardrail caught %q = %v, want %v -- a case it already caught proves nothing new", c.name, got, c.legacy)
			}

			db := testdb.New(t)

			if err := runIn(t, db, c, "", c.stmt); err != nil {
				t.Fatalf("%s failed on an empty table, so a PR would already have caught it: %v", c.name, err)
			}
			err := runIn(t, db, c, c.row, c.stmt)
			if err == nil {
				t.Fatalf("%s succeeded against a populated table; it is not row-dependent after all", c.name)
			}
			t.Logf("trunk's migrate job would have failed with: %v", err)

			if c.safe == "" {
				return
			}
			if got := migrations.RowDependent(upWrap(c.safe)); len(got) != 0 {
				t.Errorf("the safe form of %q is still reported: %+v", c.name, got)
			}
			if err := runIn(t, db, c, c.row, c.safe); err != nil {
				t.Errorf("the safe form of %q failed against a populated table: %v", c.name, err)
			}
		})
	}
}

// runIn builds one world -- the case's schema, then row (empty for the
// world a pull request gets), then the case's pre -- in a throwaway
// Postgres schema, and runs stmt there. Only stmt's error is returned;
// a failure anywhere in the setup is the test's own bug.
func runIn(t *testing.T, db *testdb.DB, c rowCase, row, stmt string) error {
	t.Helper()
	ctx := context.Background()

	conn, err := db.Admin.Conn(ctx)
	if err != nil {
		t.Fatalf("open conn: %v", err)
	}
	defer func() { _ = conn.Close() }()

	name := "w" + strings.Map(keepWordByte, t.Name())
	for _, s := range []string{"DROP SCHEMA IF EXISTS " + name + " CASCADE", "CREATE SCHEMA " + name, "SET search_path TO " + name} {
		if _, err := conn.ExecContext(ctx, s); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
	if err := execEach(ctx, conn, c.schema+row+c.pre); err != nil {
		t.Fatalf("build world: %v", err)
	}
	return execEach(ctx, conn, stmt)
}

// execEach runs each statement of sql on conn, stopping at the first
// error -- one ExecContext per statement, so a failure names the
// statement that caused it the way goose does.
func execEach(ctx context.Context, conn *sql.Conn, sql string) error {
	for _, s := range migrations.SplitStatements(sql) {
		if _, err := conn.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("%s: %w", strings.TrimSpace(s), err)
		}
	}
	return nil
}

// keepWordByte maps a subtest name to something usable as an identifier.
func keepWordByte(r rune) rune {
	switch {
	case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		return r
	case r >= 'A' && r <= 'Z':
		return r + ('a' - 'A')
	default:
		return '_'
	}
}

// upWrap presents a bare statement as a goose Up section, which is what
// RowDependent reads.
func upWrap(stmt string) string {
	return migrations.UpSection("-- +goose Up\n" + stmt + "\n-- +goose Down\n")
}
