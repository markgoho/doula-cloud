// Package migrations' guardrail: a migration that passes every PR and
// fails the first trunk push is the one failure mode CI cannot show you
// before you merge, because the job that applies migrations to a real
// database (migrate, .github/workflows/ci.yml) runs only on trunk. Every
// PR builds an empty database per run, so a constraint that only existing
// rows can violate is invisible until it is too late to catch cheaply.
//
// #1021 is the incident this exists to prevent a repeat of: #967 shipped
// `ALTER TABLE contracts ADD COLUMN amount_cents bigint NOT NULL`, green
// on its PR, and trunk's migrate job then failed with `column
// "amount_cents" of relation "contracts" contains null values`. Trunk
// stayed red across seven merges and no deploy ran in that window.
package migrations

import (
	"regexp"
	"strings"
	"testing"
)

// addColumnNotNull matches an ADD COLUMN whose definition carries NOT
// NULL. The type sits between the name and the constraint and can be
// anything (bigint, text, an enum name), so the pattern is deliberately
// loose about it and strict about the two things that matter: that this
// is an ADD COLUMN, and that NOT NULL appears in its definition.
var addColumnNotNull = regexp.MustCompile(`(?i)ADD\s+COLUMN\s+[^;]*?\bNOT\s+NULL\b[^;]*;`)

// grandfathered are the ADD COLUMN ... NOT NULL statements that predate
// this guardrail and already applied cleanly, each against a table that
// held no rows at the time. They are recorded by file rather than
// rewritten: a migration that has already run on doula-cloud-pg must not
// change, because goose will not re-run it and the two would silently
// disagree. Nothing may be added to this list -- a new migration takes
// the DEFAULT-then-DROP form instead.
var grandfathered = map[string]string{
	"00030_employment_attachment_offer.sql": "applied 2026-06 against an empty staff_practices; goose has recorded it, so it cannot be rewritten",
	"00095_manual_payment_recording.sql":    "applied 2026-09-08 against an empty invoices; goose has recorded it, so it cannot be rewritten",
}

// TestNoAddColumnNotNullWithoutDefault fails a migration that adds a NOT
// NULL column with no DEFAULT to backfill the rows already in the table.
//
// The form that is safe, and what a new migration must use:
//
//	ALTER TABLE t ADD COLUMN c bigint NOT NULL DEFAULT 0;
//	ALTER TABLE t ALTER COLUMN c DROP DEFAULT;
//
// The DEFAULT backfills existing rows; dropping it afterwards means no
// future INSERT silently gets a value it never supplied, so a column
// whose zero value is meaningless (see #966's "no rate set is not zero")
// still forces every new row to say what it means.
func TestNoAddColumnNotNullWithoutDefault(t *testing.T) {
	entries, err := FS.ReadDir(".")
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}

	checked := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}
		body, err := FS.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		// Only the Up section can add a column to a populated table; a
		// Down runs against whatever the Up left behind and is not the
		// failure mode this guards.
		up := upSection(string(body))
		for _, stmt := range addColumnNotNull.FindAllString(up, -1) {
			if strings.Contains(strings.ToUpper(stmt), "DEFAULT") {
				continue
			}
			if reason, ok := grandfathered[name]; ok {
				t.Logf("%s: grandfathered (%s)", name, reason)
				continue
			}
			t.Errorf(`%s adds a NOT NULL column with no DEFAULT:

    %s

This passes every PR -- testdb builds an empty database, so no row can
violate the constraint -- and fails the first time trunk's migrate job
applies it to doula-cloud-pg, which has rows. Write it as two statements
instead, so existing rows are backfilled and new rows still must supply
a value:

    ALTER TABLE <table> ADD COLUMN <col> <type> NOT NULL DEFAULT <value>;
    ALTER TABLE <table> ALTER COLUMN <col> DROP DEFAULT;

See #1021 and the comment at the top of this file.`, name, strings.TrimSpace(collapse(stmt)))
		}
		checked++
	}

	if checked == 0 {
		t.Fatal("no migrations found to check -- the embed pattern or this test's filter is wrong")
	}
}

// TestGrandfatheredListDoesNotGrow keeps the exemption list from becoming
// a habit: every name in it must still exist and must still be an
// offender, so a migration that gets fixed or deleted is removed from the
// list rather than left as cover for a future one.
func TestGrandfatheredListDoesNotGrow(t *testing.T) {
	const want = 2
	if len(grandfathered) != want {
		t.Fatalf("grandfathered has %d entries, want exactly %d -- a new migration must take the DEFAULT-then-DROP form, not an exemption", len(grandfathered), want)
	}
	for name := range grandfathered {
		body, err := FS.ReadFile(name)
		if err != nil {
			t.Errorf("grandfathered migration %s no longer exists; drop it from the list", name)
			continue
		}
		found := false
		for _, stmt := range addColumnNotNull.FindAllString(upSection(string(body)), -1) {
			if !strings.Contains(strings.ToUpper(stmt), "DEFAULT") {
				found = true
			}
		}
		if !found {
			t.Errorf("grandfathered migration %s no longer adds a NOT NULL column without a DEFAULT; drop it from the list", name)
		}
	}
}

// upSection returns everything between the goose Up annotation and the
// Down annotation, or the whole body when there is no Down.
func upSection(body string) string {
	const upMarker = "+goose Up"
	const downMarker = "+goose Down"
	start := strings.Index(body, upMarker)
	if start < 0 {
		return body
	}
	rest := body[start:]
	up, _, found := strings.Cut(rest, downMarker)
	if found {
		return up
	}
	return rest
}

// collapse squeezes a statement onto one line so a multi-line ALTER reads
// as a single quotable line in the failure message.
func collapse(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
