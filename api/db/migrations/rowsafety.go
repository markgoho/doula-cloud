// The row-safety classifier that guardrail_test.go enforces over the
// embedded migrations.
//
// The problem it exists for: the job that applies migrations to a real
// database (migrate, .github/workflows/ci.yml) runs only on a push to
// trunk, alongside deploy-api and deploy-app. Every pull request builds
// an empty Postgres per test process instead, so any statement whose
// success depends on the rows a table already holds is green on the PR
// and red on the first trunk push -- and the deploys queued behind
// migrate never run. #1021 is the incident: #967 shipped
// `ALTER TABLE contracts ADD COLUMN amount_cents bigint NOT NULL`, green
// on its PR, and trunk stayed red across seven merges.
//
// The rule enforced here, stated once: a migration's Up section may
// contain no statement whose success depends on the rows a table already
// holds, unless a safety note in safety/ says why those rows cannot
// break it. RowDependent reports every such statement; rowClasses
// enumerates the family, each member derived from a Postgres operation
// documented as scanning, rewriting or verifying existing rows, and each
// proved against a real populated Postgres in rowsafety_pg_test.go.

package migrations

import (
	"embed"
	"regexp"
	"strings"
)

// SafetyFS holds the safety notes that attest to a row-dependent
// statement being safe against the rows already in the table. They are a
// separate embed from FS so goose never sees them: a migration that has
// already applied cannot be edited to carry its own marker, so the
// marker lives beside the file rather than inside it.
//
//go:embed safety/*.md
var SafetyFS embed.FS

// Finding is one row-dependent statement found in an Up section.
type Finding struct {
	// Class names the shape, e.g. "CREATE UNIQUE INDEX".
	Class string
	// Statement is the offending statement, collapsed onto one line.
	Statement string
	// Failure is the trunk-only failure the statement risks.
	Failure string
	// Remedy is what the author should write instead.
	Remedy string
}

// rowClass is one member of the family. pattern is matched against a
// single comment-stripped, whitespace-collapsed, upper-cased statement.
// safe, when set, exempts a statement that also matches it -- the form
// Postgres can apply without consulting a single existing row.
type rowClass struct {
	name    string
	pattern *regexp.Regexp
	safe    *regexp.Regexp
	failure string
	remedy  string
}

