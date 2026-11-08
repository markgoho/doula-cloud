package visit_test

import (
	"encoding/json/v2"
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/visit"
)

// #2666: a reassignment's Activity entry says which Visit moved: its
// scheduled instant at the time (a JSON null for a Visit with no date) and
// the instant it was added, so two reassigned Visits leave two different
// entries, an undated pair included.
func TestReassign_RecordsWhichVisitMoved(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "owner-which-visit"
	practiceID, _ := testdb.SeedStaffAtNewPractice(t, db, identityUID, []string{ownerRole}, "employee")
	from := testdb.SeedStaffAtPractice(t, db, practiceID, "doula-which-from", []string{doulaRole}, "employee")
	to := testdb.SeedStaffAtPractice(t, db, practiceID, "doula-which-to", []string{doulaRole}, "employee")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	when := time.Date(2026, 11, 13, 21, 0, 0, 0, time.UTC)
	dated := seedScheduledVisit(t, db, engagementID, from, when)
	undatedA := seedVisit(t, db, engagementID, from)
	undatedB := seedVisit(t, db, engagementID, from)

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()
	url := visitsURL(srv.URL, practiceID, engagementID)
	body, err := json.Marshal(visit.ReassignRequest{StaffID: to})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	for _, id := range []string{dated, undatedA, undatedB} {
		resp := authedPatch(t, session, url+"/"+id, body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("reassign %s status = %d", id, resp.StatusCode)
		}
	}

	rows, err := db.Admin.QueryContext(t.Context(),
		`SELECT diff FROM activity WHERE subject_id = $1 AND action = $2 ORDER BY created_at, id`,
		engagementID, string(activity.ActionVisitReassigned))
	if err != nil {
		t.Fatalf("read activity: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var diffs []map[string]any
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			t.Fatalf("scan: %v", err)
		}
		var diff map[string]any
		if err := json.Unmarshal(raw, &diff); err != nil {
			t.Fatalf("decode diff: %v", err)
		}
		diffs = append(diffs, diff)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate activity: %v", err)
	}
	if len(diffs) != 3 {
		t.Fatalf("entries = %d, want 3", len(diffs))
	}

	got, ok := diffs[0][activity.DiffKeyVisitScheduledAt].(string)
	if !ok {
		t.Fatalf("dated entry scheduled = %v, want an instant", diffs[0][activity.DiffKeyVisitScheduledAt])
	}
	if parsed, err := time.Parse(time.RFC3339, got); err != nil || !parsed.Equal(when) {
		t.Fatalf("dated entry scheduled = %q (%v), want %v", got, err, when)
	}
	created := map[string]bool{}
	for i, diff := range diffs {
		if i > 0 {
			value, present := diff[activity.DiffKeyVisitScheduledAt]
			if !present || value != nil {
				t.Errorf("undated entry %d scheduled = %v (present %v), want an explicit null", i, value, present)
			}
		}
		at, ok := diff[activity.DiffKeyVisitCreatedAt].(string)
		if !ok || at == "" {
			t.Fatalf("entry %d added = %v, want an instant", i, diff[activity.DiffKeyVisitCreatedAt])
		}
		created[at] = true
	}
	if len(created) != 3 {
		t.Errorf("added instants = %v, want three different ones", created)
	}
}
