// PROTOTYPE -- throwaway. Exercises Shape B (prototype_view.go +
// staffauth/prototype_reader.go) against the case Shape A cannot express:
// one Contract, two shapes, money stripped for a Doula. Contrast with
// prototype_mount_test.go's whole-endpoint cases.
package contracts_test

import (
	"testing"

	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

func seedPracticeForView(t *testing.T, db *testdb.DB) (practiceID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(), `INSERT INTO practices (name) VALUES ('Prototype View Practice') RETURNING id`).Scan(&practiceID); err != nil {
		t.Fatalf("seed practice: %v", err)
	}
	return practiceID
}

// seedStaffWithRoles adds a Staff member with roles to the already-seeded
// practiceID -- every caller must share one Practice, or ResolveReader
// looks up a membership under the wrong practice_id and RLS correctly
// hides it.
func seedStaffWithRoles(t *testing.T, db *testdb.DB, practiceID, identityUID, roles string) (staffID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff (identity_uid, name, email) VALUES ($1, 'Test Staff', $1 || '@example.com') RETURNING id`, identityUID,
	).Scan(&staffID); err != nil {
		t.Fatalf("seed staff: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_memberships (practice_id, staff_id, roles) VALUES ($1, $2, $3::practice_role[])`,
		practiceID, staffID, roles,
	); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	return staffID
}

// fullContract is a stand-in for what fetchContract would return: scope
// fields plus a money field, mirroring a real Contract's merge field
// values.
func fullContract() contracts.ContractResponse {
	return contracts.ContractResponse{
		EngagementID: "engagement-1",
		Status:       "sent",
		MergeFields:  []string{"visit_count", "due_date", "price"},
		Values: contracts.MergeFieldValues{
			"visit_count": "4",
			"due_date":    "2027-03-01",
			"price":       "4200.00",
		},
	}
}

// TestReadContract_OwnerGetsMoney_DoulaDoesNot is the Contract case
// ADR-0006 calls the sharpest instance: one record, two shapes, decided by
// the type ReadContract hands back rather than a field the caller might
// forget to strip.
func TestReadContract_OwnerGetsMoney_DoulaDoesNot(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPracticeForView(t, db)
	ownerID := seedStaffWithRoles(t, db, practiceID, "prototype-owner", "{owner}")
	doulaID := seedStaffWithRoles(t, db, practiceID, "prototype-doula", "{doula}")

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set practice: %v", err)
	}

	ownerReader, err := staffauth.ResolveReader(t.Context(), tx, practiceID, ownerID)
	if err != nil {
		t.Fatalf("resolve owner reader: %v", err)
	}
	doulaReader, err := staffauth.ResolveReader(t.Context(), tx, practiceID, doulaID)
	if err != nil {
		t.Fatalf("resolve doula reader: %v", err)
	}

	full := fullContract()

	ownerView := contracts.ReadContract(ownerReader, full)
	if _, ok := ownerView.(contracts.ContractFull); !ok {
		t.Fatalf("owner view = %T, want ContractFull", ownerView)
	}
	ownerFull := ownerView.(contracts.ContractFull)
	if ownerFull.MoneyValues["price"] != "4200.00" {
		t.Fatalf("owner did not receive price, got %q", ownerFull.MoneyValues["price"])
	}
	if ownerFull.Values["visit_count"] != "4" {
		t.Fatalf("owner did not receive scope, got %q", ownerFull.Values["visit_count"])
	}

	doulaView := contracts.ReadContract(doulaReader, full)
	scope, ok := doulaView.(contracts.ContractScope)
	if !ok {
		t.Fatalf("doula view = %T, want ContractScope (no money field reachable at all)", doulaView)
	}
	if scope.Values["visit_count"] != "4" {
		t.Fatalf("doula did not receive scope, got %q", scope.Values["visit_count"])
	}
	if _, present := scope.Values["price"]; present {
		t.Fatal("doula's Values carries the price key -- money leaked into scope")
	}
}
