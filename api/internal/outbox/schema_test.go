package outbox_test

import (
	"slices"
	"strings"
	"testing"

	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/testdb"
)

// TestEverySharedOutboxHoldsTheDeclaredStates ties lifecycle.go's
// declaration to the schema it describes. Every table whose status enum
// can hold 'skipped' -- found by the enum, not by name -- holds exactly
// the declared labels (portal_invite_outbox alone also bounced and
// complained), carries skip_reason as the shared outbox_skip_reason and
// skipped_at, and has the <table>_status_shape CHECK that ties each state
// to its columns. No CHECK on any outbox is left NOT VALID.
func TestEverySharedOutboxHoldsTheDeclaredStates(t *testing.T) {
	db := testdb.New(t)

	rows, err := db.Admin.QueryContext(t.Context(), `
		SELECT c.table_name,
		       (SELECT string_agg(e.enumlabel, ',' ORDER BY e.enumsortorder) FROM pg_enum e WHERE e.enumtypid = ty.oid),
		       COALESCE((SELECT k.udt_name FROM information_schema.columns k
		                  WHERE k.table_schema = 'public' AND k.table_name = c.table_name AND k.column_name = 'skip_reason'), ''),
		       EXISTS (SELECT 1 FROM information_schema.columns k
		                WHERE k.table_schema = 'public' AND k.table_name = c.table_name AND k.column_name = 'skipped_at'),
		       EXISTS (SELECT 1 FROM pg_constraint con JOIN pg_class rel ON rel.oid = con.conrelid
		                WHERE rel.relname = c.table_name AND con.conname = c.table_name || '_status_shape' AND con.convalidated)
		  FROM information_schema.columns c
		  JOIN pg_type ty ON ty.typname = c.udt_name
		 WHERE c.table_schema = 'public' AND c.column_name = 'status'
		   AND EXISTS (SELECT 1 FROM pg_enum e WHERE e.enumtypid = ty.oid AND e.enumlabel = 'skipped')
		 ORDER BY c.table_name`)
	if err != nil {
		t.Fatalf("query outbox tables: %v", err)
	}
	defer func() { _ = rows.Close() }()

	shared := []string{string(outbox.StatusPending), string(outbox.StatusSent), string(outbox.StatusSkipped), string(outbox.StatusDeadLettered)}
	seen := 0
	for rows.Next() {
		var table, labels, reasonType string
		var hasSkippedAt, shaped bool
		if err := rows.Scan(&table, &labels, &reasonType, &hasSkippedAt, &shaped); err != nil {
			t.Fatalf("scan: %v", err)
		}
		seen++
		want := shared
		if table == "portal_invite_outbox" {
			want = append(slices.Clone(shared), string(outbox.StatusBounced), string(outbox.StatusComplained))
		}
		got := strings.Split(labels, ",")
		slices.Sort(got)
		want = slices.Sorted(slices.Values(want))
		if !slices.Equal(got, want) {
			t.Errorf("%s status labels = %v, want the declared %v", table, got, want)
		}
		if reasonType != "outbox_skip_reason" {
			t.Errorf("%s.skip_reason type = %q, want outbox_skip_reason", table, reasonType)
		}
		if !hasSkippedAt {
			t.Errorf("%s has no skipped_at", table)
		}
		if !shaped {
			t.Errorf("%s has no validated %s_status_shape CHECK", table, table)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate: %v", err)
	}
	if seen != 21 {
		t.Errorf("found %d outbox tables on the shared lifecycle, want 21 (every outbox but site_build_outbox)", seen)
	}

	var unvalidated string
	if err := db.Admin.QueryRowContext(t.Context(), `
		SELECT COALESCE(string_agg(con.conname, ', '), '')
		  FROM pg_constraint con JOIN pg_class rel ON rel.oid = con.conrelid
		 WHERE rel.relname LIKE '%\_outbox' AND NOT con.convalidated`).Scan(&unvalidated); err != nil {
		t.Fatalf("query unvalidated constraints: %v", err)
	}
	if unvalidated != "" {
		t.Errorf("outbox constraints left NOT VALID: %s", unvalidated)
	}
}

// TestSkipReasonsMatchTheSchema holds the Go declaration of SkipReason to
// the outbox_skip_reason enum, label for label.
func TestSkipReasonsMatchTheSchema(t *testing.T) {
	db := testdb.New(t)
	var labels string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT string_agg(enumlabel, ',' ORDER BY enumsortorder) FROM pg_enum WHERE enumtypid = 'outbox_skip_reason'::regtype`,
	).Scan(&labels); err != nil {
		t.Fatalf("read outbox_skip_reason: %v", err)
	}
	declared := []outbox.SkipReason{
		outbox.SkipAlreadyResolved, outbox.SkipNoRecipients, outbox.SkipLoginDeleted, outbox.SkipPracticeRestored,
		outbox.SkipEngagementStarted, outbox.SkipFoundingGrant, outbox.SkipPracticeDeleting, outbox.SkipOwnersSuppressed,
	}
	want := make([]string, len(declared))
	for i, r := range declared {
		want[i] = string(r)
	}
	if got := strings.Split(labels, ","); !slices.Equal(got, want) {
		t.Errorf("outbox_skip_reason = %v, want the declared %v", got, want)
	}
}
