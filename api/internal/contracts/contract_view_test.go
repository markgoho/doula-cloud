package contracts_test

import (
	"testing"

	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// contractReader resolves a staffauth.Reader for staffID at practiceID --
// ReadContract takes a Reader, not a bare role list, so these tests build
// one the same way a real handler does.
func contractReader(t *testing.T, db *testdb.DB, practiceID, staffID string) staffauth.Reader {
	t.Helper()
	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set practice id: %v", err)
	}
	reader, err := staffauth.ResolveReader(t.Context(), tx, practiceID, staffID)
	if err != nil {
		t.Fatalf("ResolveReader: %v", err)
	}
	return reader
}

var viewFullProse = "Agreement for {{client_name}} at {{money_price}}, on-call terms {{scope_of_service}}."

func fullContractResponse() contracts.ContractResponse {
	return contracts.ContractResponse{
		EngagementID: "engagement-1",
		Status:       "draft",
		Prose:        viewFullProse,
		MergeFields:  []string{"client_name", "money_price", "scope_of_service"},
		Values: contracts.MergeFieldValues{
			"client_name":      "Jamie",
			"money_price":      testPriceValue,
			"scope_of_service": testScopeOfService,
		},
	}
}

// TestReadContract_OwnerAndAdminGetMoney proves ADR-0008's read-table
// money row for both roles that hold it: ContractFull, with the
// money-tagged merge field's value reachable.
func TestReadContract_OwnerAndAdminGetMoney(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Contract View Test Practice")

	ownerID := seedStaff(t, db, "view-owner")
	seedMembership(t, db, practiceID, ownerID, "{owner}")
	adminID := seedStaff(t, db, "view-admin")
	seedMembership(t, db, practiceID, adminID, "{admin}")

	for _, staffID := range []string{ownerID, adminID} {
		reader := contractReader(t, db, practiceID, staffID)
		view := contracts.ReadContract(reader, fullContractResponse())
		full, ok := view.(contracts.ContractFull)
		if !ok {
			t.Fatalf("ReadContract() type = %T, want contracts.ContractFull", view)
		}
		if full.MoneyValues["money_price"] != testPriceValue {
			t.Fatalf("MoneyValues[money_price] = %q, want %s", full.MoneyValues["money_price"], testPriceValue)
		}
		if _, present := full.Values["money_price"]; present {
			t.Fatal("Values carries the money key too -- it must live only in MoneyValues")
		}
		if full.Values["scope_of_service"] != testScopeOfService {
			t.Fatalf("scope value missing from Full's own Values")
		}
	}
}

// TestReadContract_DoulaNeverGetsMoney proves the type-level guarantee the
// AC asks for -- "no price key reachable, not just a redacted value" --
// holds for an employee Doula, who has full scope reach but ADR-0008
// never gives Contract money.
func TestReadContract_DoulaNeverGetsMoney(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Contract View Doula Practice")
	doulaID := seedStaff(t, db, "view-doula")
	seedMembership(t, db, practiceID, doulaID, "{doula}")

	reader := contractReader(t, db, practiceID, doulaID)
	view := contracts.ReadContract(reader, fullContractResponse())

	scope, ok := view.(contracts.ContractScope)
	if !ok {
		t.Fatalf("ReadContract() type = %T, want contracts.ContractScope (no money field reachable at all)", view)
	}
	if _, present := scope.Values["money_price"]; present {
		t.Fatal("ContractScope.Values carries the money key -- it must never be reachable outside ContractFull")
	}
	if scope.Values["scope_of_service"] != testScopeOfService {
		t.Fatal("scope value missing from Doula's own Values")
	}
	if len(scope.MergeFields) != 3 {
		t.Fatalf("MergeFields = %v, want all 3 keys named (values, not names, are what's gated)", scope.MergeFields)
	}
}
