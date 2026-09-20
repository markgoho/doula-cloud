package oncall_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/oncall"
	"doula-cloud/api/internal/testdb"
)

// gapEditFixture is the solo fixture with a backup on the birth and one
// uncovered gap already saved through the API.
type gapEditFixture struct {
	soloFixture
	backupID, gapID, ownerID string
}

func newGapEditFixture(t *testing.T, db *testdb.DB, prefix string) gapEditFixture {
	t.Helper()
	f := gapEditFixture{soloFixture: newSoloFixture(t, db, prefix)}
	f.ownerID = countOwner(t, db, f.soloFixture)
	f.backupID = testdb.SeedNamedStaffAtPractice(t, db, f.practiceID, prefix+"-backup", "Bo Backup", []string{doulaRole}, employeeType)
	testdb.SeedGrantedAttachment(t, db, f.engagementID, f.backupID)

	srv, session := newServer(t, db, f.ownerUID)
	defer srv.Close()
	f.gapID = decode[oncall.Gap](t, authedBody(t, session, http.MethodPost, gapsURL(srv.URL, f.practiceID, f.engagementID), gapBody(f.doulaID, nil)), http.StatusCreated).ID
	return f
}

func (f gapEditFixture) gapURL(srvURL string) string {
	return gapsURL(srvURL, f.practiceID, f.engagementID) + "/" + f.gapID
}

func TestUpdateGap_AddingCoverQueuesNothingAndIsRecorded(t *testing.T) {
	db := testdb.New(t)
	f := newGapEditFixture(t, db, "gap-edit-cover")
	exec(t, db, `UPDATE coverage_gap_outbox SET status = 'sent', sent_at = now()`)
	srv, session, enq := newServerWithNudge(t, db, f.ownerUID)
	defer srv.Close()

	gap := decode[oncall.Gap](t, authedBody(t, session, http.MethodPut, f.gapURL(srv.URL), gapBody(f.doulaID, &f.backupID)), http.StatusOK)
	if gap.CoveringStaffID == nil || *gap.CoveringStaffID != f.backupID {
		t.Fatalf("covering = %v, want the backup", gap.CoveringStaffID)
	}
	if n := countRows(t, db, `SELECT count(*) FROM coverage_gap_outbox WHERE gap_id = $1`, f.gapID); n != 1 || len(enq.Calls()) != 0 {
		t.Fatalf("adding cover queued: %d rows (want the original 1) and %d nudges (want 0)", n, len(enq.Calls()))
	}
	if got := activityActors(t, db, f.engagementID, "coverage_gap_updated"); len(got) != 1 || got[0] != f.ownerID {
		t.Fatalf("updated entries = %v, want one naming the Owner", got)
	}
}

func TestUpdateGap_SavingItStillUncoveredQueuesAgain(t *testing.T) {
	db := testdb.New(t)
	f := newGapEditFixture(t, db, "gap-edit-uncovered")
	exec(t, db, `UPDATE coverage_gap_outbox SET status = 'sent', sent_at = now()`)
	srv, session, enq := newServerWithNudge(t, db, f.ownerUID)
	defer srv.Close()

	decode[oncall.Gap](t, authedBody(t, session, http.MethodPut, f.gapURL(srv.URL), gapBody(f.doulaID, nil)), http.StatusOK)
	if n := pendingNotices(t, db, f.gapID); n != 1 || len(enq.Calls()) != 1 {
		t.Fatalf("an uncovered save queued %d pending and %d nudges, want 1 and 1", n, len(enq.Calls()))
	}
}

func TestUpdateGap_TwoSavesBeforeTheMailGoesAreOneNotice(t *testing.T) {
	db := testdb.New(t)
	f := newGapEditFixture(t, db, "gap-edit-dedupe")
	srv, session, enq := newServerWithNudge(t, db, f.ownerUID)
	defer srv.Close()

	decode[oncall.Gap](t, authedBody(t, session, http.MethodPut, f.gapURL(srv.URL), gapBody(f.doulaID, nil)), http.StatusOK)
	if n := pendingNotices(t, db, f.gapID); n != 1 || len(enq.Calls()) != 0 {
		t.Fatalf("a second save before the send left %d pending and %d nudges, want 1 and 0", n, len(enq.Calls()))
	}
}

func TestUpdateGap_ADoulaCannotEditAColleaguesGap(t *testing.T) {
	db := testdb.New(t)
	f := newGapEditFixture(t, db, "gap-edit-colleague")
	srv, session := newServer(t, db, "gap-edit-colleague-backup")
	defer srv.Close()

	decode[map[string]any](t, authedBody(t, session, http.MethodPut, f.gapURL(srv.URL), gapBody(f.doulaID, &f.backupID)), http.StatusForbidden)
	decode[map[string]any](t, authedBody(t, session, http.MethodDelete, f.gapURL(srv.URL), nil), http.StatusForbidden)
}

func TestClearGap_ClearsOnceAndRecordsIt(t *testing.T) {
	db := testdb.New(t)
	f := newGapEditFixture(t, db, "gap-clear")
	srv, session := newServer(t, db, f.ownerUID)
	defer srv.Close()

	resp := authedBody(t, session, http.MethodDelete, f.gapURL(srv.URL), nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("clear status = %d, want 204", resp.StatusCode)
	}
	if n := countRows(t, db, `SELECT count(*) FROM engagement_coverage_gaps WHERE id = $1 AND cleared_by = $2 AND cleared_at IS NOT NULL`, f.gapID, f.ownerID); n != 1 {
		t.Fatal("the gap row was not cleared by the Owner; a clear must keep the row")
	}
	if got := activityActors(t, db, f.engagementID, "coverage_gap_cleared"); len(got) != 1 || got[0] != f.ownerID {
		t.Fatalf("cleared entries = %v, want one naming the Owner", got)
	}

	decode[map[string]any](t, authedBody(t, session, http.MethodDelete, f.gapURL(srv.URL)+"?again", nil), http.StatusNotFound)
	decode[map[string]any](t, authedBody(t, session, http.MethodPut, f.gapURL(srv.URL), gapBody(f.doulaID, nil)), http.StatusNotFound)
}
