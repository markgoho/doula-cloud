package fixture

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// Mount is a package's Mount call, the one main.go makes on the real
// GatedRouter and idempotency.Router. app is the low-privilege connection
// the Scene's database serves, for a Mount that takes one.
type Mount func(g *staffauth.GatedRouter, ir *idempotency.Router, app *sql.DB)

// Routes is a Mount for a package whose Mount takes no database, such as
// engagement.Mount.
func Routes(mount func(*staffauth.GatedRouter, *idempotency.Router)) Mount {
	return func(g *staffauth.GatedRouter, ir *idempotency.Router, _ *sql.DB) { mount(g, ir) }
}

// Scene is what a handler test in any package starts from: a database, a
// Practice, one Staff member at the screen with a live session, and a
// server with the package's routes mounted behind staffauth.Middleware. A
// package embeds *Scene in its own env and adds the records and helpers
// only it needs.
type Scene struct {
	T        *testing.T
	DB       *testdb.DB
	Srv      *httptest.Server
	Practice PracticeRecord
	// Staff is the Staff member at the screen, an employee Doula unless
	// SignedInAs says otherwise, and the zero record under NobodySignedIn.
	Staff StaffRecord
	// SignedIn signs a request in as Staff.
	SignedIn reqtest.Option
	// Engagement is the Engagement the test is about, seeded only when the
	// Scene is built WithEngagement or WithClient.
	Engagement EngagementRecord
	// AsClient signs a request in as the Engagement's Client, set only when
	// she was seeded WithPortalLogin.
	AsClient reqtest.Option

	at time.Time
}

type sceneSpec struct {
	practice   []PracticeOption
	staff      []StaffOption
	nobody     bool
	engagement []EngagementOption
	client     []ClientOption
	withIt     bool
	attached   bool
	at         time.Time
	mounts     []Mount
}

// SceneOption shapes a Scene.
type SceneOption func(*sceneSpec)

// InPractice shapes the Practice.
func InPractice(opts ...PracticeOption) SceneOption {
	return func(s *sceneSpec) { s.practice = append(s.practice, opts...) }
}

// SignedInAs shapes the Staff member who is at the screen.
func SignedInAs(opts ...StaffOption) SceneOption {
	return func(s *sceneSpec) { s.staff = append(s.staff, opts...) }
}

// NobodySignedIn seeds no Staff member at all, for a test that builds the
// Practice's people itself.
func NobodySignedIn() SceneOption {
	return func(s *sceneSpec) { s.nobody = true }
}

// WithEngagement seeds an Engagement at the Practice and shapes it. With
// no option it is an intake birth for a new Client.
func WithEngagement(opts ...EngagementOption) SceneOption {
	return func(s *sceneSpec) {
		s.withIt = true
		s.engagement = append(s.engagement, opts...)
	}
}

// WithClient seeds an Engagement, as WithEngagement does, and shapes its
// new Client.
func WithClient(opts ...ClientOption) SceneOption {
	return func(s *sceneSpec) {
		s.withIt = true
		s.client = append(s.client, opts...)
	}
}

// Attached grants the Staff member at the screen an open Attachment on
// the Engagement, and seeds the Engagement if no option has.
func Attached() SceneOption {
	return func(s *sceneSpec) {
		s.withIt = true
		s.attached = true
	}
}

// At fixes the instant every request is served at. The zero time leaves
// the real clock in place.
func At(at time.Time) SceneOption {
	return func(s *sceneSpec) { s.at = at }
}

// AlsoMounting mounts another package's routes beside the first, for a
// test whose subject is how the two agree.
func AlsoMounting(mount Mount) SceneOption {
	return func(s *sceneSpec) { s.mounts = append(s.mounts, mount) }
}

// NewScene seeds the Practice and the Staff member at the screen, mounts
// mount on a server and closes the server with the test.
func NewScene(t *testing.T, mount Mount, opts ...SceneOption) *Scene {
	t.Helper()
	var spec sceneSpec
	for _, opt := range opts {
		opt(&spec)
	}
	db := testdb.New(t)
	s := &Scene{T: t, DB: db, Practice: Practice(t, db, spec.practice...), at: spec.at}
	if !spec.nobody {
		s.Staff = Staff(t, db, s.Practice, spec.staff...)
		s.SignedIn = s.As(s.Staff)
	}
	if spec.withIt {
		engagementOpts := spec.engagement
		if len(spec.client) > 0 {
			engagementOpts = append(engagementOpts, NewClient(spec.client...))
		}
		s.Engagement = Engagement(t, db, s.Practice, engagementOpts...)
		if spec.attached {
			testdb.SeedGrantedAttachment(t, db, s.Engagement.ID, s.Staff.ID)
		}
		if s.Engagement.Client.PortalUID != "" {
			s.AsClient = s.As(s.Engagement.Client)
		}
	}

	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	for _, m := range append([]Mount{mount}, spec.mounts...) {
		m(g, ir, db.App)
	}
	var handler http.Handler = mux
	if !spec.at.IsZero() {
		handler = clock.Middleware(func() time.Time { return spec.at })(mux)
	}
	s.Srv = httptest.NewServer(handler)
	t.Cleanup(s.Srv.Close)
	return s
}

// As signs a request in as who, a Staff member or a Client with a portal
// login. Under At the session is minted at that instant, so the fixed
// clock does not read it as expired or not yet begun.
func (s *Scene) As(who Signer) reqtest.Option {
	s.T.Helper()
	if s.at.IsZero() {
		return reqtest.Session(Session(s.T, s.DB, who))
	}
	return reqtest.Session(authntest.SeedSessionAt(s.T, s.DB.App, who.identity(), s.at))
}

// Colleague is another Staff member at the Practice.
func (s *Scene) Colleague(opts ...StaffOption) StaffRecord {
	s.T.Helper()
	return Staff(s.T, s.DB, s.Practice, opts...)
}

// AnotherEngagement is a further Engagement at the Practice.
func (s *Scene) AnotherEngagement(opts ...EngagementOption) EngagementRecord {
	s.T.Helper()
	return Engagement(s.T, s.DB, s.Practice, opts...)
}

// URL is the server's address plus path.
func (s *Scene) URL(path string) string {
	return s.Srv.URL + path
}

// PracticeURL is the Practice's route plus path.
func (s *Scene) PracticeURL(path string) string {
	return s.URL("/api/practices/" + s.Practice.ID + path)
}

// Exec runs one fixture statement on the superuser connection.
func (s *Scene) Exec(query string, args ...any) {
	s.T.Helper()
	_, err := s.DB.Admin.ExecContext(s.T.Context(), query, args...)
	must(s.T, "exec "+query, err)
}
