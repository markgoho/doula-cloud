package engagement_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/engagement"
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

// changeKindAs PUTs a kind change as uid and returns the status code and
// decoded body (zero value on a non-200).
func changeKindAs(t *testing.T, db *testdb.DB, srv *httptest.Server, uid, practiceID, engagementID, kind string) (int, kindChangeResponseBody) {
	t.Helper()
	encoded, err := json.Marshal(map[string]any{"kind": kind})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut,
		srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/kind", bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	authntest.AddSessionCookie(req, authntest.SeedSession(t, db.App, uid))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	var out kindChangeResponseBody
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp.StatusCode, out
}

// readEngagementKind is the raw column every assertion here checks
// against, bypassing the API to prove the database itself agrees.
func readEngagementKind(t *testing.T, db *testdb.DB, engagementID string) string {
	t.Helper()
	var kind string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT kind::text FROM engagements WHERE id = $1`, engagementID,
	).Scan(&kind); err != nil {
		t.Fatalf("read engagement kind: %v", err)
	}
	return kind
}

// assertKindEvent reads the most recent 'kind_changed' engagement_events
// row and checks both sides of the fact, plus that a human actor is on
// it -- the cross-cutting audit expectation "how did this thing come to
// be?", the same shape assertOutcomeEvent checks for its own fact.
func assertKindEvent(t *testing.T, db *testdb.DB, engagementID, prevKind, kind string) {
	t.Helper()
	var gotPrevKind, gotKind string
	var actor *string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT previous_kind::text, kind::text, actor_staff_id::text
		   FROM engagement_events
		  WHERE engagement_id = $1 AND event_type = 'kind_changed'
		  ORDER BY created_at DESC LIMIT 1`, engagementID,
	).Scan(&gotPrevKind, &gotKind, &actor); err != nil {
		t.Fatalf("read kind event: %v", err)
	}
	if gotPrevKind != prevKind || gotKind != kind {
		t.Fatalf("event kind = %q -> %q, want %q -> %q", gotPrevKind, gotKind, prevKind, kind)
	}
	if actor == nil {
		t.Fatal("actor_staff_id is null, want the Staff member who changed it")
	}
}

// newKindServer mounts this package's whole surface and seeds a birth
// Engagement at a new Practice, with uid holding roles.
func newKindServer(t *testing.T, db *testdb.DB, uid string, roles []string, employmentType string) (srv *httptest.Server, practiceID, engagementID string) {
	t.Helper()
	practiceID, _ = testdb.SeedStaffAtNewPractice(t, db, uid, roles, employmentType)
	_, engagementID = testdb.SeedEngagementWithKind(t, db, practiceID, "Client", uid+"@example.com", string(engagement.KindBirth))
	srv, _ = newServer(t, db, uid)
	t.Cleanup(srv.Close)
	return srv, practiceID, engagementID
}

// TestChangeKindHandler_RolesMayChange is ADR-0015's role table read the
// same way #293's own birth-outcome test reads it: any role that may move
// a status may change kind too (refuseFactWrite), and a contractor Doula
// -- refused every status move outright -- is refused this too.
func TestChangeKindHandler_RolesMayChange(t *testing.T) {
	cases := []struct {
		role           string
		roles          []string
		employmentType string
		wantOK         bool
	}{
		{ownerRole, []string{ownerRole}, employeeType, true},
		{adminRole, []string{adminRole}, employeeType, true},
		{employeeDoulaKind, []string{doulaRole}, employeeType, true},
		{contractorDoulaKind, []string{doulaRole}, contractorType, false},
		{"no role", []string{}, employeeType, false},
	}
	for _, tc := range cases {
		t.Run(tc.role, func(t *testing.T) {
			db := testdb.New(t)
			uid := "change-kind-" + tc.role
			srv, practiceID, engagementID := newKindServer(t, db, uid, tc.roles, tc.employmentType)

			status, body := changeKindAs(t, db, srv, uid, practiceID, engagementID, string(engagement.KindPostpartum))

			if !tc.wantOK {
				if status != http.StatusForbidden {
					t.Fatalf("status = %d, want 403", status)
				}
				if got := readEngagementKind(t, db, engagementID); got != string(engagement.KindBirth) {
					t.Fatalf("kind = %q, want left unchanged at birth", got)
				}
				return
			}
			if status != http.StatusOK {
				t.Fatalf("status = %d, want 200", status)
			}
			if body.Kind != string(engagement.KindPostpartum) {
				t.Fatalf("kind = %q, want postpartum", body.Kind)
			}
			if got := readEngagementKind(t, db, engagementID); got != string(engagement.KindPostpartum) {
				t.Fatalf("kind = %q, want postpartum", got)
			}
			if n := countEngagementEvents(t, db, engagementID); n != 1 {
				t.Fatalf("engagement_events rows = %d, want 1", n)
			}
			assertKindEvent(t, db, engagementID, string(engagement.KindBirth), string(engagement.KindPostpartum))
		})
	}
}

