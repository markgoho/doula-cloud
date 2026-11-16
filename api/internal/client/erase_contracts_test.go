package client_test

import (
	"slices"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/testdb"
)

// TestEraseHandler_VoidsHerUnsignedContracts proves erasure voids every
// Contract of hers that is not signed, in the transaction that redacts her
// (#2990): a Draft and a Sent one are voided, as the act's own entries on
// their Engagements, by the Owner who erased her, and a signed one and one
// already voided are untouched.
func TestEraseHandler_VoidsHerUnsignedContracts(t *testing.T) {
	e := newEraseEnv(t)
	unsigned := []string{contractDraft, contractSent}
	byStatus := map[string]string{}
	engagements := map[string]string{}
	for _, status := range append(slices.Clone(unsigned), contractSigned, contractVoided) {
		engagements[status] = e.activeEngagement()
		byStatus[status] = seedClientContract(t, e.DB, engagements[status], status)
	}
	voidedAt := testdb.Scalar[string](t, e.DB.Admin, `SELECT voided_at::text FROM contracts WHERE id = $1`, byStatus[contractVoided])

	e.erase()

	for status, id := range byStatus {
		want := status
		if slices.Contains(unsigned, status) {
			want = contractVoided
		}
		if got := testdb.Scalar[string](t, e.DB.Admin, `SELECT status::text FROM contracts WHERE id = $1`, id); got != want {
			t.Errorf("%s Contract status after erasure = %q, want %q", status, got, want)
		}
	}
	for _, status := range unsigned {
		var byErasure bool
		var voidedBy *string
		var sameInstant bool
		if err := e.DB.Admin.QueryRowContext(t.Context(),
			`SELECT c.voided_by_erasure, c.voided_by::text, c.voided_at = cl.erased_at
			   FROM contracts c JOIN engagements en ON en.id = c.engagement_id JOIN clients cl ON cl.id = en.client_id
			  WHERE c.id = $1`, byStatus[status]).Scan(&byErasure, &voidedBy, &sameInstant); err != nil {
			t.Fatalf("read %s Contract: %v", status, err)
		}
		if !byErasure || voidedBy != nil || !sameInstant {
			t.Errorf("%s Contract: voided_by_erasure = %v, voided_by = %v, at the erasure's instant = %v; want true, nil, true", status, byErasure, voidedBy, sameInstant)
		}
		var actor, by string
		if err := e.DB.Admin.QueryRowContext(t.Context(),
			`SELECT actor_kind::text, actor_staff_id::text FROM activity
			  WHERE subject_kind = 'engagement' AND subject_id = $1 AND action = $2`,
			engagements[status], string(activity.ActionContractVoidedByErasure)).Scan(&actor, &by); err != nil {
			t.Fatalf("read %s Contract's Activity: %v", status, err)
		}
		if actor != string(activity.ActorStaff) || by != e.Staff.ID {
			t.Errorf("%s Contract's Activity actor = %s %s, want the Owner %s", status, actor, by, e.Staff.ID)
		}
	}
	for _, status := range []string{contractSigned, contractVoided} {
		if n := testdb.Scalar[int](t, e.DB.Admin,
			`SELECT count(*) FROM activity WHERE subject_kind = 'engagement' AND subject_id = $1 AND action = $2`,
			engagements[status], string(activity.ActionContractVoidedByErasure)); n != 0 {
			t.Errorf("%s Contract has %d erasure void entries, want 0", status, n)
		}
	}
	if got := testdb.Scalar[string](t, e.DB.Admin, `SELECT voided_at::text FROM contracts WHERE id = $1`, byStatus[contractVoided]); got != voidedAt {
		t.Errorf("already-voided Contract's voided_at = %s, want it kept at %s", got, voidedAt)
	}
	if voided := testdb.Scalar[bool](t, e.DB.Admin, `SELECT voided_by_erasure FROM contracts WHERE id = $1`, byStatus[contractVoided]); voided {
		t.Error("an already-voided Contract was marked voided by erasure")
	}

	var scope struct {
		ContractsVoided int `json:"contractsVoided"`
	}
	e.readErasureScope(&scope)
	if scope.ContractsVoided != 2 {
		t.Errorf("erasureScope.contractsVoided = %d, want 2", scope.ContractsVoided)
	}
}
