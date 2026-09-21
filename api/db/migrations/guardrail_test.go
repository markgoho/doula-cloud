package migrations

import (
	"embed"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// safetyFS holds the safety notes that attest to a row-dependent
// statement being safe against the rows already in the table. It is a
// separate embed from FS so goose never sees the notes, and it lives in
// this test file so the running BFF -- which imports this package for
// the migrations themselves -- does not carry them.
//
//go:embed safety/*.md
var safetyFS embed.FS

// grandfatheredEntry names every class a migration is excused for, and
// why the guardrail could not run when the migration applied. classes is
// what actually narrows the exemption to the class it names -- a class
// the migration raises that is not in the list still fails the
// guardrail, the same way an unquoted statement still fails a safety
// note (see statementQuoted).
type grandfatheredEntry struct {
	classes []string
	reason  string
}

// beforeGuardrail is the reason shared by every entry excused because it
// applied before #1139 widened the guardrail to the class it names.
const beforeGuardrail = "already applied to doula-cloud-pg before the guardrail covered it (#1139)"

// grandfathered are migrations that carry a row-dependent statement,
// have already applied against doula-cloud-pg, and cannot be rewritten:
// goose has recorded them, so editing one makes the file and the
// database silently disagree. Each entry names every class it is
// excused for and says why. The list is closed --
// TestGrandfatheredListDoesNotGrow pins its exact size, so a new
// migration takes the safe form or writes a safety note, never an
// exemption.
//
//nolint:gosec // G101 reads the constraint classes below as a possible credential; every value is a filename and a sentence of English, and nothing in this package holds a secret
var grandfathered = map[string]grandfatheredEntry{
	"00002_practice_staff_tenancy.sql":        {classes: []string{classDoBlock}, reason: beforeGuardrail},
	"00020_contracts_recreate_after_void.sql": {classes: []string{classCreateUniqueIndex}, reason: beforeGuardrail},
	"00026_client_portal_provisioning.sql":    {classes: []string{classCreateUniqueIndex}, reason: beforeGuardrail},
	"00029_stripe_connect_accounts_v2.sql": {
		classes: []string{classAddColumnDefaultWithInlineConstraint},
		reason:  beforeGuardrail,
	},
	"00030_employment_attachment_offer.sql": {
		classes: []string{classAddColumnNotNullWithoutDefault, classAlterColumnSetNotNull},
		reason:  "applied 2026-06 against an empty staff_practices",
	},
	"00039_membership_events.sql": {classes: []string{classCreateUniqueIndex}, reason: beforeGuardrail},
	"00042_client_intake_schema.sql": {
		classes: []string{classAlterColumnSetNotNull, classDML},
		reason:  beforeGuardrail,
	},
	"00043_staff_work_state.sql": {
		classes: []string{classAddConstraintCheck, classAlterColumnSetNotNull, classDML},
		reason:  beforeGuardrail,
	},
	"00046_practice_page_slug.sql": {
		classes: []string{classAddConstraintCheck, classCreateUniqueIndex, classDML, classDoBlock},
		reason:  beforeGuardrail,
	},
	"00049_site_build_and_page_liveness.sql": {
		classes: []string{classAddConstraintCheck, classDML},
		reason:  beforeGuardrail,
	},
	"00052_credit_lot_provenance.sql": {
		classes: []string{classAddConstraintCheck, classAlterColumnType, classDML},
		reason:  beforeGuardrail,
	},
	"00054_one_refund_per_request.sql": {
		classes: []string{classAddConstraintCheck, classCreateUniqueIndex},
		reason:  beforeGuardrail,
	},
	"00055_founding_grant.sql": {
		classes: []string{classAddConstraintCheck, classAlterColumnType, classCreateUniqueIndex},
		reason:  beforeGuardrail,
	},
	"00057_engagement_status_drop_postpartum.sql": {
		classes: []string{classAlterColumnType},
		reason:  beforeGuardrail,
	},
	"00063_mfa_recovery_cleared_notice.sql": {classes: []string{classCreateUniqueIndex}, reason: beforeGuardrail},
	"00072_totp_mfa_auth_events.sql":        {classes: []string{classAddConstraintCheck}, reason: beforeGuardrail},
	"00073_portal_accounts.sql": {
		classes: []string{classAddConstraintForeignKey, classDML},
		reason:  beforeGuardrail,
	},
	"00075_retire_identity_account_delete.sql": {
		classes: []string{classAlterColumnType, classDML},
		reason:  beforeGuardrail,
	},
	"00078_session_evicted_one_pending.sql":  {classes: []string{classCreateUniqueIndex}, reason: beforeGuardrail},
	"00089_credit_ledger_forfeit_shape.sql":  {classes: []string{classAddConstraintCheck}, reason: beforeGuardrail},
	"00090_engagement_status_transition.sql": {classes: []string{classAddConstraintCheck}, reason: beforeGuardrail},
	"00093_engagement_birth_outcome.sql":     {classes: []string{classAddConstraintCheck}, reason: beforeGuardrail},
	"00094_engagement_completion_requires_outcome.sql": {
		classes: []string{classAddConstraintCheck, classDML},
		reason:  beforeGuardrail,
	},
	"00095_manual_payment_recording.sql": {
		classes: []string{classAddColumnNotNullWithoutDefault, classAddConstraintCheck},
		reason:  "applied 2026-09-08 against an empty invoices",
	},
	"00101_staff_login_deletion_rules.sql": {classes: []string{classAddConstraintCheck}, reason: beforeGuardrail},
	"00103_payment_reversal.sql": {
		classes: []string{classAddConstraintCheck, classCreateUniqueIndex},
		reason:  beforeGuardrail,
	},
	"00112_client_merge_moves_history.sql": {
		classes: []string{classAddConstraintCheck, classDML},
		reason:  beforeGuardrail,
	},
}

// TestNoRowDependentStatementWithoutASafetyNote is the guardrail. It
// fails any migration whose Up section contains a statement that only
// existing rows can refuse, unless safety/<migration>.md explains why
// those rows cannot refuse it.
func TestNoRowDependentStatementWithoutASafetyNote(t *testing.T) {
	checked := 0
	for _, name := range migrationFiles(t) {
		checked++
		findings := RowDependent(UpSection(readMigration(t, name)))
		if len(findings) == 0 {
			continue
		}
		quoted := safetyNoteStatements(t, name)
		for _, f := range findings {
			if statementQuoted(quoted[f.Class], f.Statement) {
				continue
			}
			if entry, ok := grandfathered[name]; ok && classNamed(entry.classes, f.Class) {
				t.Logf("%s: grandfathered (%s: %s)", name, f.Class, entry.reason)
				continue
			}
			t.Error(violation(name, f))
		}
	}
	if checked == 0 {
		t.Fatal("no migrations found to check -- the embed pattern or this test's filter is wrong")
	}
}

// statementQuoted reports whether statement (already normalized, as
// Finding.Statement is) appears among quoted (already normalized by
// safetyNoteStatements).
func statementQuoted(quoted []string, statement string) bool {
	return slices.Contains(quoted, statement)
}

// classNamed reports whether class is one of the classes a grandfathered
// entry names.
func classNamed(classes []string, class string) bool {
	return slices.Contains(classes, class)
}

// fence is the markdown code-fence marker a safety note quotes a
// statement inside. It cannot be written into the raw string below --
// Go's raw strings have no escape for a backtick -- so it is
// interpolated instead.
const fence = "```"

// violation is the failure message: the statement, the trunk-only
// failure it risks, what to write instead, and exactly what to quote in
// a safety note to excuse this one statement.
func violation(name string, f Finding) string {
	return fmt.Sprintf(`%s carries a row-dependent statement -- %s:

    %s

This passes every PR (testdb builds an empty database, so no existing row
can refuse it) and fails the first time trunk's migrate job applies it to
doula-cloud-pg, which has rows -- taking deploy-api and deploy-app down
with it. What it hits there:

    %s

Write it this way instead:

    %s

If the statement is already safe, attest to it in
api/db/migrations/safety/%s.md under a "## %s" heading, inside a fenced
%ssql block quoting exactly this statement (case and whitespace do not
have to match, everything else does):

    %ssql
    %s
    %s

That block excuses only this statement -- a second %s statement in the
same migration needs a block of its own. See #1021, #1139 and the
package doc in embed.go.`,
		name, f.Class, f.Statement, f.Failure, f.Remedy,
		strings.TrimSuffix(name, ".sql"), f.Class, fence, fence, f.Statement, fence, f.Class)
}

// safetyNoteClasses returns the classes safety/<migration>.md attests
// to, keyed by the guardrail's own name for each class.
func safetyNoteClasses(t *testing.T, name string) map[string]bool {
	t.Helper()
	body, err := safetyFS.ReadFile(safetyNotePath(name))
	if err != nil {
		return nil
	}
	covered := map[string]bool{}
	for line := range strings.SplitSeq(string(body), "\n") {
		if heading, ok := strings.CutPrefix(strings.TrimSpace(line), "## "); ok {
			covered[strings.TrimSpace(heading)] = true
		}
	}
	return covered
}

// safetyNoteStatements returns, for each class heading in
// safety/<migration>.md, the statements a fenced ```sql block under that
// heading quotes -- normalized the same way RowDependent normalizes a
// Finding.Statement (see normalizeStatement), so the two compare with
// plain equality. A heading with no block, or a statement outside any
// heading, excuses nothing: only a quoted block under its own class
// narrows the exemption to the one statement it names.
func safetyNoteStatements(t *testing.T, name string) map[string][]string {
	t.Helper()
	body, err := safetyFS.ReadFile(safetyNotePath(name))
	if err != nil {
		return nil
	}
	quoted := map[string][]string{}
	class := ""
	var block *strings.Builder
	for line := range strings.SplitSeq(string(body), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "## "):
			class = strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
		case trimmed == fence+"sql":
			block = &strings.Builder{}
		case trimmed == fence && block != nil:
			quoted[class] = append(quoted[class], normalizeStatement(block.String()))
			block = nil
		case block != nil:
			block.WriteString(line)
			block.WriteString(" ")
		}
	}
	return quoted
}

