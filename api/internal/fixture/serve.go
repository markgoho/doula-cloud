package fixture

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// ServeStaff mounts mount on a server, closes it with the test, and seeds
// a live session for uid, returning the token its __session cookie
// carries. It is the Scene's server for a test that seeds its own people
// and keeps a literal uid.
func ServeStaff(t *testing.T, db *testdb.DB, uid string, mount Mount) (srv *httptest.Server, session string) {
	t.Helper()
	srv = httptest.NewServer(staffMux(db, mount))
	t.Cleanup(srv.Close)
	return srv, authntest.SeedSession(t, db.App, uid)
}

// ServeStaffAt is ServeStaff with the request clock fixed at now and the
// session minted at it.
func ServeStaffAt(t *testing.T, db *testdb.DB, uid string, mount Mount, now time.Time) (srv *httptest.Server, session string) {
	t.Helper()
	srv = httptest.NewServer(clock.Middleware(func() time.Time { return now })(staffMux(db, mount)))
	t.Cleanup(srv.Close)
	return srv, authntest.SeedSessionAt(t, db.App, uid, now)
}

func staffMux(db *testdb.DB, mount Mount) *http.ServeMux {
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	mount(g, idempotency.NewRouter(g, db.App), db.App)
	return mux
}
