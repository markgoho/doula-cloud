package practicerate_test

import (
	"database/sql"
	"encoding/json/v2"
	"testing"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/testdb"
)

// seedContract seeds a Contract row directly (bypassing PostContractHandler)
// for a new Engagement of kind, at an explicit status and override flag,
// always at 15000 -- #968's reprice pass needs to be proven against every
// one of those axes, which no single endpoint in this package can put a
// Contract into. It returns the Engagement's id.
func (e *env) seedContract(kind, status string, overridden bool) (engagementID string) {
	e.T.Helper()
	engagementID = e.AnotherEngagement(fixture.Kind(kind)).ID
	e.Exec(
		`INSERT INTO contracts (engagement_id, status, prose, amount_cents, amount_overridden)
		 VALUES ($1, $2::contract_status, 'Agreement.', 15000, $3)`,
		engagementID, status, overridden)
	return engagementID
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
	var parsed activity.AmountDiff
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
	e := ownerEnv(t)
	e.seedRate(15000)
	draftEngagementID := e.seedContract(fixture.KindBirth, "draft", false)
	sentEngagementID := e.seedContract(fixture.KindBirth, "sent", false)

	e.putRate(fixture.KindBirth, 20000)

	for _, engagementID := range []string{draftEngagementID, sentEngagementID} {
		state := readContractState(t, e.DB, engagementID)
		if state.amountCents != 20000 {
			t.Fatalf("engagement %q amountCents = %d, want 20000", engagementID, state.amountCents)
		}
		if state.amountChangedAt == nil {
			t.Fatalf("engagement %q amountChangedAt = nil, want set", engagementID)
		}
		found, before, after := repriceActivityDiff(t, e.DB, engagementID)
		if !found {
			t.Fatalf("engagement %q: no contract_amount_repriced activity row", engagementID)
		}
		if before != 15000 || after != 20000 {
			t.Fatalf("engagement %q diff = before %d after %d, want before 15000 after 20000", engagementID, before, after)
		}
	}
}

// TestPutRateHandler_LeavesIneligibleContractsAlone proves the exclusions
// #968's AC names: a signed Contract's amount is untouched by any later
// rate change, a voided one (which was signed before it was voided) is
// equally frozen, a Contract whose amount an Owner/Admin overrode keeps
// that override rather than re-deriving, and a rate change to "birth"
// never touches a "postpartum" Contract, even an eligible (draft,
// un-overridden) one -- the AC scopes reprice to "that kind".
func TestPutRateHandler_LeavesIneligibleContractsAlone(t *testing.T) {
	e := ownerEnv(t)
	e.seedRate(15000)
	untouched := map[string]string{
		"signed":                 e.seedContract(fixture.KindBirth, "signed", false),
		"voided":                 e.seedContract(fixture.KindBirth, "voided", false),
		"overridden":             e.seedContract(fixture.KindBirth, "draft", true),
		"other kind of Contract": e.seedContract(fixture.KindPostpartum, "draft", false),
	}

	e.putRate(fixture.KindBirth, 20000)

	for name, engagementID := range untouched {
		state := readContractState(t, e.DB, engagementID)
		if state.amountCents != 15000 {
			t.Fatalf("%s: amountCents = %d, want unchanged 15000", name, state.amountCents)
		}
		if state.amountChangedAt != nil {
			t.Fatalf("%s: amountChangedAt = %v, want nil (never repriced)", name, state.amountChangedAt)
		}
		if found, _, _ := repriceActivityDiff(t, e.DB, engagementID); found {
			t.Fatalf("%s: unwanted contract_amount_repriced activity row", name)
		}
	}
}