// normalizeStatement collapses a quoted statement onto one line,
// upper-cases it and drops a trailing semicolon -- exactly how
// RowDependent builds Finding.Statement, plus the semicolon a block
// copied straight out of a migration's SQL still carries.
func normalizeStatement(s string) string {
	return strings.TrimSuffix(strings.ToUpper(collapse(s)), ";")
}

// safetyNotePath is where a migration's safety note lives.
func safetyNotePath(name string) string {
	return "safety/" + strings.TrimSuffix(name, ".sql") + ".md"
}

// TestSafetyNotesAreStillNeeded keeps a note from outliving the
// statement it justifies: every heading in every note must name a class
// the guardrail still reports on that migration, and every statement a
// heading quotes must be one the guardrail still reports for that class
// -- a stale quote, like a stale heading, fails the build rather than
// silently covering a statement that no longer exists.
func TestSafetyNotesAreStillNeeded(t *testing.T) {
	entries, err := safetyFS.ReadDir("safety")
	if err != nil {
		t.Fatalf("read safety notes: %v", err)
	}
	notes := 0
	for _, e := range entries {
		if e.Name() == "README.md" {
			continue
		}
		notes++
		migration := strings.TrimSuffix(e.Name(), ".md") + ".sql"
		raised := map[string][]string{}
		for _, f := range RowDependent(UpSection(readMigration(t, migration))) {
			raised[f.Class] = append(raised[f.Class], f.Statement)
		}
		for class := range safetyNoteClasses(t, migration) {
			if len(raised[class]) == 0 {
				t.Errorf("%s attests to %q, which %s no longer raises; drop the heading", e.Name(), class, migration)
			}
		}
		for class, statements := range safetyNoteStatements(t, migration) {
			for _, q := range statements {
				if !statementQuoted(raised[class], q) {
					t.Errorf("%s quotes a %q statement %s no longer contains: %s", e.Name(), class, migration, q)
				}
			}
		}
	}
	if notes == 0 {
		t.Fatal("no safety notes found -- the embed pattern is wrong")
	}
}

