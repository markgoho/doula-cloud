package practicerate_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/testdb"
)

// seedContract seeds a Contract row directly (bypassing PostContractHandler),
// at an explicit status and override flag, always at 15000 -- #968's
// reprice pass needs to be proven against every one of those axes, which
// no single endpoint in this package can put a Contract into. Mirrors
// contracts_test's own seedContract (also hardcoded at 15000), kept
// local for the same reason that one is: no package shares a "seed a
// Contract in an arbitrary state" helper via testdb.
func seedContract(t *testing.T, db *testdb.DB, engagementID, status string, overridden bool) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO contracts (engagement_id, status, prose, amount_cents, amount_overridden)
		 VALUES ($1, $2::contract_status, 'Agreement.', 15000, $3)`,
		engagementID, status, overridden,
	); err != nil {
		t.Fatalf("seed contract: %v", err)
	}
}

// contractState is what each test below reads back per Contract to
// check the reprice pass touched exactly the rows #968's AC says it
// should.
type contractState struct {
	amountCents     int64
	amountChangedAt *time.Time
}

func readContractState(t *testing.T, db *testdb.DB, engagementID string) contractState {
	t.Helper()
	var out contractState
	var changedAt sql.NullTime
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT amount_cents, amount_changed_at FROM contracts WHERE engagement_id = $1`,
		engagementID,
	).Scan(&out.amountCents, &changedAt); err != nil {
		t.Fatalf("read contract state: %v", err)
	}
	if changedAt.Valid {
		out.amountChangedAt = &changedAt.Time
	}
	return out
}

func repriceActivityDiff(t *testing.T, db *testdb.DB, engagementID string) (found bool, before, after int64) {
	t.Helper()
	var diff []byte
	var actorKind string
	err := db.Admin.QueryRowContext(t.Context(),
		`SELECT diff, actor_kind FROM activity WHERE subject_kind = 'engagement' AND subject_id = $1 AND action = 'contract_amount_repriced'`,
		engagementID,
	).Scan(&diff, &actorKind)
	if err == sql.ErrNoRows {
		return false, 0, 0
	}
	if err != nil {
		t.Fatalf("query reprice activity: %v", err)
	}
	if actorKind != "system" {
		t.Fatalf("actor_kind = %q, want %q (ADR-0022: nobody performed a reprice)", actorKind, "system")
	}
	var parsed struct {
		AmountCentsBefore int64 `json:"amountCentsBefore"`
		AmountCentsAfter  int64 `json:"amountCentsAfter"`
	}
	if err := json.Unmarshal(diff, &parsed); err != nil {
		t.Fatalf("unmarshal reprice diff: %v", err)
	}
	return true, parsed.AmountCentsBefore, parsed.AmountCentsAfter
}

// TestPutRateHandler_RepricesDraftAndSentContracts proves #968's core
// AC: changing a rate updates the amount on every not-yet-signed
// Contract for that kind, whether it is still a Draft or already Sent
// for signature, and records the move under the system actor
// (ADR-0022) with the before/after amounts.
func TestPutRateHandler_RepricesDraftAndSentContracts(t *testing.T) {
	db := testdb.New(t)
	const uid = "reprice-draft-sent"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)
	seedRate(t, db, practiceID, 15000)
	_, draftEngagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, draftEngagementID, "draft", false)
	_, sentEngagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, sentEngagementID, "sent", false)

	srv, session := newRateServer(t, db, uid)
	defer srv.Close()

	resp := putRate(t, srv, session, practiceID, "birth", 20000)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	for _, engagementID := range []string{draftEngagementID, sentEngagementID} {
		state := readContractState(t, db, engagementID)
		if state.amountCents != 20000 {
			t.Fatalf("engagement %q amountCents = %d, want 20000", engagementID, state.amountCents)
		}
		if state.amountChangedAt == nil {
			t.Fatalf("engagement %q amountChangedAt = nil, want set", engagementID)
		}
		found, before, after := repriceActivityDiff(t, db, engagementID)
		if !found {
			t.Fatalf("engagement %q: no contract_amount_repriced activity row", engagementID)
		}
		if before != 15000 || after != 20000 {
			t.Fatalf("engagement %q diff = before %d after %d, want before 15000 after 20000", engagementID, before, after)
		}
	}
}

// TestPutRateHandler_LeavesSignedVoidedAndOverriddenContractsAlone
// proves the three exclusions #968's AC names: a signed Contract's
// amount is untouched by any later rate change, a voided one (which was
// signed before it was voided) is equally frozen, and a Contract whose
// amount an Owner/Admin overrode keeps that override rather than
// re-deriving.
func TestPutRateHandler_LeavesSignedVoidedAndOverriddenContractsAlone(t *testing.T) {
	db := testdb.New(t)
	const uid = "reprice-excluded"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)
	seedRate(t, db, practiceID, 15000)

	_, signedEngagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, signedEngagementID, "signed", false)
	_, voidedEngagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, voidedEngagementID, "voided", false)
	_, overriddenEngagementID := testdb.SeedEngagement(t, db, practiceID)
	seedContract(t, db, overriddenEngagementID, "draft", true)

	srv, session := newRateServer(t, db, uid)
	defer srv.Close()

	resp := putRate(t, srv, session, practiceID, "birth", 20000)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	for _, engagementID := range []string{signedEngagementID, voidedEngagementID, overriddenEngagementID} {
		state := readContractState(t, db, engagementID)
		if state.amountCents != 15000 {
			t.Fatalf("engagement %q amountCents = %d, want unchanged 15000", engagementID, state.amountCents)
		}
		if state.amountChangedAt != nil {
			t.Fatalf("engagement %q amountChangedAt = %v, want nil (never repriced)", engagementID, state.amountChangedAt)
		}
		if found, _, _ := repriceActivityDiff(t, db, engagementID); found {
			t.Fatalf("engagement %q: unwanted contract_amount_repriced activity row", engagementID)
		}
	}
}

// TestPutRateHandler_RepricesOnlyTheChangedKind proves a rate change to
// "birth" never touches a "postpartum" Contract, even an eligible
// (draft, un-overridden) one -- #968's AC scopes reprice to "that kind".
func TestPutRateHandler_RepricesOnlyTheChangedKind(t *testing.T) {
	db := testdb.New(t)
	const uid = "reprice-other-kind"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)
	seedRate(t, db, practiceID, 15000)
	_, engagementID := testdb.SeedEngagementWithKind(t, db, practiceID, "PP Client", "pp@example.com", "postpartum")
	seedContract(t, db, engagementID, "draft", false)

	srv, session := newRateServer(t, db, uid)
	defer srv.Close()

	resp := putRate(t, srv, session, practiceID, "birth", 20000)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	state := readContractState(t, db, engagementID)
	if state.amountCents != 15000 {
		t.Fatalf("postpartum contract amountCents = %d, want unchanged 15000", state.amountCents)
	}
	if found, _, _ := repriceActivityDiff(t, db, engagementID); found {
		t.Fatalf("postpartum contract: unwanted contract_amount_repriced activity row from a birth rate change")
	}
}
