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
		want []string
	}{
		{"no annotations at all", "SELECT 1;", []string{"SELECT 1"}},
		{"up with no down", "-- +goose Up\nSELECT 1;", []string{"\nSELECT 1"}},
		{"down is excluded", "-- +goose Up\nSELECT 1;\n-- +goose Down\nSELECT 2;", []string{"\nSELECT 1"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Read through the splitter, which is how every caller reads
			// it -- the raw text carries whitespace and a half-eaten
			// comment that nothing downstream can see.
			got := SplitStatements(UpSection(c.body))
			if strings.Join(got, "|") != strings.Join(c.want, "|") {
				t.Errorf("statements of UpSection(%q) = %q, want %q", c.body, got, c.want)
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
