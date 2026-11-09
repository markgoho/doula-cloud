package notificationpref_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/notificationpref"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// env is what a test in this package starts from: a Practice with one
// Engagement whose Client has a Portal Account and a live session, and this
// package's whole surface mounted through notificationpref.Mount, the same
// call main.go makes.
type env struct {
	*fixture.Scene
}

func newEnv(t *testing.T) *env {
	t.Helper()
	return &env{Scene: fixture.NewScene(t, notificationpref.Mount, fixture.WithClient(fixture.WithPortalLogin()))}
}

// preferenceURL is the route for one Engagement's push preference.
func (e *env) preferenceURL(engagementID string) string {
	return e.URL("/api/portal/engagements/" + engagementID + "/notification-preference")
}

// enabled reads whether push is on for the Engagement, as the Client the
// session belongs to.
func (e *env) enabled(engagementID string) bool {
	e.T.Helper()
	return reqtest.Expect[notificationpref.PreferenceResponse](e.T, http.MethodGet, e.preferenceURL(engagementID), http.StatusOK, e.AsClient).Enabled
}

// set turns push on or off for the Engagement, as the Client the session
// belongs to, and returns what the route says is now on.
func (e *env) set(engagementID string, enabled bool) bool {
	e.T.Helper()
	return reqtest.Expect[notificationpref.PreferenceResponse](e.T, http.MethodPut, e.preferenceURL(engagementID), http.StatusOK,
		e.AsClient, reqtest.JSON(notificationpref.SetRequest{Enabled: enabled})).Enabled
}

// readPreferenceRow reads notification_preferences' ground truth for
// identityUID/engagementID, via the superuser Admin connection (bypassing
// RLS -- used only to assert on ground truth, not to observe RLS in
// effect). found is false when no row exists yet.
func readPreferenceRow(t *testing.T, db *testdb.DB, identityUID, engagementID string) (muted bool, found bool) {
	t.Helper()
	err := db.Admin.QueryRowContext(t.Context(),
		`SELECT muted FROM notification_preferences WHERE identity_uid = $1 AND engagement_id = $2 AND channel = 'push'`,
		identityUID, engagementID,
	).Scan(&muted)
	if err != nil {
		return false, false
	}
	return muted, true
}

// latestActivityAction reads the most recent activity row's action for
// engagementID, via the superuser Admin connection.
func latestActivityAction(t *testing.T, db *testdb.DB, engagementID string) (action, actorKind string) {
	t.Helper()
	err := db.Admin.QueryRowContext(t.Context(),
		`SELECT action, actor_kind::text FROM activity
		 WHERE subject_kind = 'engagement' AND subject_id = $1
		 ORDER BY created_at DESC LIMIT 1`,
		engagementID,
	).Scan(&action, &actorKind)
	if err != nil {
		t.Fatalf("read latest activity for %q: %v", engagementID, err)
	}
	return action, actorKind
}