// TestChangeKindHandler_Validation proves the handler refuses a value
// outside ADR-0015's two-member vocabulary before it ever reaches the
// database.
func TestChangeKindHandler_Validation(t *testing.T) {
	db := testdb.New(t)
	const uid = "kind-validation-owner"
	srv, practiceID, engagementID := newKindServer(t, db, uid, []string{ownerRole}, employeeType)

	status, _ := changeKindAs(t, db, srv, uid, practiceID, engagementID, "postnatal")
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", status)
	}
	if got := readEngagementKind(t, db, engagementID); got != string(engagement.KindBirth) {
		t.Fatalf("kind = %q, want left unchanged", got)
	}
	if n := countEngagementEvents(t, db, engagementID); n != 0 {
		t.Fatalf("engagement_events rows = %d, want 0 -- a refused write leaves no audit row", n)
	}
}

// TestChangeKindHandler_BothDirectionsAndNoOp proves ADR-0015's "mutable
// in both directions" is not a one-way door while the pregnancy is still
// expected (birth_outcome stays null throughout -- SeedEngagementWithKind
// records none), and that PUT is naturally idempotent: re-sending the
// kind an Engagement already holds writes nothing and raises no second
// audit row (docs/api-design.md rule 4). The postpartum -> birth move
// once a birth outcome is recorded is TestChangeKindHandler_UpgradeAfterBirthRefused's
// own subject, not this one's.
func TestChangeKindHandler_BothDirectionsAndNoOp(t *testing.T) {
	db := testdb.New(t)
	const uid = "kind-both-directions-owner"
	srv, practiceID, engagementID := newKindServer(t, db, uid, []string{ownerRole}, employeeType)

	if status, body := changeKindAs(t, db, srv, uid, practiceID, engagementID, string(engagement.KindPostpartum)); status != http.StatusOK || body.Kind != string(engagement.KindPostpartum) {
		t.Fatalf("birth -> postpartum: status = %d, body = %+v", status, body)
	}

	if status, body := changeKindAs(t, db, srv, uid, practiceID, engagementID, string(engagement.KindPostpartum)); status != http.StatusOK || body.Kind != string(engagement.KindPostpartum) {
		t.Fatalf("resend: status = %d, body = %+v", status, body)
	}
	if n := countEngagementEvents(t, db, engagementID); n != 1 {
		t.Fatalf("engagement_events rows = %d, want 1 -- the resend writes nothing", n)
	}

	if status, body := changeKindAs(t, db, srv, uid, practiceID, engagementID, string(engagement.KindBirth)); status != http.StatusOK || body.Kind != string(engagement.KindBirth) {
		t.Fatalf("postpartum -> birth: status = %d, body = %+v", status, body)
	}
	if n := countEngagementEvents(t, db, engagementID); n != 2 {
		t.Fatalf("engagement_events rows = %d, want 2", n)
	}
	assertKindEvent(t, db, engagementID, string(engagement.KindPostpartum), string(engagement.KindBirth))
}