// rowClasses is the family. It was built by walking the Postgres
// ALTER TABLE reference for every subform documented as scanning,
// rewriting or verifying the table's existing contents, then adding
// CREATE UNIQUE INDEX (the same duplicate check spelled differently) and
// the statements whose entire effect is on rows -- DML and an anonymous
// DO block, which a PR's empty database runs over nothing at all.
var rowClasses = []rowClass{
	{
		name:    "ADD COLUMN ... NOT NULL without DEFAULT",
		pattern: regexp.MustCompile(`\bADD COLUMN\b.*\bNOT NULL\b`),
		safe:    regexp.MustCompile(`\bDEFAULT\b`),
		failure: `column "..." of relation "..." contains null values (23502) -- every existing row gets NULL`,
		remedy: "ALTER TABLE <table> ADD COLUMN <col> <type> NOT NULL DEFAULT <value>;\n" +
			"    ALTER TABLE <table> ALTER COLUMN <col> DROP DEFAULT;",
	},
	{
		name:    "ALTER COLUMN ... SET NOT NULL",
		pattern: regexp.MustCompile(`\bALTER COLUMN\b.*\bSET NOT NULL\b`),
		failure: `column "..." of relation "..." contains null values (23502) -- one existing NULL refuses it`,
		remedy: "Backfill in the same migration and say in a safety note why the backfill\n" +
			"    reaches every row:\n\n" +
			"    UPDATE <table> SET <col> = <value> WHERE <col> IS NULL;\n" +
			"    ALTER TABLE <table> ALTER COLUMN <col> SET NOT NULL;",
	},
	{
		name:    "ALTER COLUMN ... TYPE",
		pattern: regexp.MustCompile(`\bALTER COLUMN\b.*\b(SET DATA )?TYPE\b`),
		failure: "the cast runs over every existing row and fails on the first value it cannot convert",
		remedy: "Add a USING clause that converts every value the column can already hold,\n" +
			"    and record in a safety note which values those are.",
	},
	{
		name:    "ADD CONSTRAINT ... UNIQUE / PRIMARY KEY / EXCLUDE",
		pattern: regexp.MustCompile(`\bADD (CONSTRAINT \S+ )?(UNIQUE|PRIMARY KEY|EXCLUDE)\b`),
		failure: `could not create unique index "..." -- Key (...) is duplicated (23505)`,
		remedy: "Prove in a safety note that no duplicate can already exist, or delete the\n" +
			"    duplicates first -- and check the delete is itself safe (see the DML class).",
	},
	{
		name:    "ADD CONSTRAINT ... CHECK",
		pattern: regexp.MustCompile(`\bADD (CONSTRAINT \S+ )?CHECK\b`),
		safe:    regexp.MustCompile(`\bNOT VALID\b`),
		failure: `check constraint "..." of relation "..." is violated by some row (23514)`,
		remedy: "Add it NOT VALID, so only new rows are held to it, or prove in a safety note\n" +
			"    that every existing row already satisfies the predicate.",
	},
	{
		name:    "ADD CONSTRAINT ... FOREIGN KEY",
		pattern: regexp.MustCompile(`\bADD (CONSTRAINT \S+ )?FOREIGN KEY\b`),
		safe:    regexp.MustCompile(`\bNOT VALID\b`),
		failure: `insert or update on table "..." violates foreign key constraint (23503) -- an existing orphan`,
		remedy:  "Add it NOT VALID, or prove in a safety note that no orphan row exists.",
	},
	{
		name:    "VALIDATE CONSTRAINT",
		pattern: regexp.MustCompile(`\bVALIDATE CONSTRAINT\b`),
		failure: "this is the scan a NOT VALID constraint deferred; it fails on the first row that breaks it",
		remedy:  "Prove in a safety note that every existing row satisfies the constraint.",
	},
	{
		name:    "CREATE UNIQUE INDEX",
		pattern: regexp.MustCompile(`^CREATE UNIQUE INDEX\b`),
		failure: `could not create unique index "..." -- Key (...) is duplicated (23505)`,
		remedy:  "Prove in a safety note that no duplicate can already exist.",
	},
	{
		name:    "ADD COLUMN ... GENERATED ALWAYS AS",
		pattern: regexp.MustCompile(`\bADD COLUMN\b.*\bGENERATED ALWAYS AS \(`),
		failure: "the generation expression is evaluated for every existing row and fails on the first one it cannot compute",
		remedy: "Prove in a safety note that the expression is total over the values the\n" +
			"    source columns already hold.",
	},
	{
		name:    "DML (UPDATE / DELETE / INSERT)",
		pattern: regexp.MustCompile(`^(UPDATE|DELETE FROM|INSERT INTO)\b`),
		failure: "a PR's empty database runs this over nothing, so nothing it does to real rows is tested -- " +
			"a DELETE meets a foreign key with no ON DELETE clause, an UPDATE meets a constraint",
		remedy: "Say in a safety note what rows this touches on a populated database and what\n" +
			"    could refuse them -- every foreign key pointing at the rows a DELETE removes included.",
	},
	{
		name:    "DO block",
		pattern: regexp.MustCompile(`^DO\b`),
		failure: "an anonymous block runs immediately and can read or write existing rows, and a PR's empty database exercises none of that",
		remedy:  "Say in a safety note what the block does when the tables it touches are not empty.",
	},
}

// createTable finds the table a CREATE TABLE creates, so statements
// later in the same Up section that target it are left alone: a table
// this migration just created holds no rows for any class to trip over.
var createTable = regexp.MustCompile(`^CREATE TABLE (?:IF NOT EXISTS )?([A-Z0-9_."]+)`)

// targetTable finds the table an ALTER TABLE, CREATE INDEX or DML
// statement acts on.
var targetTable = regexp.MustCompile(`^(?:ALTER TABLE (?:IF EXISTS )?(?:ONLY )?|CREATE (?:UNIQUE )?INDEX (?:CONCURRENTLY )?(?:IF NOT EXISTS )?\S+ ON (?:ONLY )?|UPDATE |DELETE FROM |INSERT INTO )([A-Z0-9_."]+)`)

// RowDependent returns every statement in an Up section whose success
// depends on the rows a table already holds. Statements acting on a
// table the same Up section creates are not reported: that table is new
// and empty, so no existing row can refuse them.
func RowDependent(up string) []Finding {
	var findings []Finding
	fresh := map[string]bool{}

	for _, stmt := range SplitStatements(up) {
		norm := strings.ToUpper(collapse(stmt))
		if m := createTable.FindStringSubmatch(norm); m != nil {
			fresh[m[1]] = true
			continue
		}
		if m := targetTable.FindStringSubmatch(norm); m != nil && fresh[m[1]] {
			continue
		}
		for _, c := range rowClasses {
			if !c.pattern.MatchString(norm) {
				continue
			}
			if c.safe != nil && c.safe.MatchString(norm) {
				continue
			}
			findings = append(findings, Finding{
				Class:     c.name,
				Statement: collapse(stmt),
				Failure:   c.failure,
				Remedy:    c.remedy,
			})
		}
	}
	return findings
}

// collapse squeezes a statement onto one line so a multi-line ALTER
// reads as a single quotable line in a failure message.
func collapse(s string) string { return strings.Join(strings.Fields(s), " ") }
