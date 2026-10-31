package portal_test

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/portal"
	"doula-cloud/api/internal/testdb"
)

// getDetail reads the portal Detail of engagementID as identityUID.
func getDetail(t *testing.T, db *testdb.DB, identityUID, engagementID string) (portal.Detail, map[string]json.RawMessage) {
	t.Helper()
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
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	var out portal.Detail
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	return out, raw
}

// TestDetailHandler_GlancesAbsentWhenNothingExistsYet is #2007's empty
// side: a new Engagement has no Contract, no Birth Plan and no Message,
// and the hub is told nothing rather than an empty object it would have
// to tell apart from a real one.
func TestDetailHandler_GlancesAbsentWhenNothingExistsYet(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "portal-glance-empty"
	engagementID, _ := seedClientAtPracticeWithDueDate(t, db, identityUID, "Riverside Doulas", "2027-06-15")

	_, raw := getDetail(t, db, identityUID, engagementID)

	for _, key := range []string{"contract", "birthPlan", "lastMessage"} {
		if _, present := raw[key]; present {
			t.Fatalf("%q present on an Engagement with nothing yet, want it absent", key)
		}
	}
}

// TestDetailHandler_GlancesCarryHerDocumentsAndThread is #2007's full
// side: the newest Contract she may see with when she signed it, her
// Birth Plan's acknowledgment, and the newest Message with its sender's
// name -- the same name the thread itself gives.
func TestDetailHandler_GlancesCarryHerDocumentsAndThread(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "portal-glance-full"
	engagementID, doulaID := seedEngagementForVisits(t, db, identityUID, "Riverside Doulas", "Priya Raman")

	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO contracts (engagement_id, status, prose, amount_cents, signed_at, created_at)
		 VALUES ($1, 'signed', 'Test prose', 15000, '2026-08-14T15:00:00Z', now() - interval '1 day')`,
		engagementID); err != nil {
		t.Fatalf("seed signed contract: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO plan_instances (engagement_id, plan_type, fields, client_acknowledged_at, changed_at)
		 VALUES ($1, 'birth_plan', '[]'::jsonb, '2026-08-20T12:00:00Z', now())`,
		engagementID); err != nil {
		t.Fatalf("seed birth plan: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO messages (engagement_id, sender_type, sender_id, body, created_at)
		 VALUES ($1, 'staff', $2, 'An older note.', now() - interval '2 days'),
		        ($1, 'staff', $2, 'Thursday works. See you at 2.', now() - interval '1 hour')`,
		engagementID, doulaID); err != nil {
		t.Fatalf("seed messages: %v", err)
	}

	out, _ := getDetail(t, db, identityUID, engagementID)

	if out.Contract == nil || out.Contract.Status != "signed" {
		t.Fatalf("contract = %+v, want a signed Contract", out.Contract)
	}
	if out.Contract.SignedAt == nil || out.Contract.SignedAt.UTC().Format("2006-01-02") != "2026-08-14" {
		t.Fatalf("contract.signedAt = %v, want 2026-08-14", out.Contract.SignedAt)
	}
	if out.BirthPlan == nil || out.BirthPlan.AcknowledgedAt == nil {
		t.Fatalf("birthPlan = %+v, want one acknowledged", out.BirthPlan)
	}
	if out.LastMessage == nil || out.LastMessage.Body != "Thursday works. See you at 2." {
		t.Fatalf("lastMessage = %+v, want the newest Message", out.LastMessage)
	}
	if out.LastMessage.SenderName != "Priya Raman" {
		t.Fatalf("lastMessage.senderName = %q, want %q", out.LastMessage.SenderName, "Priya Raman")
	}
}

// TestDetailHandler_ContractGlanceNeverShowsADraft holds the Contract
// read's own rule on the hub: a Draft is the Practice's, not hers, so a
// Contract still being written reads as no Contract at all.
func TestDetailHandler_ContractGlanceNeverShowsADraft(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "portal-glance-draft"
	engagementID, _ := seedClientAtPracticeWithDueDate(t, db, identityUID, "Riverside Doulas", "2027-06-15")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO contracts (engagement_id, status, prose, amount_cents) VALUES ($1, 'draft', 'Test prose', 15000)`,
		engagementID); err != nil {
		t.Fatalf("seed draft contract: %v", err)
	}

	_, raw := getDetail(t, db, identityUID, engagementID)

	if _, present := raw["contract"]; present {
		t.Fatalf("contract present for a Draft, want it absent")
	}
}

// TestDetailHandler_BirthPlanGlanceOnlyWhereOffered keeps the hub from
// mentioning a Birth Plan she cannot have: a postpartum-only Engagement
// with a stray Plan Instance still reads no birthPlan.
func TestDetailHandler_BirthPlanGlanceOnlyWhereOffered(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "portal-glance-postpartum"
	engagementID, _ := seedClientAtPracticeWithDueDateAndKind(t, db, identityUID, "Riverside Doulas", "", "postpartum")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO plan_instances (engagement_id, plan_type, fields, changed_at) VALUES ($1, 'birth_plan', '[]'::jsonb, now())`,
		engagementID); err != nil {
		t.Fatalf("seed birth plan: %v", err)
	}

	_, raw := getDetail(t, db, identityUID, engagementID)

	if _, present := raw["birthPlan"]; present {
		t.Fatalf("birthPlan present where none is offered, want it absent")
	}
}
