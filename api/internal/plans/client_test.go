package plans_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/plans"
	"doula-cloud/api/internal/reqtest"
)

// portalRoutes are the three routes of the Client-portal Birth Plan view,
// mounted behind clientauth.Middleware: read it, download its PDF, and
// confirm she has read it.
var portalRoutes = []struct {
	name   string
	method string
	path   string
}{
	{"read", http.MethodGet, ""},
	{"pdf", http.MethodGet, "/pdf"},
	{"acknowledge", http.MethodPost, "/acknowledge"},
}

func TestClientGetBirthPlanHandler_Success(t *testing.T) {
	e := newEnv(t, withPortalLogin())
	e.seedBirthPlan()

	out := e.birthPlan()

	if out.EngagementID != e.Engagement.ID || out.PlanType != birthPlanType || out.Answers["location"] != hospital {
		t.Fatalf("unexpected response: %+v", out)
	}
}

// TestClientBirthPlanRoutes_RefuseABirthPlanThatIsNotOffered proves each
// portal route 404s when the Client is not offered a Birth Plan: none was
// started, only a Care Plan exists (so the Care Plan stays completely
// unreachable from the client-portal role, with no fallback to whatever
// plan_instances row it can find), the Engagement is postpartum-only (#311:
// independently of the portal's own nav/hub gating, even when reached
// straight by URL, and on the write side so a stale tab cannot stamp
// client_acknowledged_at), or the birth outcome is unknown (ADR-0015's
// presume-nothing value: no baby is presumed).
func TestClientBirthPlanRoutes_RefuseABirthPlanThatIsNotOffered(t *testing.T) {
	conditions := []struct {
		name string
		opts []envOption
		seed func(*env)
	}{
		{"none started", nil, func(*env) {}},
		{"only a Care Plan", nil, func(e *env) { e.seedInstance(carePlanType, f1Fields, `{"f1":"secret"}`) }},
		{"postpartum engagement", []envOption{withEngagement(fixture.Kind(fixture.KindPostpartum))}, (*env).seedBirthPlan},
		{"birth outcome unknown", nil, func(e *env) {
			e.seedBirthPlan()
			e.recordOutcome("unknown", nil)
		}},
	}
	for _, route := range portalRoutes {
		for _, c := range conditions {
			t.Run(route.name+"/"+c.name, func(t *testing.T) {
				e := newEnv(t, append(c.opts, withPortalLogin())...)
				c.seed(e)

				reqtest.ExpectStatus(t, route.method, e.portalURL(route.path), http.StatusNotFound, e.AsClient)
			})
		}
	}
}

// TestClientBirthPlanRoutes_OtherClientsEngagementRejected proves
// clientauth.Middleware itself rejects a foreign Engagement id with 403
// before any portal handler runs.
func TestClientBirthPlanRoutes_OtherClientsEngagementRejected(t *testing.T) {
	for _, route := range portalRoutes {
		t.Run(route.name, func(t *testing.T) {
			e := newEnv(t, withPortalLogin())
			other := e.AnotherEngagement()
			seedInstance(t, e.DB, other.ID, birthPlanType, locationNoOptions, `{}`)

			reqtest.ExpectStatus(t, route.method, e.portalURLOf(other.ID, route.path), http.StatusForbidden, e.AsClient)
		})
	}
}

// TestClientAcknowledgeBirthPlanHandler_Success is #301's central AC: a
// Client can confirm she has read her Birth Plan, the write lands on the
// row, and it is attributed to her in the audit trail (ADR-0022).
func TestClientAcknowledgeBirthPlanHandler_Success(t *testing.T) {
	e := newEnv(t, withPortalLogin())
	e.seedBirthPlan()

	out := reqtest.Expect[plans.InstanceResponse](t, http.MethodPost, e.portalURL("/acknowledge"), http.StatusOK, e.AsClient)
	if out.ClientAcknowledgedAt == nil {
		t.Fatalf("clientAcknowledgedAt = nil, want it set by the acknowledge write")
	}

	if got := e.birthPlan(); got.ClientAcknowledgedAt == nil {
		t.Fatalf("a subsequent GET clientAcknowledgedAt = nil, want the acknowledgment to persist")
	}

	var action, actorKind, actorClientID string
	if err := e.DB.Admin.QueryRowContext(t.Context(),
		`SELECT action, actor_kind::text, actor_client_id FROM activity
		 WHERE subject_kind = 'engagement' AND subject_id = $1 ORDER BY created_at DESC LIMIT 1`,
		e.Engagement.ID,
	).Scan(&action, &actorKind, &actorClientID); err != nil {
		t.Fatalf("query activity: %v", err)
	}
	if action != "birth_plan_acknowledged" || actorKind != "client" || actorClientID != e.Engagement.Client.ID {
		t.Fatalf("activity row = (action=%q, actorKind=%q, actorClientID=%q), want (birth_plan_acknowledged, client, %q)", action, actorKind, actorClientID, e.Engagement.Client.ID)
	}
}
