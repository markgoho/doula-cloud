package portal_test

import (
	"encoding/json/v2"
	"testing"
)

// TestDetailHandler_SaysWhenCareHasEnded is #2217: once the Practice
// marks care complete, "Your care" says so, in the register's one label
// for `completed` (ADR-0015: "Care ended", true for every Client, after a
// loss included). Only that fact crosses: careEnded, a boolean, never the
// status itself, so no other care phase reaches the hub (#2007). Each
// completed row is seeded with a loss, the outcome Nadia Haddad's walk
// found it on, and a live birth beside it, and neither outcome crosses.
func TestDetailHandler_SaysWhenCareHasEnded(t *testing.T) {
	tests := []struct {
		name    string
		status  string
		outcome string
		want    bool
	}{
		{"care still going", "active", "", false},
		{"care ended after a loss", "completed", outcomeLoss, true},
		{"care ended after a live birth", "completed", outcomeLiveBirth, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnvWithOutcome(t, tt.outcome, "2027-03-04")
			e.Exec(`UPDATE engagements SET status = $2::engagement_status,
				        ending_reason = CASE WHEN $2 = 'completed' THEN 'care_complete'::engagement_ending_reason END
				 WHERE id = $1`,
				e.Engagement.ID, tt.status)

			fields := e.detailFields()
			var careEnded bool
			if err := json.Unmarshal(fields["careEnded"], &careEnded); err != nil {
				t.Fatalf("careEnded = %s, want a boolean: %v", fields["careEnded"], err)
			}
			if careEnded != tt.want {
				t.Fatalf("careEnded = %v, want %v", careEnded, tt.want)
			}
			for _, key := range []string{"status", "birthOutcome", "endingReason"} {
				if _, present := fields[key]; present {
					t.Fatalf("portal detail carries %q; only careEnded crosses", key)
				}
			}
		})
	}
}
