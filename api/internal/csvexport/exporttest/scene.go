package exporttest

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/csvexport"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
)

// Action is the Activity action the Scene's route records.
const Action = string(activity.ActionClientsExported)

// NewScene mounts one POST /test-export download behind the Owner and Admin
// gate, built by csvexport.Handler: the Practice's zone in its diff, a
// Test-Count header, and a two-row CSV, or, when refuse is set, its own 400.
// The Staff member at the screen is an employee Doula unless opts say otherwise.
func NewScene(t *testing.T, refuse bool, opts ...fixture.SceneOption) *fixture.Scene {
	t.Helper()
	spec := csvexport.Spec{Limit: "test_export", Action: activity.ActionClientsExported, File: "test"}
	handler := csvexport.Handler(spec, func(w http.ResponseWriter, _ *http.Request, _ staffauth.Call, env csvexport.Env) (csvexport.File, bool) {
		if refuse {
			apierr.WriteError(w, "That download is refused.", http.StatusBadRequest)
			return csvexport.File{}, false
		}
		return csvexport.File{
			Body:    []byte("a,b\n1,2\n"),
			Diff:    map[string]any{"zone": env.Zone.String()},
			Headers: map[string]string{"Test-Count": "1"},
		}, true
	})
	mount := fixture.Routes(func(_ *staffauth.GatedRouter, ir *idempotency.Router) {
		ir.ExemptGated("POST /api/practices/{practiceId}/test-export", "a test route that writes only its audit row", false, staffauth.OwnerAndAdmin, handler)
	})
	return fixture.NewScene(t, mount, opts...)
}

// Send is one request to the Scene's download as the Staff member at the
// screen.
func Send(s *fixture.Scene) *http.Response {
	return reqtest.Do(s.T, http.MethodPost, s.PracticeURL("/test-export"), s.SignedIn)
}
