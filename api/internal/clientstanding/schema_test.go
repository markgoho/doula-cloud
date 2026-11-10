package clientstanding_test

import (
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/testdb"
)

// TestSchema_RefusesTheIllegalCombinations proves 00171's constraints
// hold even for a writer that bypasses every route and every policy: the
// superuser connection.
func TestSchema_RefusesTheIllegalCombinations(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	owner := fixture.Staff(t, db, p, fixture.Owner())
	c := fixture.Client(t, db, p)
	other := fixture.Client(t, db, p)

	testdb.RequireRefused(t, db, "", map[string]testdb.Write{
		"an erasure with no author": {
			Query: `UPDATE clients SET erased_at = now() WHERE id = $1`, Args: []any{c.ID}},
		"a staff erasure naming no Staff member": {
			Query: `UPDATE clients SET erased_at = now(), erased_by_kind = 'staff' WHERE id = $1`, Args: []any{c.ID}},
		"a system erasure naming a Staff member": {
			Query: `UPDATE clients SET erased_at = now(), erased_by_kind = 'system', erased_by_staff_id = $2 WHERE id = $1`, Args: []any{c.ID, owner.ID}},
		"an erasure by the Client herself": {
			Query: `UPDATE clients SET erased_at = now(), erased_by_kind = 'client' WHERE id = $1`, Args: []any{c.ID}},
		"an author on a record that is not erased": {
			Query: `UPDATE clients SET erased_by_kind = 'system' WHERE id = $1`, Args: []any{c.ID}},
		"a merge author with no merge": {
			Query: `UPDATE clients SET merged_by_staff_id = $2 WHERE id = $1`, Args: []any{c.ID, owner.ID}},
		"a record merged into itself": {
			Query: `UPDATE clients SET merged_into = id, merged_at = now() WHERE id = $1`, Args: []any{c.ID}},
		"a standing written by hand": {
			Query: `UPDATE clients SET standing = 'erased' WHERE id = $1`, Args: []any{c.ID}},
	})

	// And the legal ones still pass: an Owner's erasure and a dated merge.
	testdb.RequireAccepted(t, db, map[string]testdb.Write{
		"an attributed erasure":     {Query: `UPDATE clients SET erased_at = now(), erased_by_kind = 'staff', erased_by_staff_id = $2 WHERE id = $1`, Args: []any{c.ID, owner.ID}},
		"a dated, attributed merge": {Query: `UPDATE clients SET merged_into = $2, merged_at = now(), merged_by_staff_id = $3 WHERE id = $1`, Args: []any{other.ID, c.ID, owner.ID}},
	})
}

// TestSchema_OnlyALiveRecordIsWritten proves clients_update (00171):
// under the Practice's own row security, an erased record and a tombstone
// match no row at all, while a live one is written.
func TestSchema_OnlyALiveRecordIsWritten(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	live := fixture.Client(t, db, p)
	erased := fixture.Client(t, db, p)
	merged := fixture.Client(t, db, p)
	fixture.EraseClient(t, db, erased.ID)
	fixture.MergeClient(t, db, merged.ID, live.ID)

	tx := db.BeginAs(t, testdb.Practice(p.ID))
	defer func() { _ = tx.Rollback() }()
	for id, want := range map[string]int64{live.ID: 1, erased.ID: 0, merged.ID: 0} {
		res, err := tx.ExecContext(t.Context(), `UPDATE clients SET phone = '555-0100' WHERE id = $1`, id)
		if err != nil {
			t.Fatalf("update %s: %v", id, err)
		}
		if n, _ := res.RowsAffected(); n != want {
			t.Errorf("update %s touched %d rows, want %d", id, n, want)
		}
	}
}
