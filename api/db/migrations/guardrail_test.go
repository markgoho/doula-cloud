package migrations

import (
	"embed"
	"fmt"
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

// The reasons that repeat across the list below, each named once rather
// than written out five times.
const (
	uniqueIndexBefore = "CREATE UNIQUE INDEX; already applied to doula-cloud-pg before the guardrail covered the class (#1139)"
	checkBefore       = "ADD CONSTRAINT ... CHECK; already applied to doula-cloud-pg before the guardrail covered the class (#1139)"
	checkAndDMLBefore = "ADD CONSTRAINT ... CHECK and DML; already applied to doula-cloud-pg before the guardrail covered the classes (#1139)"
)

// grandfathered are migrations that carry a row-dependent statement,
// have already applied against doula-cloud-pg, and cannot be rewritten:
// goose has recorded them, so editing one makes the file and the
// database silently disagree. Each entry names the class it carries and
// says why it is on the list. The list is closed --
// TestGrandfatheredListDoesNotGrow pins its exact size, so a new
// migration takes the safe form or writes a safety note, never an
// exemption.
//
//nolint:gosec // G101 reads the constraint classes below as a possible credential; every value is a filename and a sentence of English, and nothing in this package holds a secret
var grandfathered = map[string]string{
	"00002_practice_staff_tenancy.sql":                 "DO block; already applied to doula-cloud-pg before the guardrail covered the class (#1139)",
	"00020_contracts_recreate_after_void.sql":          uniqueIndexBefore,
	"00026_client_portal_provisioning.sql":             uniqueIndexBefore,
	"00029_stripe_connect_accounts_v2.sql":             "ADD COLUMN ... DEFAULT with an inline CHECK the default satisfies; already applied to doula-cloud-pg before the guardrail covered the class (#1139)",
	"00030_employment_attachment_offer.sql":            "ADD COLUMN ... NOT NULL without DEFAULT; applied 2026-06 against an empty staff_practices",
	"00039_membership_events.sql":                      uniqueIndexBefore,
	"00042_client_intake_schema.sql":                   "ALTER COLUMN ... SET NOT NULL and DML; already applied to doula-cloud-pg before the guardrail covered the classes (#1139)",
	"00043_staff_work_state.sql":                       "ADD CONSTRAINT ... CHECK, ALTER COLUMN ... SET NOT NULL and DML; already applied to doula-cloud-pg before the guardrail covered the classes (#1139)",
	"00046_practice_page_slug.sql":                     "ADD CONSTRAINT ... CHECK, CREATE UNIQUE INDEX, DML and a DO block; already applied to doula-cloud-pg before the guardrail covered the classes (#1139)",
	"00049_site_build_and_page_liveness.sql":           checkAndDMLBefore,
	"00052_credit_lot_provenance.sql":                  "ADD CONSTRAINT ... CHECK, ALTER COLUMN ... TYPE and DML; already applied to doula-cloud-pg before the guardrail covered the classes (#1139)",
	"00054_one_refund_per_request.sql":                 "ADD CONSTRAINT ... CHECK and CREATE UNIQUE INDEX; already applied to doula-cloud-pg before the guardrail covered the classes (#1139)",
	"00055_founding_grant.sql":                         "ADD CONSTRAINT ... CHECK, ALTER COLUMN ... TYPE and CREATE UNIQUE INDEX; already applied to doula-cloud-pg before the guardrail covered the classes (#1139)",
	"00057_engagement_status_drop_postpartum.sql":      "ALTER COLUMN ... TYPE; already applied to doula-cloud-pg before the guardrail covered the class (#1139)",
	"00063_mfa_recovery_cleared_notice.sql":            uniqueIndexBefore,
	"00072_totp_mfa_auth_events.sql":                   checkBefore,
	"00073_portal_accounts.sql":                        "ADD CONSTRAINT ... FOREIGN KEY, CREATE UNIQUE INDEX and DML; already applied to doula-cloud-pg before the guardrail covered the classes (#1139)",
	"00075_retire_identity_account_delete.sql":         "ALTER COLUMN ... TYPE; already applied to doula-cloud-pg before the guardrail covered the class (#1139)",
	"00078_session_evicted_one_pending.sql":            uniqueIndexBefore,
	"00089_credit_ledger_forfeit_shape.sql":            checkBefore,
	"00090_engagement_status_transition.sql":           checkBefore,
	"00093_engagement_birth_outcome.sql":               checkBefore,
	"00094_engagement_completion_requires_outcome.sql": checkAndDMLBefore,
	"00095_manual_payment_recording.sql":               "ADD COLUMN ... NOT NULL without DEFAULT; applied 2026-09-08 against an empty invoices",
	"00101_staff_login_deletion_rules.sql":             checkBefore,
	"00103_payment_reversal.sql":                       "ADD CONSTRAINT ... CHECK and CREATE UNIQUE INDEX; already applied to doula-cloud-pg before the guardrail covered the classes (#1139)",
	"00112_client_merge_moves_history.sql":             checkAndDMLBefore,
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
		covered := safetyNoteClasses(t, name)
		for _, f := range findings {
			if covered[f.Class] {
				continue
			}
			if reason, ok := grandfathered[name]; ok {
				t.Logf("%s: grandfathered (%s)", name, reason)
				continue
			}
			t.Error(violation(name, f))
		}
	}
	if checked == 0 {
		t.Fatal("no migrations found to check -- the embed pattern or this test's filter is wrong")
	}
}

// violation is the failure message: the statement, the trunk-only
// failure it risks, and what to write instead.
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

If the statement is already safe, say why in
api/db/migrations/safety/%s.md under a "## %s" heading. See #1021, #1139
and the package doc in embed.go.`,
		name, f.Class, f.Statement, f.Failure, f.Remedy,
		strings.TrimSuffix(name, ".sql"), f.Class)
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

// safetyNotePath is where a migration's safety note lives.
func safetyNotePath(name string) string {
	return "safety/" + strings.TrimSuffix(name, ".sql") + ".md"
}

// TestSafetyNotesAreStillNeeded keeps a note from outliving the
// statement it justifies: every heading in every note must name a class
// the guardrail still reports on that migration.
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
		raised := map[string]bool{}
		for _, f := range RowDependent(UpSection(readMigration(t, migration))) {
			raised[f.Class] = true
		}
		for class := range safetyNoteClasses(t, migration) {
			if !raised[class] {
				t.Errorf("%s attests to %q, which %s no longer raises; drop the heading", e.Name(), class, migration)
			}
		}
	}
	if notes == 0 {
		t.Fatal("no safety notes found -- the embed pattern is wrong")
	}
}

// TestGrandfatheredListDoesNotGrow keeps the exemption list from
// becoming a habit: every name in it must still exist and must still be
// an offender, so a migration that gets fixed or deleted is removed from
// the list rather than left as cover for a future one.
func TestGrandfatheredListDoesNotGrow(t *testing.T) {
	const want = 27
	if len(grandfathered) != want {
		t.Fatalf("grandfathered has %d entries, want exactly %d -- a new migration takes the safe form or writes a safety note, not an exemption", len(grandfathered), want)
	}
	for name := range grandfathered {
		if len(RowDependent(UpSection(readMigration(t, name)))) == 0 {
			t.Errorf("grandfathered migration %s no longer carries a row-dependent statement; drop it from the list", name)
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
