package portal_test

import (
	"database/sql"
	"encoding/json/jsontext"
	"net/http"
	"slices"
	"testing"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/activityfeed"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/portal"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

const (
	// The birth outcomes these tests seed, as the birth_outcome enum spells them.
	outcomeLoss      = "loss"
	outcomeLiveBirth = "live_birth"

	doulaName = "Priya Raman"
)

// env is what a test in this package starts from: a Practice with one
// Engagement whose Client has a portal login, this package's whole surface
// mounted through portal.Mount, and one Staff member at the Practice, the
// Doula a Visit names and an Activity row's actor. AsClient signs a request
// in as the Client; the Engagement's routes are engagementURL(suffix).
type env struct {
	*fixture.Scene
}

func mountPortal(g *staffauth.GatedRouter, _ *idempotency.Router, app *sql.DB) {
	portal.Mount(g, app)
}

func newEnv(t *testing.T, opts ...fixture.SceneOption) *env {
	t.Helper()
	all := append([]fixture.SceneOption{
		fixture.WithClient(fixture.WithPortalLogin()),
		fixture.SignedInAs(fixture.StaffName(doulaName)),
	}, opts...)
	return &env{fixture.NewScene(t, mountPortal, all...)}
}

func (e *env) engagementURL(suffix string) string {
	return e.URL("/api/portal/engagements/" + e.Engagement.ID + suffix)
}

func (e *env) detail() portal.Detail {
	e.T.Helper()
	return reqtest.Expect[portal.Detail](e.T, http.MethodGet, e.engagementURL(""), http.StatusOK, e.AsClient)
}

func (e *env) detailFields() map[string]jsontext.Value {
	e.T.Helper()
	return reqtest.Expect[map[string]jsontext.Value](e.T, http.MethodGet, e.engagementURL(""), http.StatusOK, e.AsClient)
}

func (e *env) activityPage(query string) activityfeed.ListResponse {
	e.T.Helper()
	return reqtest.Expect[activityfeed.ListResponse](e.T, http.MethodGet, e.engagementURL("/activity"+query), http.StatusOK, e.AsClient)
}

func (e *env) visitsPage(query string) portal.VisitsResponse {
	e.T.Helper()
	return reqtest.Expect[portal.VisitsResponse](e.T, http.MethodGet, e.engagementURL("/visits"+query), http.StatusOK, e.AsClient)
}

// seedActivity writes one Activity row per action on the Engagement, by
// the Staff member at the screen.
func (e *env) seedActivity(actions ...activity.EngagementAction) {
	e.T.Helper()
	for _, action := range actions {
		testdb.SeedActivity(e.T, e.DB, e.Practice.ID, activity.SubjectEngagement, e.Engagement.ID, string(action), activity.StaffActor(e.Staff.ID))
	}
}

// activityActions is the Client's ledger as the sorted actions it names.
func (e *env) activityActions() []string {
	e.T.Helper()
	items := e.activityPage("").Items
	actions := make([]string, 0, len(items))
	for _, item := range items {
		actions = append(actions, item.Action)
	}
	slices.Sort(actions)
	return actions
}

// visit inserts a Visit by the Staff member at the screen; the zero time
// leaves it with no scheduled instant, the row #478 asks the read to omit.
func (e *env) visit(scheduledAt time.Time) string {
	e.T.Helper()
	return e.visitFor(e.Engagement.ID, e.Staff.ID, scheduledAt)
}

func (e *env) visitFor(engagementID, staffID string, scheduledAt time.Time) (visitID string) {
	e.T.Helper()
	var at any
	if !scheduledAt.IsZero() {
		at = scheduledAt
	}
	if err := e.DB.Admin.QueryRowContext(e.T.Context(),
		`INSERT INTO visits (engagement_id, staff_id, scheduled_at) VALUES ($1, $2, $3) RETURNING id`,
		engagementID, staffID, at,
	).Scan(&visitID); err != nil {
		e.T.Fatalf("seed visit: %v", err)
	}
	return visitID
}

// recordOutcome sets the birth outcome the Practice recorded and, when it
// has one, the date the pregnancy ended.
func (e *env) recordOutcome(outcome, endedOn string) {
	e.T.Helper()
	if outcome == "" {
		return
	}
	var ended *string
	if endedOn != "" {
		ended = &endedOn
	}
	e.Exec(`UPDATE engagements SET birth_outcome = $2::birth_outcome, pregnancy_ended_on = $3::date WHERE id = $1`,
		e.Engagement.ID, outcome, ended)
}
