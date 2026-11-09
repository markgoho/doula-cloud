package engagement_test

import (
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// kindChangeResponseBody decodes both shapes this endpoint answers with:
// the success DTO, and apierr's own error envelope's Code, the same
// dual-purpose shape outcomeResponseBody uses.
type kindChangeResponseBody struct {
	EngagementID string      `json:"engagementId"`
	Kind         string      `json:"kind"`
	Code         apierr.Code `json:"code"`
}

// changeKind PUTs a kind change for the Engagement as the Staff member at
// the screen and requires want.
func (e *env) changeKind(kind engagement.Kind, want int) kindChangeResponseBody {
	e.T.Helper()
	return reqtest.Expect[kindChangeResponseBody](e.T, http.MethodPut, e.engagementURL("/kind"), want, e.SignedIn,
		reqtest.JSON(map[string]any{"kind": string(kind)}))
}

// kind is the Engagement's stored kind, read past the API to prove the
// database itself agrees.
func (e *env) kind() string {
	e.T.Helper()
	return testdb.Scalar[string](e.T, e.DB.Admin, `SELECT kind::text FROM engagements WHERE id = $1`, e.Engagement.ID)
}

// assertKindEvent reads the most recent kind_changed activity entry
// and checks both sides of the fact, plus that a human actor is on it --
// the cross-cutting audit expectation "how did this thing come to be?",
// the same shape assertOutcomeEvent checks for its own fact.
func assertKindEvent(t *testing.T, db *testdb.DB, engagementID, prevKind, kind string) {
	t.Helper()
	assertDiff(t, readFactDiff(t, db, engagementID, "kind_changed"), map[string]any{
		"kindBefore": prevKind,
		"kindAfter":  kind,
	})
}

const (
	birthKind      = string(engagement.KindBirth)
	postpartumKind = string(engagement.KindPostpartum)
)

// TestChangeKindHandler_RolesMayChange is ADR-0015's role table read the
// same way #293's own birth-outcome test reads it: any role that may move
// a status may change kind too (refuseFactWrite), and a contractor Doula
// -- refused every status move outright -- is refused this too.
func TestChangeKindHandler_RolesMayChange(t *testing.T) {
	eachFactWriter(t, func(t *testing.T, e *env, wantOK bool) {
		body := e.changeKind(engagement.KindPostpartum, okOrForbidden(wantOK))

		if !wantOK {
			if got := e.kind(); got != birthKind {
				t.Fatalf("kind = %q, want left unchanged at birth", got)
			}
			return
		}
		if body.Kind != postpartumKind {
			t.Fatalf("kind = %q, want postpartum", body.Kind)
		}
		if got := e.kind(); got != postpartumKind {
			t.Fatalf("kind = %q, want postpartum", got)
		}
		if n := countFactEntries(t, e.DB, e.Engagement.ID); n != 1 {
			t.Fatalf("fact entries = %d, want 1", n)
		}
		assertKindEvent(t, e.DB, e.Engagement.ID, birthKind, postpartumKind)
	})
}

// TestChangeKindHandler_Validation proves the handler refuses a value
// outside ADR-0015's two-member vocabulary before it ever reaches the
// database.
func TestChangeKindHandler_Validation(t *testing.T) {
	e := newOwnerEnv(t)

	e.changeKind("postnatal", http.StatusBadRequest)

	if got := e.kind(); got != birthKind {
		t.Fatalf("kind = %q, want left unchanged", got)
	}
	if n := countFactEntries(t, e.DB, e.Engagement.ID); n != 0 {
		t.Fatalf("fact entries = %d, want 0 -- a refused write leaves no audit row", n)
	}
}

// TestChangeKindHandler_BothDirectionsAndNoOp proves ADR-0015's "mutable
// in both directions" is not a one-way door while the pregnancy is still
// expected (no birth outcome is recorded throughout), and that PUT is
// naturally idempotent: re-sending the kind an Engagement already holds
// writes nothing and raises no second audit row (docs/api-design.md rule
// 4). The postpartum -> birth move once a birth outcome is recorded is
// TestChangeKindHandler_UpgradeAfterBirthRefused's own subject, not this
// one's.
func TestChangeKindHandler_BothDirectionsAndNoOp(t *testing.T) {
	e := newOwnerEnv(t)

	if body := e.changeKind(engagement.KindPostpartum, http.StatusOK); body.Kind != postpartumKind {
		t.Fatalf("birth -> postpartum: body = %+v", body)
	}

	if body := e.changeKind(engagement.KindPostpartum, http.StatusOK); body.Kind != postpartumKind {
		t.Fatalf("resend: body = %+v", body)
	}
	if n := countFactEntries(t, e.DB, e.Engagement.ID); n != 1 {
		t.Fatalf("fact entries = %d, want 1 -- the resend writes nothing", n)
	}

	if body := e.changeKind(engagement.KindBirth, http.StatusOK); body.Kind != birthKind {
		t.Fatalf("postpartum -> birth: body = %+v", body)
	}
	if n := countFactEntries(t, e.DB, e.Engagement.ID); n != 2 {
		t.Fatalf("fact entries = %d, want 2", n)
	}
	assertKindEvent(t, e.DB, e.Engagement.ID, postpartumKind, birthKind)
}

// TestChangeKindHandler_AfterABirthOutcome is ADR-0015's own sentence:
// "the product stops offering a postpartum -> birth change once the birth
// outcome is recorded, because attending a birth that has already
// happened means nothing." The upgrade a bare 200 would otherwise allow
// is refused with a named 409, the kind is left untouched, and no audit
// row is written. The downgrade is the other side of the same rule:
// standing rule 2 names 'birth' -> 'postpartum' "the rare downgrade"
// kind's own lack of a database freeze exists for, and the ADR attaches
// no birth-outcome caveat to it, so it goes through exactly as it does
// with no outcome recorded at all.
func TestChangeKindHandler_AfterABirthOutcome(t *testing.T) {
	cases := []struct {
		name       string
		from       string
		to         engagement.Kind
		wantStatus int
		wantKind   string
		wantFacts  int
	}{
		{"the upgrade to birth is refused", postpartumKind, engagement.KindBirth, http.StatusConflict, postpartumKind, 0},
		{"the downgrade to postpartum is still allowed", birthKind, engagement.KindPostpartum, http.StatusOK, postpartumKind, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newOwnerEnv(t, withEngagement(fixture.Kind(tc.from)))
			testdb.SeedBirthOutcome(t, e.DB, e.Engagement.ID)

			body := e.changeKind(tc.to, tc.wantStatus)

			if tc.wantStatus == http.StatusConflict && body.Code != apierr.CodeConflict {
				t.Fatalf("code = %q, want %s", body.Code, apierr.CodeConflict)
			}
			if got := e.kind(); got != tc.wantKind {
				t.Fatalf("kind = %q, want %q", got, tc.wantKind)
			}
			if n := countFactEntries(t, e.DB, e.Engagement.ID); n != tc.wantFacts {
				t.Fatalf("fact entries = %d, want %d", n, tc.wantFacts)
			}
		})
	}
}

