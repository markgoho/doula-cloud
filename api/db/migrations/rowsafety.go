// The row-safety classifier that guardrail_test.go enforces over the
// embedded migrations. The rule it enforces, and why CI cannot see the
// failure any other way, is the package doc in embed.go.

package migrations

import (
	"regexp"
	"strings"
)

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
		name: "ADD COLUMN ... DEFAULT with an inline constraint",
		// A DEFAULT fills the new column in every existing row, so a
		// constraint written on the column itself is checked against
		// those filled-in values -- the one case where DEFAULT, which
		// makes NOT NULL safe, is what makes something else unsafe.
		// Written twice because the column's clauses come in either
		// order and RE2 has no lookahead.
		pattern: regexp.MustCompile(`\bADD COLUMN\b.*\bDEFAULT\b.*\b(REFERENCES|CHECK|UNIQUE)\b|` +
			`\bADD COLUMN\b.*\b(REFERENCES|CHECK|UNIQUE)\b.*\bDEFAULT\b`),
		failure: "the DEFAULT gives every existing row a value, and the inline constraint is then checked against it -- an orphan (23503), a violated predicate (23514), or, since every row gets the same value, a duplicate (23505)",
		remedy: "Add the column with its DEFAULT first, drop the default, and add the\n" +
			"    constraint in a later statement whose own safety you can state.",
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

// alterHeader matches the "ALTER TABLE <name>" an action hangs off, so
// each action can be read with its table still attached.
var alterHeader = regexp.MustCompile(`^ALTER TABLE (?:IF EXISTS )?(?:ONLY )?[A-Z0-9_."]+ `)

// alterActions splits one statement into the units a safe marker
// belongs to. A DEFAULT or a NOT VALID is written inside one action of
// an ALTER TABLE and exempts that action alone; reading the statement
// whole would let one action's NOT VALID cover the action beside it, and
// would report a second action's missing DEFAULT against the first one's
// present one. Anything that is not an ALTER TABLE is a single action.
func alterActions(norm string) []string {
	header := alterHeader.FindString(norm)
	if header == "" {
		return []string{norm}
	}
	var actions []string
	for _, action := range splitTopLevel(norm[len(header):]) {
		actions = append(actions, header+strings.TrimSpace(action))
	}
	return actions
}

// splitTopLevel splits on the commas that separate one ALTER TABLE
// action from the next -- those outside any parentheses and outside any
// quoted string, so a CHECK's value list and a DEFAULT's literal stay
// with the action that wrote them.
func splitTopLevel(s string) []string {
	var out []string
	depth, quoted, start := 0, false, 0
	for i := range len(s) {
		switch {
		case quoted:
			quoted = s[i] != '\''
		case s[i] == '\'':
			quoted = true
		case s[i] == '(':
			depth++
		case s[i] == ')':
			depth--
		case s[i] == ',' && depth == 0:
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

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
		for _, action := range alterActions(norm) {
			for _, c := range rowClasses {
				if !c.pattern.MatchString(action) {
					continue
				}
				if c.safe != nil && c.safe.MatchString(action) {
					continue
				}
				findings = append(findings, Finding{
					Class:     c.name,
					Statement: action,
					Failure:   c.failure,
					Remedy:    c.remedy,
				})
			}
		}
	}
	return findings
}

// collapse squeezes a statement onto one line so a multi-line ALTER
// reads as a single quotable line in a failure message.
func collapse(s string) string { return strings.Join(strings.Fields(s), " ") }
