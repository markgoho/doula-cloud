package portal_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/testdb"
)

// The birth outcomes these tests seed, as the birth_outcome enum spells them.
const (
	outcomeLoss      = "loss"
	outcomeLiveBirth = "live_birth"
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
		uid     string
		status  string
		outcome string
		want    bool
	}{
		{"care still going", "portal-care-going-uid", "active", "", false},
		{"care ended after a loss", "portal-care-ended-loss-uid", "completed", outcomeLoss, true},
		{"care ended after a live birth", "portal-care-ended-live-uid", "completed", outcomeLiveBirth, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testdb.New(t)
			engagementID, _ := seedClientAtPracticeWithDueDate(t, db, tt.uid, "Riverside Doulas", "2027-06-15")
			if tt.outcome != "" {
				if _, err := db.Admin.ExecContext(t.Context(),
					`UPDATE engagements SET birth_outcome = $2::birth_outcome, pregnancy_ended_on = '2027-03-04'::date WHERE id = $1`,
					engagementID, tt.outcome); err != nil {
					t.Fatalf("record the birth outcome: %v", err)
				}
			}
			if _, err := db.Admin.ExecContext(t.Context(),
				`UPDATE engagements SET status = $2::engagement_status,
				        ending_reason = CASE WHEN $2 = 'completed' THEN 'care_complete'::engagement_ending_reason END
				 WHERE id = $1`,
				engagementID, tt.status); err != nil {
				t.Fatalf("move the status: %v", err)
			}

			srv, session := newServer(t, db, tt.uid)
			defer srv.Close()
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/api/portal/engagements/"+engagementID, nil)
			if err != nil {
				t.Fatalf("build request: %v", err)
			}
			authntest.AddSessionCookie(req, session)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
			}
			var fields map[string]json.RawMessage
			if err := json.NewDecoder(resp.Body).Decode(&fields); err != nil {
				t.Fatalf("decode response: %v", err)
			}
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
