package portal_test

import (
	"testing"

	"doula-cloud/api/internal/fixture"
)

// TestDetailHandler_GlancesAbsentWhenNothingExistsYet is #2007's empty
// side: a new Engagement has no Contract, no Birth Plan and no Message,
// and the hub is told nothing rather than an empty object it would have
// to tell apart from a real one.
func TestDetailHandler_GlancesAbsentWhenNothingExistsYet(t *testing.T) {
	e := newEnv(t, fixture.WithEngagement(fixture.DueDate("2027-06-15")))

	raw := e.detailFields()

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
	e := newEnv(t, fixture.WithEngagement(fixture.DueDate("2027-06-15")))
	e.Exec(`INSERT INTO contracts (engagement_id, status, prose, amount_cents, signed_at, created_at)
		 VALUES ($1, 'signed', 'Test prose', 15000, '2026-08-14T15:00:00Z', now() - interval '1 day')`,
		e.Engagement.ID)
	e.Exec(`INSERT INTO plan_instances (engagement_id, plan_type, fields, client_acknowledged_at, changed_at)
		 VALUES ($1, 'birth_plan', '[]'::jsonb, '2026-08-20T12:00:00Z', now())`,
		e.Engagement.ID)
	e.Exec(`INSERT INTO messages (engagement_id, sender_type, sender_id, body, created_at)
		 VALUES ($1, 'staff', $2, 'An older note.', now() - interval '2 days'),
		        ($1, 'staff', $2, 'Thursday works. See you at 2.', now() - interval '1 hour')`,
		e.Engagement.ID, e.Staff.ID)

	out := e.detail()

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
	if out.LastMessage.SenderName != doulaName {
		t.Fatalf("lastMessage.senderName = %q, want %q", out.LastMessage.SenderName, doulaName)
	}
}

// TestDetailHandler_GlancesHideWhatIsNotHers holds the two reads the hub
// must not widen: a Contract still a Draft is the Practice's, not hers, so
// it reads as no Contract at all; and a postpartum-only Engagement with a
// stray Plan Instance still reads no birthPlan, so the hub never mentions
// a Birth Plan she cannot have.
func TestDetailHandler_GlancesHideWhatIsNotHers(t *testing.T) {
	tests := []struct {
		name   string
		kind   string
		insert string
		key    string
	}{
		{
			"a Contract that is still a Draft", fixture.KindBirth,
			`INSERT INTO contracts (engagement_id, status, prose, amount_cents) VALUES ($1, 'draft', 'Test prose', 15000)`,
			"contract",
		},
		{
			"a Birth Plan on a postpartum-only Engagement", fixture.KindPostpartum,
			`INSERT INTO plan_instances (engagement_id, plan_type, fields, changed_at) VALUES ($1, 'birth_plan', '[]'::jsonb, now())`,
			"birthPlan",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t, fixture.WithEngagement(fixture.Kind(tt.kind)))
			e.Exec(tt.insert, e.Engagement.ID)

			if _, present := e.detailFields()[tt.key]; present {
				t.Fatalf("%q present for %s, want it absent", tt.key, tt.name)
			}
		})
	}
}