// TestChangeKindHandler_UpgradeAfterBirthRefused is ADR-0015's own
// sentence: "the product stops offering a postpartum -> birth change once
// the birth outcome is recorded, because attending a birth that has
// already happened means nothing." Once SeedBirthOutcome records one,
// the upgrade a bare 200 would otherwise allow is refused with a named
// 409, the kind is left untouched, and no audit row is written.
func TestChangeKindHandler_UpgradeAfterBirthRefused(t *testing.T) {
	db := testdb.New(t)
	const uid = "kind-upgrade-after-birth-owner"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)
	_, engagementID := testdb.SeedEngagementWithKind(t, db, practiceID, "Client", uid+"@example.com", string(engagement.KindPostpartum))
	testdb.SeedBirthOutcome(t, db, engagementID)
	srv, _ := newServer(t, db, uid)
	t.Cleanup(srv.Close)

	status, body := changeKindAs(t, db, srv, uid, practiceID, engagementID, string(engagement.KindBirth))
	if status != http.StatusConflict {
		t.Fatalf("status = %d, want 409", status)
	}
	if body.Code != apierr.CodeConflict {
		t.Fatalf("code = %q, want %s", body.Code, apierr.CodeConflict)
	}
	if got := readEngagementKind(t, db, engagementID); got != string(engagement.KindPostpartum) {
		t.Fatalf("kind = %q, want left unchanged at postpartum", got)
	}
	if n := countEngagementEvents(t, db, engagementID); n != 0 {
		t.Fatalf("engagement_events rows = %d, want 0 -- the refusal writes nothing", n)
	}
}

// TestChangeKindHandler_DowngradeAfterBirthStillAllowed is the other side
// of the same rule: standing rule 2 names 'birth' -> 'postpartum' "the
// rare downgrade" kind's own lack of a database freeze exists for, and
// the ADR attaches no birth-outcome caveat to it. A recorded outcome
// refuses the upgrade only; the downgrade goes through exactly as it does
// with no outcome recorded at all.
func TestChangeKindHandler_DowngradeAfterBirthStillAllowed(t *testing.T) {
	db := testdb.New(t)
	const uid = "kind-downgrade-after-birth-owner"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)
	_, engagementID := testdb.SeedEngagementWithKind(t, db, practiceID, "Client", uid+"@example.com", string(engagement.KindBirth))
	testdb.SeedBirthOutcome(t, db, engagementID)
	srv, _ := newServer(t, db, uid)
	t.Cleanup(srv.Close)

	status, body := changeKindAs(t, db, srv, uid, practiceID, engagementID, string(engagement.KindPostpartum))
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if body.Kind != string(engagement.KindPostpartum) {
		t.Fatalf("kind = %q, want postpartum", body.Kind)
	}
	if n := countEngagementEvents(t, db, engagementID); n != 1 {
		t.Fatalf("engagement_events rows = %d, want 1", n)
	}
}

// TestChangeKindHandler_UnknownEngagement covers both doors an engagement
// id can fail at: one that is not a UUID at all, and a well-formed one
// belonging to nobody this caller can reach.
func TestChangeKindHandler_UnknownEngagement(t *testing.T) {
	cases := []struct {
		name         string
		engagementID string
		want         int
	}{
		{"not a uuid", "not-a-uuid", http.StatusBadRequest},
		{"no such engagement", "00000000-0000-0000-0000-000000000000", http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testdb.New(t)
			uid := "missing-kind-engagement-" + tc.name
			srv, practiceID, _ := newKindServer(t, db, uid, []string{ownerRole}, employeeType)

			status, _ := changeKindAs(t, db, srv, uid, practiceID, tc.engagementID, string(engagement.KindPostpartum))
			if status != tc.want {
				t.Fatalf("status = %d, want %d", status, tc.want)
			}
		})
	}
}

// TestChangeKindHandler_MalformedBody covers the decode refusal.
func TestChangeKindHandler_MalformedBody(t *testing.T) {
	db := testdb.New(t)
	const uid = "malformed-kind-owner"
	srv, practiceID, engagementID := newKindServer(t, db, uid, []string{ownerRole}, employeeType)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut,
		srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/kind",
		bytes.NewReader([]byte("{")))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	authntest.AddSessionCookie(req, authntest.SeedSession(t, db.App, uid))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// TestChangeKindHandler_MovesNothingElse is ADR-0015's own list, proved
