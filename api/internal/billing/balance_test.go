package billing_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

func seedStaff(t *testing.T, db *testdb.DB, identityUID string) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff (identity_uid, name, email) VALUES ($1, 'Test Staff', 'staff@example.com') RETURNING id`,
		identityUID,
	).Scan(&id); err != nil {
		t.Fatalf("seed staff %q: %v", identityUID, err)
	}
	return id
}

func seedMembership(t *testing.T, db *testdb.DB, practiceID, staffID string, roles string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_memberships (practice_id, staff_id, roles) VALUES ($1, $2, $3::practice_role[])`,
		practiceID, staffID, roles,
	); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
}

// seedMember seeds a Practice and a Staff member holding a doula (non-Owner)
// role there -- GetBalanceHandler must allow this role, unlike Owner-gated
// handlers elsewhere.
func seedMember(t *testing.T, db *testdb.DB, identityUID string) (practiceID string) {
	t.Helper()
	practiceID = seedPractice(t, db, "Test Practice")
	staffID := seedStaff(t, db, identityUID)
	seedMembership(t, db, practiceID, staffID, "{doula}")
	return practiceID
}

func seedLedgerRow(t *testing.T, db *testdb.DB, practiceID, origin string, quantity int) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO credit_ledger (practice_id, origin, quantity) VALUES ($1, $2::credit_ledger_origin, $3)`,
		practiceID, origin, quantity,
	); err != nil {
		t.Fatalf("seed credit_ledger row: %v", err)
	}
}

func newBillingServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("GET /practices/{practiceId}/billing",
		staffauth.Middleware(db.App)(billing.GetBalanceHandler()))
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

func getBalance(t *testing.T, srv *httptest.Server, session string, practiceID string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/practices/"+practiceID+"/billing", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

// TestBalance_SumsLedgerRows exercises the billing package boundary AC
// #75 calls for: a Practice's derived balance after a sequence of
// credit_ledger rows of different origins and signs.
func TestBalance_SumsLedgerRows(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Test Practice")
	seedLedgerRow(t, db, practiceID, "signup_bonus", 3)
	seedLedgerRow(t, db, practiceID, "purchase", 5)
	seedLedgerRow(t, db, practiceID, "purchase", -1)

	tx, err := db.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	got, err := billing.Balance(t.Context(), tx, practiceID)
	if err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if got != 7 {
		t.Fatalf("balance = %d, want 7", got)
	}
}

// TestBalance_ZeroForPracticeWithNoLedgerRows proves SUM's NULL-on-no-rows
// case is coalesced to 0, not surfaced as a scan error or a negative/odd
// value.
func TestBalance_ZeroForPracticeWithNoLedgerRows(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Test Practice")

	tx, err := db.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	got, err := billing.Balance(t.Context(), tx, practiceID)
	if err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if got != 0 {
		t.Fatalf("balance = %d, want 0", got)
	}
}

// TestGetBalanceHandler_AnyMemberAllowed proves a non-Owner Staff member
// can read the balance and ledger history -- AC #75 explicitly rules out
// an Owner-only restriction.
func TestGetBalanceHandler_AnyMemberAllowed(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-any-member"
	practiceID := seedMember(t, db, uid)
	seedLedgerRow(t, db, practiceID, "signup_bonus", 3)
	seedLedgerRow(t, db, practiceID, "purchase", 5)

	srv, session := newBillingServer(t, db, uid)
	defer srv.Close()

	resp := getBalance(t, srv, session, practiceID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var out billing.BalanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Balance != 8 {
		t.Fatalf("balance = %d, want 8", out.Balance)
	}
	if len(out.Ledger) != 2 {
		t.Fatalf("ledger = %+v, want 2 entries", out.Ledger)
	}
	// Most recent first: the purchase (seeded second) precedes the
	// signup_bonus (seeded first).
	if out.Ledger[0].Origin != "purchase" || out.Ledger[0].Quantity != 5 {
		t.Fatalf("ledger[0] = %+v, want purchase +5", out.Ledger[0])
	}
	if out.Ledger[1].Origin != "signup_bonus" || out.Ledger[1].Quantity != 3 {
		t.Fatalf("ledger[1] = %+v, want signup_bonus +3", out.Ledger[1])
	}
}

// TestGetBalanceHandler_EmptyLedgerReturnsZeroBalance proves a brand-new
// Practice with no ledger rows gets a 0 balance and an empty (not null)
// ledger array.
func TestGetBalanceHandler_EmptyLedgerReturnsZeroBalance(t *testing.T) {
	db := testdb.New(t)
	const uid = "get-empty-ledger"
	practiceID := seedMember(t, db, uid)

	srv, session := newBillingServer(t, db, uid)
	defer srv.Close()

	resp := getBalance(t, srv, session, practiceID)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var out billing.BalanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Balance != 0 {
		t.Fatalf("balance = %d, want 0", out.Balance)
	}
	if out.Ledger == nil || len(out.Ledger) != 0 {
		t.Fatalf("ledger = %+v, want empty non-nil slice", out.Ledger)
	}
}
