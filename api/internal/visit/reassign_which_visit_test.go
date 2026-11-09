package visit_test

import (
	"encoding/json/v2"
	"testing"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/testdb"
)

// #2666: a reassignment's Activity entry says which Visit moved: its
// scheduled instant at the time (a JSON null for a Visit with no date) and
// the instant it was added, so two reassigned Visits leave two different
// entries, an undated pair included.
func TestReassign_RecordsWhichVisitMoved(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	from, to := e.Colleague(), e.Colleague()
	when := time.Date(2026, 11, 13, 21, 0, 0, 0, time.UTC)
	dated := seedScheduledVisit(t, e.DB, e.Engagement.ID, from.ID, when)
	undatedA := seedVisit(t, e.DB, e.Engagement.ID, from.ID)
	undatedB := seedVisit(t, e.DB, e.Engagement.ID, from.ID)

	for _, id := range []string{dated, undatedA, undatedB} {
		e.reassignOK(id, to.ID)
	}

	raws := testdb.Column[[]byte](t, e.DB.Admin,
		`SELECT diff FROM activity WHERE subject_id = $1 AND action = $2 ORDER BY created_at, id`,
		e.Engagement.ID, string(activity.ActionVisitReassigned))
	var diffs []map[string]any
	for _, raw := range raws {
		var diff map[string]any
		if err := json.Unmarshal(raw, &diff); err != nil {
			t.Fatalf("decode diff: %v", err)
		}
		diffs = append(diffs, diff)
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
