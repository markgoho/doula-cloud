package attachmentstate_test

import (
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/testdb"
)

// TestSchema_RefusesTheIllegalCombinations proves 00173's constraints and
// its one-way trigger hold even for a writer that bypasses every route
// and every policy: the superuser connection.
func TestSchema_RefusesTheIllegalCombinations(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	owner := fixture.Staff(t, db, p, fixture.Owner())
	doula := fixture.Staff(t, db, p, fixture.Doula())
	open := fixture.Engagement(t, db, p).ID
	ended := fixture.Engagement(t, db, p).ID
	testdb.SeedGrantedAttachment(t, db, open, doula.ID)
	testdb.SeedAttachment(t, db, ended, doula.ID, "granted", true)
	fresh := fixture.Engagement(t, db, p).ID

	testdb.RequireRefused(t, db, "", map[string]testdb.Write{
		"an ending naming nobody": {
			Query: `UPDATE engagement_attachments SET ended_at = now(), ended_reason = 'taken_off' WHERE engagement_id = $1`, Args: []any{open}},
		"an ending with no reason": {
			Query: `UPDATE engagement_attachments SET ended_at = now(), ended_by = $2 WHERE engagement_id = $1`, Args: []any{open, owner.ID}},
		"a reason on an open attachment": {
			Query: `UPDATE engagement_attachments SET ended_reason = 'taken_off' WHERE engagement_id = $1`, Args: []any{open}},
		"an accrual someone else wrote": {
			Query: `INSERT INTO engagement_attachments (engagement_id, staff_id, origin, attached_by) VALUES ($1, $2, 'accrued', $3)`, Args: []any{fresh, doula.ID, owner.ID}},
		"a fee on an accrued attachment": {
			Query: `INSERT INTO engagement_attachments (engagement_id, staff_id, origin, attached_by, fee_amount_cents) VALUES ($1, $2, 'accrued', $2, 100)`, Args: []any{fresh, doula.ID}},
		"a granted attachment going back to accrued": {
			Query: `UPDATE engagement_attachments SET origin = 'accrued' WHERE engagement_id = $1`, Args: []any{open}},
		"an ended attachment reopened": {
			Query: `UPDATE engagement_attachments SET ended_at = NULL, ended_by = NULL, ended_reason = NULL WHERE engagement_id = $1`, Args: []any{ended}},
		"an ended attachment ended again": {
			Query: `UPDATE engagement_attachments SET ended_at = now() + interval '1 day', ended_reason = 'membership_ended' WHERE engagement_id = $1`, Args: []any{ended}},
		"an attachment moved to another Doula": {
			Query: `UPDATE engagement_attachments SET staff_id = $2 WHERE engagement_id = $1`, Args: []any{open, owner.ID}},
		"a second open attachment for the pair": {
			Query: `INSERT INTO engagement_attachments (engagement_id, staff_id, origin, attached_by) VALUES ($1, $2, 'accrued', $2)`, Args: []any{open, doula.ID}},
		"a state written by hand": {
			Query: `UPDATE engagement_attachments SET state = 'ended' WHERE engagement_id = $1`, Args: []any{open}},
	})

	// And the legal ones still pass: an attributed, explained ending, and
	// a new attachment for a pair whose last one ended.
	testdb.RequireAccepted(t, db, map[string]testdb.Write{
		"an attributed ending":                {Query: `UPDATE engagement_attachments SET ended_at = now(), ended_by = $2, ended_reason = 'taken_off' WHERE engagement_id = $1`, Args: []any{open, owner.ID}},
		"a new attachment after an ended one": {Query: `INSERT INTO engagement_attachments (engagement_id, staff_id, origin, attached_by) VALUES ($1, $2, 'granted', $3)`, Args: []any{ended, doula.ID, owner.ID}},
	})
}

// TestHolderLeft_IsTheOneDefinition proves engagement_holder_left (00173):
// she has left when she holds no open Attachment and a granted one has
// ended -- not while an accrued one is still open, and not when the only
// one that ended was accrued.
func TestHolderLeft_IsTheOneDefinition(t *testing.T) {
	db := testdb.New(t)
	p := fixture.Practice(t, db)
	doula := fixture.Staff(t, db, p, fixture.Doula())
	cases := []struct {
		name  string
		seed  func(engagementID string)
		wants bool
	}{
		{"never on it", func(string) {}, false},
		{"on it now", func(e string) { testdb.SeedGrantedAttachment(t, db, e, doula.ID) }, false},
		{"taken off", func(e string) { testdb.SeedAttachment(t, db, e, doula.ID, "granted", true) }, true},
		{"taken off but still working it", func(e string) {
			testdb.SeedAttachment(t, db, e, doula.ID, "granted", true)
			testdb.SeedAttachment(t, db, e, doula.ID, "accrued", false)
		}, false},
		{"only ever accrued, and that ended", func(e string) { testdb.SeedAttachment(t, db, e, doula.ID, "accrued", true) }, false},
	}
	for _, tc := range cases {
		e := fixture.Engagement(t, db, p).ID
		tc.seed(e)
		tx := db.BeginAs(t, testdb.Practice(p.ID))
		got := testdb.Scalar[bool](t, tx, `SELECT engagement_holder_left($1, $2)`, e, doula.ID)
		_ = tx.Rollback()
		if got != tc.wants {
			t.Errorf("%s: engagement_holder_left = %v, want %v", tc.name, got, tc.wants)
		}
	}
}
