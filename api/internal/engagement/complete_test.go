package engagement_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/testdb"
)

// newCompleteServer mounts this package's whole surface through
// newServer -- the completion route is one of them -- and closes it on
// t.Cleanup rather than making every caller defer srv.Close().
func newCompleteServer(t *testing.T, db *testdb.DB) *httptest.Server {
	t.Helper()
	srv, _ := newServer(t, db, "engagement-complete-mount")
	t.Cleanup(srv.Close)
	return srv
}

// completeAs sends the completion request as uid and returns its status.
func completeAs(t *testing.T, db *testdb.DB, srv *httptest.Server, uid, practiceID, engagementID string) int {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		srv.URL+"/api/practices/"+practiceID+"/engagements/"+engagementID+"/complete", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, authntest.SeedSession(t, db.App, uid))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode
}

// seedOwnerAtPractice inserts an Owner Membership, the role completion
// requires.
func seedOwnerAtPractice(t *testing.T, db *testdb.DB, practiceID, identityUID string) (staffID string) {
	t.Helper()
	return testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{"owner"}, "employee")
}

// Completion is one act with three effects: the Engagement's status, the
// open Offers, and the open attachments.
func TestCompleteHandler_RunsTheWholeCascade(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedStaffWithMembership(t, db, "complete-doula")
	ownerID := seedOwnerAtPractice(t, db, practiceID, "complete-owner")
	_, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com", "active")

	var doulaID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT id FROM staff WHERE identity_uid = 'complete-doula'`).Scan(&doulaID); err != nil {
		t.Fatalf("read doula: %v", err)
	}
	seedGrantedAttachment(t, db, engagementID, doulaID)
	var offerID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagement_offers
		     (engagement_id, staff_id, employment_type, amount_cents, client_first_initial, client_area,
		      due_date, offered_by, expires_at)
		 VALUES ($1, $2, 'contractor', 45000, 'R', 'North side', now() + interval '90 days', $3, now() + interval '7 days')
		 RETURNING id`,
		engagementID, doulaID, ownerID,
	).Scan(&offerID); err != nil {
		t.Fatalf("seed offer: %v", err)
	}

	srv := newCompleteServer(t, db)
	if status := completeAs(t, db, srv, "complete-owner", practiceID, engagementID); status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}

	var engagementStatus, offerStateValue string
	var decidedBy, endedBy *string
	var ended bool
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status::text FROM engagements WHERE id = $1`, engagementID).Scan(&engagementStatus); err != nil {
		t.Fatalf("read engagement: %v", err)
	}
	if engagementStatus != "completed" {
		t.Fatalf("engagement status = %q, want completed", engagementStatus)
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT state::text, decided_by::text FROM engagement_offers WHERE id = $1`, offerID,
	).Scan(&offerStateValue, &decidedBy); err != nil {
		t.Fatalf("read offer: %v", err)
	}
	if offerStateValue != "withdrawn" {
		t.Fatalf("offer state = %q, want withdrawn", offerStateValue)
	}
	if decidedBy != nil {
		t.Fatalf("decided_by = %v, want NULL -- the cascade has no human actor", *decidedBy)
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT ended_at IS NOT NULL, ended_by::text FROM engagement_attachments
		  WHERE engagement_id = $1 AND staff_id = $2`, engagementID, doulaID,
	).Scan(&ended, &endedBy); err != nil {
		t.Fatalf("read attachment: %v", err)
	}
	if !ended || endedBy == nil || *endedBy != ownerID {
		t.Fatalf("attachment ended = %v by %v, want ended by the completer", ended, endedBy)
	}
}

func TestCompleteHandler_RefusesWhatItShould(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedStaffWithMembership(t, db, "complete-refuse-doula")
	seedOwnerAtPractice(t, db, practiceID, "complete-refuse-owner")
	_, engagementID := seedClientEngagement(t, db, practiceID, "Client", "client@example.com", "active")
	srv := newCompleteServer(t, db)

	cases := []struct {
		name         string
		uid          string
		engagementID string
		want         int
	}{
		{"a doula may not complete", "complete-refuse-doula", engagementID, http.StatusForbidden},
		{"unknown engagement", "complete-refuse-owner", "11111111-1111-1111-1111-111111111111", http.StatusNotFound},
		{"engagement id is not a uuid", "complete-refuse-owner", "not-a-uuid", http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if status := completeAs(t, db, srv, tc.uid, practiceID, tc.engagementID); status != tc.want {
				t.Fatalf("status = %d, want %d", status, tc.want)
			}
		})
	}
}