// TestChangeKindHandler_RefusesARequestItCannotRead covers the doors a
// request can fail at before any kind is weighed.
func TestChangeKindHandler_RefusesARequestItCannotRead(t *testing.T) {
	e := newOwnerEnv(t)

	e.refusesAnUnreadableRequest(t, http.MethodPut, "/kind", reqtest.JSON(map[string]any{"kind": postpartumKind}))
}

// TestChangeKindHandler_MovesNothingElse is ADR-0015's own list, proved
// against the database rather than asserted in prose: changing kind moves
// no Visit, spends no second Credit, and touches no Contract or Invoice.
// It also folds together this ticket's audit requirement (#874's AC2)
// into one assertion: exactly one fact entry is written, and it is the
// kind_changed one. #874's AC6 -- kind never reaches the Client -- is now
// activity.StaffingActions()' job since #1423 folded engagement_events
// into activity; portal's own TestActivityHandler_HidesTheEngagementAudit
// proves the portal read leaves it out.
func TestChangeKindHandler_MovesNothingElse(t *testing.T) {
	e := newOwnerEnv(t)
	admin := e.DB.Admin
	engagementID := e.Engagement.ID

	var visitID string
	var visitCreatedAt time.Time
	if err := admin.QueryRowContext(t.Context(),
		`INSERT INTO visits (engagement_id, staff_id) VALUES ($1, $2) RETURNING id, created_at`,
		engagementID, e.Staff.ID,
	).Scan(&visitID, &visitCreatedAt); err != nil {
		t.Fatalf("seed visit: %v", err)
	}
	contractID := testdb.Scalar[string](t, admin,
		`INSERT INTO contracts (engagement_id, prose, amount_cents) VALUES ($1, 'test prose', 450000) RETURNING id`, engagementID)
	invoiceID := testdb.Scalar[string](t, admin,
		`INSERT INTO invoices (practice_id, contract_id, stripe_invoice_id, amount_cents, reference, due_at)
		 VALUES ($1, $2, 'in_test_kind_moves_nothing', 450000, 'INV-KIND-1', now()) RETURNING id`, e.Practice.ID, contractID)
	// A consumption row names the lot it drew from (#420's
	// credit_ledger_lot_or_draw), the same shape billing.ConsumeCredit
	// itself writes: a grant lot first, then a draw against it.
	lotID := testdb.Scalar[string](t, admin,
		`INSERT INTO credit_ledger (practice_id, origin, quantity) VALUES ($1, 'signup_bonus', 1) RETURNING id`, e.Practice.ID)
	creditID := testdb.Scalar[string](t, admin,
		`INSERT INTO credit_ledger (practice_id, origin, quantity, consumed_engagement_id, consumed_at, drawn_lot_id)
		 VALUES ($1, 'consumption', -1, $2, now(), $3) RETURNING id`, e.Practice.ID, engagementID, lotID)

	e.changeKind(engagement.KindPostpartum, http.StatusOK)

	// The Visit row is untouched -- its type is derived fresh on every
	// read from the Engagement's pregnancy-end date alone
	// (visit.DeriveType), never stored, so there is nothing here for a
	// kind change to move.
	if got := testdb.Scalar[time.Time](t, admin, `SELECT created_at FROM visits WHERE id = $1`, visitID); !got.Equal(visitCreatedAt) {
		t.Fatalf("visit changed: got %v, want %v", got, visitCreatedAt)
	}
	if got := testdb.Scalar[string](t, admin, `SELECT status::text FROM contracts WHERE id = $1`, contractID); got != "draft" {
		t.Fatalf("contract status = %q, want draft (untouched)", got)
	}
	if got := testdb.Scalar[string](t, admin, `SELECT status::text FROM invoices WHERE id = $1`, invoiceID); got != "draft" {
		t.Fatalf("invoice status = %q, want draft (untouched)", got)
	}
	if got := testdb.Scalar[int](t, admin, `SELECT quantity FROM credit_ledger WHERE id = $1`, creditID); got != -1 {
		t.Fatalf("credit quantity = %d, want -1 (untouched, no second Credit spent)", got)
	}
	if got := testdb.Scalar[int](t, admin, `SELECT count(*) FROM credit_ledger WHERE consumed_engagement_id = $1`, engagementID); got != 1 {
		t.Fatalf("credit_ledger rows for this Engagement = %d, want 1 -- no second Credit spent", got)
	}
	if n := countFactEntries(t, e.DB, engagementID); n != 1 {
		t.Fatalf("fact entries = %d, want 1", n)
	}
	if n := e.countActivity("kind_changed"); n != 1 {
		t.Fatalf("kind_changed entries = %d, want 1", n)
	}
}
