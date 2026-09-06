package contracts_test

import (
	"testing"

	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/staffauth"
)

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
	readers := []staffauth.Reader{
		staffauth.NewReader("view-owner", []string{"owner"}, "employee"),
		staffauth.NewReader("view-admin", []string{"admin"}, "employee"),
	}
	for _, reader := range readers {
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
	reader := staffauth.NewReader("view-doula", []string{doulaRole}, "employee")
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
