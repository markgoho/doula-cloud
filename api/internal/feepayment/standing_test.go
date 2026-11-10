package feepayment_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"doula-cloud/api/internal/testdb"
)

// violates reports whether err is a refusal naming constraint.
func violates(err error, constraint string) bool {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	return ok && pgErr.ConstraintName == constraint
}

// TestFeePaymentsRefuseWhatNoActWrites pins 00175's Fee Payment trigger:
// the rows RecordHandler and ReverseHandler refuse before writing are
// refused by the database too, naming the rule.
func TestFeePaymentsRefuseWhatNoActWrites(t *testing.T) {
	f := newFixture(t)
	view := f.recordPaid(t)
	_, otherEngagement := testdb.SeedEngagement(t, f.db, f.practiceID)
	other := seedAcceptedOffer(t, f.db, otherEngagement, f.lenaID, feeCents)

	cases := []struct {
		name       string
		query      string
		args       []any
		constraint string
	}{
		{"a second record on a paid fee",
			`INSERT INTO attachment_fee_payments (practice_id, attachment_id, kind, amount_cents, method, paid_on, recorded_by, recorded_at)
			 VALUES ($1, $2, 'recorded', 100, 'cash', '2026-10-01', $3, now())`,
			[]any{f.practiceID, f.attachmentID, f.lenaID}, "attachment_fee_payments_one_standing_per_attachment"},
		{"a reversal on another Attachment",
			`INSERT INTO attachment_fee_payments (practice_id, attachment_id, kind, amount_cents, target_id, reason, recorded_by, recorded_at)
			 VALUES ($1, $2, 'reversal', -100, $3, 'no', $4, now())`,
			[]any{f.practiceID, other, view.ID, f.lenaID}, "attachment_fee_payments_reversal_of_a_recorded_payment"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.db.Admin.ExecContext(t.Context(), tc.query, tc.args...)
			if !violates(err, tc.constraint) {
				t.Fatalf("insert err = %v, want a violation of %s", err, tc.constraint)
			}
		})
	}

	reversal := f.reverse(t, view.ID)
	if reversal.status != http.StatusCreated {
		t.Fatalf("reverse status = %d, want 201", reversal.status)
	}
	var reversalID string
	if err := f.db.Admin.QueryRowContext(t.Context(),
		`SELECT id FROM attachment_fee_payments WHERE kind = 'reversal' AND attachment_id = $1`, f.attachmentID,
	).Scan(&reversalID); err != nil {
		t.Fatalf("read reversal: %v", err)
	}
	if _, err := f.db.Admin.ExecContext(t.Context(),
		`INSERT INTO attachment_fee_payments (practice_id, attachment_id, kind, amount_cents, target_id, reason, recorded_by, recorded_at)
		 VALUES ($1, $2, 'reversal', -100, $3, 'no', $4, now())`,
		f.practiceID, f.attachmentID, reversalID, f.lenaID); !violates(err, "attachment_fee_payments_reversal_of_a_recorded_payment") {
		t.Fatalf("reversal of a reversal err = %v, want it refused", err)
	}
}
