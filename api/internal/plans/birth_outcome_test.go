package plans_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"doula-cloud/api/internal/plans"
	"doula-cloud/api/internal/testdb"
)

// recordOutcome puts a birth outcome on an Engagement the way Staff's own
// endpoint would, without going through it -- this package tests the
// portal's reading of the fact, not #293's writing of it. It opens
// 00093's correction door for the transaction, because a second call
// against the same Engagement is exactly the correction ADR-0015's
// motivating typo needs and the freeze trigger would otherwise refuse it.
func recordOutcome(t *testing.T, db *testdb.DB, engagementID, outcome string, endedOn *string) {
	t.Helper()
	tx, err := db.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(t.Context(),
		`SELECT set_config('app.allow_outcome_correction', 'on', true)`); err != nil {
		t.Fatalf("open the correction door: %v", err)
	}
	if _, err := tx.ExecContext(t.Context(),
		`UPDATE engagements SET birth_outcome = $2::birth_outcome, pregnancy_ended_on = $3::date WHERE id = $1`,
		engagementID, outcome, endedOn); err != nil {
		t.Fatalf("record the birth outcome %q: %v", outcome, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit the birth outcome %q: %v", outcome, err)
	}
}

// seedBirthPlanClient stands up a birth Engagement with a Birth Plan
// instance already filled in, and a portal session for its Client.
func seedBirthPlanClient(t *testing.T, db *testdb.DB, identityUID string) string {
	t.Helper()
	practiceID := testdb.SeedPractice(t, db, "Practice")
	clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Nadia Haddad", "nadia@example.com")
	testdb.SeedPortalUser(t, db, testdb.PortalUID(identityUID), clientID)
	seedInstance(t, db, engagementID, birthPlanType,
		`[{"id":"location","type":"single_select","label":"Planned birth location","options":["Home","Hospital"],"order":0}]`,
		`{"location":"Hospital"}`,
	)
	return engagementID
}

// TestClientBirthPlan_SuppressionIsDerivedNotStored is #294's whole
// claim in one walk. Nadia Haddad's pregnancy ends in a loss, and the
// Birth Plan stops being offered to her at the API -- read, PDF and
// acknowledgement alike -- with nothing on the Plan Instance changed to
// make that happen. The outcome is then corrected, as ADR-0015's own
// motivating typo requires, and the plan comes back with the same
// answers it always had: no retirement to reverse, because nothing was
// ever retired.
func TestClientBirthPlan_SuppressionIsDerivedNotStored(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-birth-plan-after-loss"
	engagementID := seedBirthPlanClient(t, db, identityUID)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	assertOffered := func(when string) {
		t.Helper()
		resp := getClientBirthPlan(t, srv, session, engagementID)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: GET status = %d, want %d", when, resp.StatusCode, http.StatusOK)
		}
		var out plans.InstanceResponse
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("%s: decode response: %v", when, err)
		}
		// The Plan Instance itself is untouched throughout: the same
		// answers Staff wrote, still readable and still whole.
		if out.Answers["location"] != hospital {
			t.Fatalf("%s: answers = %v, want the Plan Instance untouched", when, out.Answers)
		}
	}

	assertOffered("before any outcome is recorded")

	endedOn := "2027-03-04"
	recordOutcome(t, db, engagementID, "loss", &endedOn)

	resp := getClientBirthPlan(t, srv, session, engagementID)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("after a loss: GET status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}

	pdf := getInstancePDF(t, srv.Client(), srv.URL+"/api/portal/engagements/"+engagementID+"/birth-plan/pdf", session)
	defer pdf.Body.Close()
	if pdf.StatusCode != http.StatusNotFound {
		t.Fatalf("after a loss: PDF status = %d, want %d", pdf.StatusCode, http.StatusNotFound)
	}

	ack := postAcknowledgeBirthPlan(t, srv, session, engagementID)
	defer ack.Body.Close()
	if ack.StatusCode != http.StatusNotFound {
		t.Fatalf("after a loss: acknowledge status = %d, want %d", ack.StatusCode, http.StatusNotFound)
	}

	born := "2027-06-10"
	recordOutcome(t, db, engagementID, "live_birth", &born)
	assertOffered("after the outcome is corrected to a live birth")
}

// TestClientBirthPlan_UnknownOutcomeRefused covers ADR-0015's
// presume-nothing value: a Client who withdrew, leaving the Practice
// never learning what happened, meets no Birth Plan either. Separate
// from the loss walk above because 'unknown' carries no date, and the
// reason it refuses is different -- not that there is no baby, but that
// the product must not presume there is one.
func TestClientBirthPlan_UnknownOutcomeRefused(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-birth-plan-unknown-outcome"
	engagementID := seedBirthPlanClient(t, db, identityUID)
	recordOutcome(t, db, engagementID, "unknown", nil)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := getClientBirthPlan(t, srv, session, engagementID)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// TestClientAcknowledgeBirthPlan_PostpartumEngagementRefused closes the
// gap #311 left on the write side: the acknowledgement endpoint carries
// the same gate the read does, so a Client who is not offered a Birth
// Plan cannot stamp client_acknowledged_at on one from a stale tab.
func TestClientAcknowledgeBirthPlan_PostpartumEngagementRefused(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "client-acknowledge-postpartum-only"
	practiceID := testdb.SeedPractice(t, db, "Practice")
	clientID, engagementID := testdb.SeedEngagementWithKind(t, db, practiceID, "Jordan Client", "jordan@example.com", "postpartum")
	testdb.SeedPortalUser(t, db, testdb.PortalUID(identityUID), clientID)
	seedInstance(t, db, engagementID, birthPlanType,
		`[{"id":"location","type":"single_select","label":"Planned birth location","options":["Home","Hospital"],"order":0}]`,
		`{"location":"Hospital"}`,
	)

	srv, session := newPortalServer(t, db, identityUID)
	defer srv.Close()

	resp := postAcknowledgeBirthPlan(t, srv, session, engagementID)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}
