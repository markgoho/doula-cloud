package fixture

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// RefusesEngagementsItCannotReach proves a route refuses an Engagement id
// it cannot reach: 400 for an id that is not a uuid and 404 for an
// Engagement of another Practice. urlOf gives the route for an Engagement
// id, and opts carry the session and any body the route reads.
func RefusesEngagementsItCannotReach(t *testing.T, db *testdb.DB, method string, urlOf func(engagementID string) string, opts ...reqtest.Option) {
	t.Helper()
	other := Engagement(t, db, Practice(t, db, PracticeName("Other Practice")))
	cases := []struct {
		name         string
		engagementID string
		want         int
	}{
		{"id is not a uuid", "not-a-uuid", http.StatusBadRequest},
		{"engagement of another Practice", other.ID, http.StatusNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reqtest.ExpectStatus(t, method, urlOf(c.engagementID), c.want, opts...)
		})
	}
}
