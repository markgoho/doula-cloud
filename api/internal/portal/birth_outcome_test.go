package portal_test

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/testdb"
)

// TestDetailHandler_NeverShowsTheBirthOutcome is ADR-0015's "staff-only,
// never Client-facing" made checkable (#293). Nadia Haddad does not need
// the software to tell her what happened; she needs it to stop asking
// her about a pregnancy -- so the birth outcome and the date it ended
// are recorded on her Engagement and reach no portal response at all,
// not even as a null field.
func TestDetailHandler_NeverShowsTheBirthOutcome(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "portal-birth-outcome-uid"
	engagementID, _ := seedClientAtPracticeWithDueDate(t, db, identityUID, "Riverside Doulas", "2027-06-15")
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE engagements SET birth_outcome = 'loss', pregnancy_ended_on = '2027-03-04'::date WHERE id = $1`,
		engagementID); err != nil {
		t.Fatalf("record the birth outcome: %v", err)
	}

	srv, session := newServer(t, db, identityUID)
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

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	for _, key := range []string{"birthOutcome", "pregnancyEndedOn"} {
		if _, present := fields[key]; present {
			t.Fatalf("portal detail carries %q; ADR-0015 keeps the birth outcome staff-only", key)
		}
	}
}
