package billing_test

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/testdb"
)

// forfeit runs billing.Forfeit for practiceID the way a Practice's
// deletion does: on an app_runtime transaction scoped to it.
func forfeit(t *testing.T, db *testdb.DB, practiceID string, now time.Time) int {
	t.Helper()
	tx := practiceTx(t, db, practiceID)
	n, err := billing.Forfeit(t.Context(), tx, practiceID, now)
	if err != nil {
		t.Fatalf("Forfeit: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return n
}

// requireLotsSumToBalance is the invariant 00176 exists for: every Credit
// that leaves the ledger names the lot it left, so what credit_lots says
// is left in a Practice's lots is, to the Credit, its balance.
func requireLotsSumToBalance(t *testing.T, db *testdb.DB, practiceID string, step string, want int) {
	t.Helper()
	var balance, remaining int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT (SELECT COALESCE(SUM(quantity), 0) FROM credit_ledger WHERE practice_id = $1),
		        (SELECT COALESCE(SUM(remaining), 0) FROM credit_lots($1, now()))`, practiceID,
	).Scan(&balance, &remaining); err != nil {
		t.Fatalf("%s: read balance and lots: %v", step, err)
	}
	if balance != remaining || balance != want {
		t.Fatalf("%s: balance %d, lots hold %d, want both %d", step, balance, remaining, want)
	}
}

// TestLots_SumToTheBalanceThroughEveryAct walks one Practice through
// every way a Credit arrives and leaves -- a grant, a founding grant, two
// purchases, consumption across lots, a refund and the deletion's
// forfeit -- and holds the lots to the balance after each.
func TestLots_SumToTheBalanceThroughEveryAct(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Every Act")
	now := time.Now()
	seedSignupBonus(t, db, practiceID)
	requireLotsSumToBalance(t, db, practiceID, "after the signup bonus", 3)
	testdb.SeedFoundingGrant(t, db, practiceID, 2, now.Add(-3*time.Hour))
	requireLotsSumToBalance(t, db, practiceID, "after the founding grant", 5)
	seedPurchase(t, db, practiceID, 4, 2000, 100, "pi_every_one", now.Add(-2*time.Hour))
	requireLotsSumToBalance(t, db, practiceID, "after the first purchase", 9)

	// The signup bonus is stamped by the database's clock, so it is the
	// newest lot: FIFO drains the founding grant, then the purchase.
	consume(t, db, practiceID, seedEngagements(t, db, practiceID, 4))
	requireLotsSumToBalance(t, db, practiceID, "after four consumptions across two lots", 5)

	r := newRefunder(t, db, billing.NewFakeStripeClient())
	if _, err := r.refund(practiceID, "req-every", 1, now); err != nil {
		t.Fatalf("Refund: %v", err)
	}
	r.commit()
	requireLotsSumToBalance(t, db, practiceID, "after a refund", 4)

	seedPurchase(t, db, practiceID, 3, 2000, 0, "pi_every_two", now.Add(-time.Hour))
	consume(t, db, practiceID, seedEngagements(t, db, practiceID, 1))
	requireLotsSumToBalance(t, db, practiceID, "after a second purchase and a consumption", 6)

	if n := forfeit(t, db, practiceID, now); n != 6 {
		t.Fatalf("forfeited %d, want 6", n)
	}
	requireLotsSumToBalance(t, db, practiceID, "after the forfeit", 0)
	if open := testdb.Scalar[int](t, db.Admin,
		`SELECT count(*) FROM credit_lots($1, now()) WHERE state <> 'drained'`, practiceID); open != 0 {
		t.Fatalf("%d lots still open after the forfeit, want none", open)
	}
	// One row per lot it drew on, each naming that lot: the second
	// purchase and the signup bonus; the first purchase was already
	// drained by consumption and the refund.
	if rows := testdb.Scalar[int](t, db.Admin,
		`SELECT count(*) FROM credit_ledger WHERE practice_id = $1 AND origin = 'forfeit' AND drawn_lot_id IS NOT NULL`, practiceID); rows != 2 {
		t.Fatalf("forfeit rows = %d, want 2, one per open lot", rows)
	}
}

// TestForfeit_LeavesNothingToRefundOrConsume is the audit's finding,
// reproduced before 00176 and closed by it: a forfeit written as one row
// naming no lot took the balance to zero and left every lot open, so ops
// refunded two Credits and a consumption spent a third, and the balance
// read -3.
func TestForfeit_LeavesNothingToRefundOrConsume(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Forfeited")
	now := time.Now()
	seedPurchase(t, db, practiceID, 4, 2000, 0, "pi_forfeited", now.AddDate(0, -1, 0))
	engagements := seedEngagements(t, db, practiceID, 1)

	if n := forfeit(t, db, practiceID, now); n != 4 {
		t.Fatalf("forfeited %d, want 4", n)
	}

	r := newRefunder(t, db, billing.NewFakeStripeClient())
	if _, err := r.refund(practiceID, "req-after-forfeit", 2, now); !errors.Is(err, billing.ErrNothingRefundable) {
		t.Fatalf("Refund after forfeit err = %v, want ErrNothingRefundable", err)
	}
	// The refund's transaction holds the Practice row's lock, as
	// ConsumeCredit's will.
	_ = r.tx.Rollback()
	tx := practiceTx(t, db, practiceID)
	if err := billing.ConsumeCredit(t.Context(), tx, practiceID, engagements[0]); !errors.Is(err, billing.ErrNoCreditsRemaining) {
		t.Fatalf("ConsumeCredit after forfeit err = %v, want ErrNoCreditsRemaining", err)
	}
	requireLotsSumToBalance(t, db, practiceID, "after the refused draws", 0)
}

func TestForfeit_WritesNothingWhenNothingIsLeft(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Nothing Left")
	if n := forfeit(t, db, practiceID, time.Now()); n != 0 {
		t.Fatalf("forfeited %d, want 0", n)
	}
	if rows := ledgerRowCount(t, db, practiceID); rows != 0 {
		t.Fatalf("ledger rows = %d, want none", rows)
	}
}

// TestDraws_StampTheClockSeam: a consumption and a forfeit both carry the
// instant the request's clock reads, not the database's.
func TestDraws_StampTheClockSeam(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Clock Seam")
	seedSignupBonus(t, db, practiceID)
	at := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	engagements := seedEngagements(t, db, practiceID, 1)

	tx := practiceTx(t, db, practiceID)
	ctx := clock.Into(t.Context(), func() time.Time { return at })
	if err := billing.ConsumeCredit(ctx, tx, practiceID, engagements[0]); err != nil {
		t.Fatalf("ConsumeCredit: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	forfeit(t, db, practiceID, at.Add(time.Hour))

	var consumedAt, consumptionAt, forfeitAt time.Time
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT c.consumed_at, c.created_at, f.created_at
		   FROM credit_ledger c, credit_ledger f
		  WHERE c.practice_id = $1 AND c.origin = 'consumption' AND f.practice_id = $1 AND f.origin = 'forfeit'`, practiceID,
	).Scan(&consumedAt, &consumptionAt, &forfeitAt); err != nil {
		t.Fatalf("read draws: %v", err)
	}
	if !consumedAt.Equal(at) || !consumptionAt.Equal(at) || !forfeitAt.Equal(at.Add(time.Hour)) {
		t.Fatalf("consumed_at %s, consumption created_at %s, forfeit created_at %s; want %s, %s, %s",
			consumedAt, consumptionAt, forfeitAt, at, at, at.Add(time.Hour))
	}
}

// TestDraws_SchemaRefusesWhatNoActWrites proves 00176's CHECK and trigger
// at the database, whoever writes: a draw past what its lot holds, a draw
// on another Practice's lot or on a row that is not a lot, and a forfeit
// that names no lot.
func TestDraws_SchemaRefusesWhatNoActWrites(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Schema Refusals")
	otherID := testdb.SeedPractice(t, db, "Someone Else")
	lotID := testdb.Scalar[string](t, db.Admin,
		`INSERT INTO credit_ledger (practice_id, origin, quantity) VALUES ($1, 'signup_bonus', 1) RETURNING id`, practiceID)
	otherLotID := seedSignupBonus(t, db, otherID)
	drawID := testdb.Scalar[string](t, db.Admin,
		`INSERT INTO credit_ledger (practice_id, origin, quantity, drawn_lot_id) VALUES ($1, 'forfeit', -1, $2) RETURNING id`, practiceID, lotID)

	const withinLot = "credit_ledger_draw_within_lot"
	for name, c := range map[string]struct {
		lot, constraint string
	}{
		"a draw past what the lot holds":    {lotID, withinLot},
		"a draw on another Practice's lot":  {otherLotID, withinLot},
		"a draw on a row that is not a lot": {drawID, withinLot},
		"a forfeit that names no lot":       {"", "credit_ledger_lot_or_draw"},
	} {
		t.Run(name, func(t *testing.T) {
			var lot any
			if c.lot != "" {
				lot = c.lot
			}
			_, err := db.Admin.ExecContext(t.Context(),
				`INSERT INTO credit_ledger (practice_id, origin, quantity, drawn_lot_id) VALUES ($1, 'forfeit', -1, $2)`, practiceID, lot)
			if err == nil || !strings.Contains(err.Error(), c.constraint) {
				t.Fatalf("insert = %v, want a %s refusal", err, c.constraint)
			}
		})
	}
}

// TestRecordRefund_RefusesALotOutsideTheWindow: ops_record_refund decides
// by the same window the quote does, so a refund row cannot be written
// against a purchase past it, or against a grant, by any caller of the
// function.
func TestRecordRefund_RefusesALotOutsideTheWindow(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Window At The Boundary")
	now := time.Now()
	staleID := seedPurchase(t, db, practiceID, 2, 2000, 0, "pi_window_stale", now.AddDate(-billing.RefundWindowYears, 0, -1))
	grantID := seedSignupBonus(t, db, practiceID)
	actID := testdb.SeedOperatorAct(t, db, "Mark Goho")

	for name, lotID := range map[string]string{"a purchase past the window": staleID, "a grant": grantID} {
		t.Run(name, func(t *testing.T) {
			_, err := opsTx(t, db).ExecContext(t.Context(),
				`SELECT ops_record_refund($1, $2, 1, 0, $3, $3, $4, $5)`, practiceID, lotID, "re_"+lotID, actID, now)
			if err == nil || !strings.Contains(err.Error(), "is not a refundable purchase") {
				t.Fatalf("ops_record_refund = %v, want a refusal", err)
			}
		})
	}
}

// TestRefundWindow_GoAndSQLAgree holds RefundWindowYears to
// credit_lot_refund_closes_at, the SQL that decides the window.
func TestRefundWindow_GoAndSQLAgree(t *testing.T) {
	db := testdb.New(t)
	bought := time.Date(2026, 11, 10, 15, 4, 5, 0, time.UTC)
	closes := testdb.Scalar[time.Time](t, db.Admin, `SELECT credit_lot_refund_closes_at($1)`, bought)
	if want := bought.AddDate(billing.RefundWindowYears, 0, 0); !closes.Equal(want) {
		t.Fatalf("credit_lot_refund_closes_at(%s) = %s, want %s", bought, closes, want)
	}
}

func TestLotActs_TableIsComplete(t *testing.T) {
	origins := []string{}
	for _, a := range billing.LotActs {
		origins = append(origins, a.Origin)
		if a.Requires != billing.LotOpen {
			t.Errorf("%s requires %q, want every act to draw only on an open lot", a.Origin, a.Requires)
		}
	}
	// Every origin 00176's credit_ledger_lot_or_draw admits as a draw.
	if want := []string{"consumption", "forfeit", "refund"}; !slices.Equal(slices.Sorted(slices.Values(origins)), want) {
		t.Errorf("LotActs origins = %v, want %v", origins, want)
	}
	if !billing.ActRefund.RefundableOnly || billing.ActConsume.RefundableOnly || billing.ActForfeit.RefundableOnly {
		t.Error("only a refund is narrowed to refundable lots")
	}
}

// writesDraw is an INSERT into credit_ledger that names a lot, up to the
// end of its backquoted SQL.
var writesDraw = regexp.MustCompile("INSERT INTO credit_ledger[^`]*drawn_lot_id[^`]*")

// writesDrawOrigin is an INSERT into credit_ledger spelling a draw origin
// out as a literal rather than taking it from a LotAct.
var writesDrawOrigin = regexp.MustCompile("INSERT INTO credit_ledger[^`]*'(consumption|refund|forfeit)'")

// derivesRemaining is a reader summing a lot's draws for itself rather
// than reading credit_lots.
var derivesRemaining = regexp.MustCompile(`drawn_lot_id\s*=\s*\w+\.id`)

// TestLotActs_EveryDrawGoesThroughAnAct is the source-scan guardrail
// contracts' lifecycle_test.go keeps for a Contract, for a lot: across
// api/internal and api/cmd, only this package writes a draw, each file
// that does takes its lots from a LotAct and its origin from the same
// act, the refund (whose row ops_record_refund writes) takes its lot from
// ActRefund, and nothing outside SQL derives a lot's remainder.
func TestLotActs_EveryDrawGoesThroughAnAct(t *testing.T) {
	billingDir, err := filepath.Abs(".")
	if err != nil {
		// coverage:ignore reason: a path inside this module always resolves while tests run
		t.Fatalf("resolve package directory: %v", err)
	}
	writers := 0
	scanned := 0
	for _, root := range []string{"..", filepath.Join("..", "..", "cmd")} {
		tree := os.DirFS(root)
		err := fs.WalkDir(tree, ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				// coverage:ignore reason: a directory WalkDir just listed
				return fmt.Errorf("walk %s: %w", path, err)
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			raw, err := fs.ReadFile(tree, path)
			if err != nil {
				// coverage:ignore reason: a file WalkDir listed a moment ago
				return fmt.Errorf("read %s: %w", path, err)
			}
			src := string(raw)
			scanned++
			if m := writesDrawOrigin.FindString(src); m != "" {
				t.Errorf("%s writes a draw origin as a literal (%q) -- take it from a billing.LotAct", path, m)
			}
			if m := derivesRemaining.FindString(src); m != "" {
				t.Errorf("%s sums a lot's draws (%q) -- read credit_lots instead", path, m)
			}
			draws := writesDraw.FindAllString(src, -1)
			if len(draws) == 0 {
				return nil
			}
			abs, _ := filepath.Abs(filepath.Join(root, filepath.Dir(path)))
			if abs != billingDir {
				t.Errorf("%s writes a draw -- only billing's LotActs may", path)
				return nil
			}
			writers += len(draws)
			if !strings.Contains(src, ".from(") || !regexp.MustCompile(`Act\w+\.Origin`).MatchString(src) {
				t.Errorf("%s writes a draw without taking its lots and origin from a LotAct", path)
			}
			return nil
		})
		if err != nil {
			// coverage:ignore reason: the source tree is readable while tests run
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	if scanned < 100 {
		t.Fatalf("scanned %d files, want the whole of api/internal and api/cmd", scanned)
	}
	// ConsumeCredit and Forfeit
	if writers != 2 {
		t.Errorf("found %d draw writers, want 2 -- a new one needs a LotAct, and this count", writers)
	}
	refund, err := os.ReadFile("refund.go")
	if err != nil {
		// coverage:ignore reason: this package's own file is always readable while its tests run
		t.Fatalf("read refund.go: %v", err)
	}
	if !strings.Contains(string(refund), "ActRefund.from(") || !strings.Contains(string(refund), "ops_record_refund") {
		t.Error("refund.go records a refund without taking its lot from ActRefund")
	}
}
