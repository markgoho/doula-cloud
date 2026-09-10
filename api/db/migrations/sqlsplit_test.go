package migrations

import (
	"strings"
	"testing"
)

// TestUpSectionBounds proves the Up section is exactly what runs on
// trunk: the annotation line itself is gone, so a statement recognized
// by what it starts with is not hidden behind it, and a Down section is
// left out.
func TestUpSectionBounds(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"no annotations at all", "SELECT 1;", "SELECT 1;"},
		{"up with no down", "-- +goose Up\nSELECT 1;", "\nSELECT 1;"},
		{"down is excluded", "-- +goose Up\nSELECT 1;\n-- +goose Down\nSELECT 2;", "\nSELECT 1;\n-- "},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := UpSection(c.body); got != c.want {
				t.Errorf("UpSection(%q) = %q, want %q", c.body, got, c.want)
			}
		})
	}
}

// TestSplitStatements proves the splitter keeps a semicolon that belongs
// to a string or a function body out of the statement boundary, and
// survives every unterminated form a half-written migration can take.
func TestSplitStatements(t *testing.T) {
	cases := []struct {
		name string
		sql  string
		want []string
	}{
		{"blank input has no statements", "  \n ", nil},
		{"comments are dropped", "-- a note\nSELECT 1;", []string{"\nSELECT 1"}},
		{"a comment need not end in a newline", "SELECT 1; -- trailing", []string{"SELECT 1"}},
		{"a semicolon inside a string does not split", "SELECT 'a;b';", []string{"SELECT 'a;b'"}},
		{"a doubled quote is an escape, not a close", "SELECT 'it''s; fine';", []string{"SELECT 'it''s; fine'"}},
		{"an unterminated string runs to the end", "SELECT 'oops", []string{"SELECT 'oops"}},
		{"a dollar-quoted body keeps its semicolons", "CREATE FUNCTION f() AS $$ BEGIN a; b; END $$;", []string{"CREATE FUNCTION f() AS $$ BEGIN a; b; END $$"}},
		{"a tagged dollar quote is matched by its tag", "DO $body$ x; $body$;", []string{"DO $body$ x; $body$"}},
		{"an unterminated dollar quote runs to the end", "DO $$ x;", []string{"DO $$ x;"}},
		{"a lone dollar is not a dollar quote", "SELECT $1;", []string{"SELECT $1"}},
		{"a dollar followed by punctuation is not a dollar quote", "SELECT a$.b;", []string{"SELECT a$.b"}},
		{"a dollar at the very end opens nothing", "SELECT a$", []string{"SELECT a$"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := SplitStatements(c.sql)
			if strings.Join(got, "|") != strings.Join(c.want, "|") {
				t.Errorf("SplitStatements(%q) = %q, want %q", c.sql, got, c.want)
			}
		})
	}
}

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