// TestGrandfatheredListDoesNotGrow keeps the exemption list from
// becoming a habit: every name in it must still exist and must still be
// an offender, every class an entry names must still be one the
// migration raises, so a migration that gets fixed or deleted -- or
// narrowed to fewer classes -- is trimmed from the list rather than left
// as cover for a future one.
func TestGrandfatheredListDoesNotGrow(t *testing.T) {
	const want = 27
	if len(grandfathered) != want {
		t.Fatalf("grandfathered has %d entries, want exactly %d -- a new migration takes the safe form or writes a safety note, not an exemption", len(grandfathered), want)
	}
	for name, entry := range grandfathered {
		findings := RowDependent(UpSection(readMigration(t, name)))
		if len(findings) == 0 {
			t.Errorf("grandfathered migration %s no longer carries a row-dependent statement; drop it from the list", name)
			continue
		}
		raised := map[string]bool{}
		for _, f := range findings {
			raised[f.Class] = true
		}
		for _, class := range entry.classes {
			if !raised[class] {
				t.Errorf("grandfathered %s names class %q, which it no longer raises; drop the class from its entry", name, class)
			}
		}
	}
}

// migrationFiles lists the embedded migrations, newest last.
func migrationFiles(t *testing.T) []string {
	t.Helper()
	entries, err := FS.ReadDir(".")
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	return names
}

// readMigration returns one embedded migration's text.
func readMigration(t *testing.T, name string) string {
	t.Helper()
	body, err := FS.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(body)
}