// against the database rather than asserted in prose: changing kind moves
// no Visit, spends no second Credit, and touches no Contract or Invoice.
// It also folds together this ticket's audit and Client-facing
// requirements (#874's AC2 and AC6) into one assertion: the one row
// written lands on engagement_events, the staff-only ledger, and the
// Client-facing activity table (ADR-0022) gets nothing at all.
func TestChangeKindHandler_MovesNothingElse(t *testing.T) {
	db := testdb.New(t)
	const uid = "kind-moves-nothing-owner"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, employeeType)
	_, engagementID := testdb.SeedEngagementWithKind(t, db, practiceID, "Client", uid+"@example.com", string(engagement.KindBirth))
	srv, _ := newServer(t, db, uid)
	t.Cleanup(srv.Close)

	var visitID string
	var visitCreatedAt time.Time
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO visits (engagement_id, staff_id) VALUES ($1, $2) RETURNING id, created_at`,
		engagementID, staffID,
	).Scan(&visitID, &visitCreatedAt); err != nil {
		t.Fatalf("seed visit: %v", err)
	}

	var contractID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO contracts (engagement_id, prose, amount_cents) VALUES ($1, 'test prose', 450000) RETURNING id`,
		engagementID,
	).Scan(&contractID); err != nil {
		t.Fatalf("seed contract: %v", err)
	}

	var invoiceID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO invoices (practice_id, contract_id, stripe_invoice_id, amount_cents, reference, due_at)
		 VALUES ($1, $2, 'in_test_kind_moves_nothing', 450000, 'INV-KIND-1', now()) RETURNING id`,
		practiceID, contractID,
	).Scan(&invoiceID); err != nil {
		t.Fatalf("seed invoice: %v", err)
	}

	// A consumption row names the lot it drew from (#420's
	// credit_ledger_lot_or_draw), the same shape billing.ConsumeCredit
	// itself writes: a grant lot first, then a draw against it.
	var lotID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO credit_ledger (practice_id, origin, quantity) VALUES ($1, 'signup_bonus', 1) RETURNING id`,
		practiceID,
	).Scan(&lotID); err != nil {
		t.Fatalf("seed credit lot: %v", err)
	}
	var creditID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO credit_ledger (practice_id, origin, quantity, consumed_engagement_id, consumed_at, drawn_lot_id)
		 VALUES ($1, 'consumption', -1, $2, now(), $3) RETURNING id`,
		practiceID, engagementID, lotID,
	).Scan(&creditID); err != nil {
		t.Fatalf("seed credit: %v", err)
	}

	status, _ := changeKindAs(t, db, srv, uid, practiceID, engagementID, string(engagement.KindPostpartum))
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}

	// The Visit row is untouched -- its type is derived fresh on every
	// read from the Engagement's pregnancy-end date alone
	// (visit.DeriveType), never stored, so there is nothing here for a
	// kind change to move.
	var gotVisitCreatedAt time.Time
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT created_at FROM visits WHERE id = $1`, visitID,
	).Scan(&gotVisitCreatedAt); err != nil {
		t.Fatalf("re-read visit: %v", err)
	}
	if !gotVisitCreatedAt.Equal(visitCreatedAt) {
		t.Fatalf("visit changed: got %v, want %v", gotVisitCreatedAt, visitCreatedAt)
	}

	var contractStatus string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status::text FROM contracts WHERE id = $1`, contractID,
	).Scan(&contractStatus); err != nil {
		t.Fatalf("re-read contract: %v", err)
	}
	if contractStatus != "draft" {
		t.Fatalf("contract status = %q, want draft (untouched)", contractStatus)
	}

	var invoiceStatus string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status::text FROM invoices WHERE id = $1`, invoiceID,
	).Scan(&invoiceStatus); err != nil {
		t.Fatalf("re-read invoice: %v", err)
	}
	if invoiceStatus != "draft" {
		t.Fatalf("invoice status = %q, want draft (untouched)", invoiceStatus)
	}

	var creditQuantity int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT quantity FROM credit_ledger WHERE id = $1`, creditID,
	).Scan(&creditQuantity); err != nil {
		t.Fatalf("re-read credit: %v", err)
	}
	if creditQuantity != -1 {
		t.Fatalf("credit quantity = %d, want -1 (untouched, no second Credit spent)", creditQuantity)
	}
	var creditCount int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM credit_ledger WHERE consumed_engagement_id = $1`, engagementID,
	).Scan(&creditCount); err != nil {
		t.Fatalf("count credit_ledger: %v", err)
	}
	if creditCount != 1 {
		t.Fatalf("credit_ledger rows for this Engagement = %d, want 1 -- no second Credit spent", creditCount)
	}

	if n := countEngagementEvents(t, db, engagementID); n != 1 {
		t.Fatalf("engagement_events rows = %d, want 1", n)
	}
	if n := countActivityActions(t, db, engagementID, "kind_changed"); n != 0 {
		t.Fatalf("activity rows for kind_changed = %d, want 0 -- kind is staff-only and never reaches the Client-facing ledger", n)
	}
}
