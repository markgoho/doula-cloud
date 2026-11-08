// Package fixture builds the records a test starts from: a Practice, the
// Staff who work at it, its Clients and their Engagements. A builder takes
// the test and the database, returns a typed record carrying the IDs the
// test goes on to use, and takes options for only the state the test is
// about, so that state is the thing visible at the call site (#2743):
//
//	p := fixture.Practice(t, db)
//	owner := fixture.Staff(t, db, p, fixture.Owner())
//	doula := fixture.Staff(t, db, p, fixture.Contractor())
//	e := fixture.Engagement(t, db, p, fixture.Status(fixture.StatusActive), fixture.WithDoula(doula))
//	resp := reqtest.Get(t, url, reqtest.Session(fixture.Session(t, db, owner)))
//
// Every builder writes through the superuser Admin connection, which
// bypasses Row-Level Security, with the same SQL the testdb seeders
// carry; none of them writes an activity row, because the seeders never
// did and a test that asserts on activity seeds its own with
// testdb.SeedActivity. Each test gets its own database from testdb.New,
// so nothing a builder writes outlives the test or reaches another.
package fixture

import (
	"fmt"
	"sync/atomic"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/testdb"
)

// The Staff roles a Practice grants.
const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleDoula  = "doula"
	employee   = "employee"
	contractor = "contractor"
)

// The kinds and statuses an Engagement takes.
const (
	KindBirth      = "birth"
	KindPostpartum = "postpartum"

	StatusIntake     = "intake"
	StatusActive     = "active"
	StatusPostpartum = "postpartum"
	StatusCompleted  = "completed"
)

var sequence atomic.Int64

func next() int64 { return sequence.Add(1) }

func must(t *testing.T, what string, err error) {
	t.Helper()
	if err != nil {
		// coverage:ignore reason: a test helper's own failure path, only reached when the fixture itself is broken
		t.Fatalf("fixture: %s: %v", what, err)
	}
}

// PracticeRecord is a seeded Practice.
type PracticeRecord struct {
	ID   string
	Name string
}

type practiceSpec struct {
	name    string
	credits int
}

// PracticeOption shapes the Practice a builder seeds.
type PracticeOption func(*practiceSpec)

// PracticeName names the Practice. The default is "Test Practice".
func PracticeName(name string) PracticeOption {
	return func(s *practiceSpec) { s.name = name }
}

// Credits gives the Practice a signup-bonus Credit balance of n.
func Credits(n int) PracticeOption {
	return func(s *practiceSpec) { s.credits = n }
}

// Practice seeds a Practice in the fixture zone (testdb.SeededPracticeZoneName).
func Practice(t *testing.T, db *testdb.DB, opts ...PracticeOption) PracticeRecord {
	t.Helper()
	spec := practiceSpec{name: "Test Practice"}
	for _, opt := range opts {
		opt(&spec)
	}
	p := PracticeRecord{ID: testdb.SeedPractice(t, db, spec.name), Name: spec.name}
	if spec.credits > 0 {
		_, err := db.Admin.ExecContext(t.Context(),
			`INSERT INTO credit_ledger (practice_id, origin, quantity) VALUES ($1, 'signup_bonus', $2)`,
			p.ID, spec.credits)
		must(t, "seed credits", err)
	}
	return p
}

// StaffRecord is a seeded Staff member and her Membership at one Practice.
// UID is the identity her session is minted for.
type StaffRecord struct {
	ID         string
	UID        string
	Name       string
	PracticeID string
	Roles      []string
	Employment string
}

type staffSpec struct {
	uid        string
	name       string
	roles      []string
	employment string
}

// StaffOption shapes the Staff member a builder seeds.
type StaffOption func(*staffSpec)

func addRole(role string) StaffOption {
	return func(s *staffSpec) { s.roles = append(s.roles, role) }
}

// Owner adds the owner role.
func Owner() StaffOption { return addRole(RoleOwner) }

// Admin adds the admin role.
func Admin() StaffOption { return addRole(RoleAdmin) }

// Doula adds the doula role, which is also what a Staff member holds when
// no role option is given.
func Doula() StaffOption { return addRole(RoleDoula) }

// Founder adds the roles Staff signup gives a Practice's founder: owner,
// admin and doula.
func Founder() StaffOption {
	return func(s *staffSpec) { s.roles = append(s.roles, RoleOwner, RoleAdmin, RoleDoula) }
}

// Roles sets exactly these roles, for a test that ranges over them. The
// default, and what Owner, Admin and Doula build up, is the Doula role alone.
func Roles(roles ...string) StaffOption {
	return func(s *staffSpec) { s.roles = roles }
}

// Contractor makes her a contractor. The default is an employee.
func Contractor() StaffOption {
	return func(s *staffSpec) { s.employment = contractor }
}

// StaffName sets her display name, split at the first space into first and
// last. The default is "Test Staff" and her UID.
func StaffName(name string) StaffOption {
	return func(s *staffSpec) { s.name = name }
}

// StaffUID sets her identity, for a test that signs in or asserts on that
// exact value. The default is unique within the process.
func StaffUID(uid string) StaffOption {
	return func(s *staffSpec) { s.uid = uid }
}

// Staff seeds a Staff member with a Membership at p. With no option she is
// an employee Doula.
func Staff(t *testing.T, db *testdb.DB, p PracticeRecord, opts ...StaffOption) StaffRecord {
	t.Helper()
	spec := staffSpec{employment: employee}
	for _, opt := range opts {
		opt(&spec)
	}
	if spec.uid == "" {
		spec.uid = fmt.Sprintf("staff-%d", next())
	}
	if spec.name == "" {
		spec.name = "Test Staff " + spec.uid
	}
	if len(spec.roles) == 0 {
		spec.roles = []string{RoleDoula}
	}
	id := testdb.SeedNamedStaffAtPractice(t, db, p.ID, spec.uid, spec.name, spec.roles, spec.employment)
	return StaffRecord{ID: id, UID: spec.uid, Name: spec.name, PracticeID: p.ID, Roles: spec.roles, Employment: spec.employment}
}

// ClientRecord is a seeded Client. PortalUID is empty unless the Client
// was seeded WithPortalLogin.
type ClientRecord struct {
	ID         string
	PracticeID string
	Name       string
	Email      string
	PortalUID  string
}

type clientSpec struct {
	name        string
	email       string
	noEmail     bool
	portalLogin bool
}

// ClientOption shapes the Client a builder seeds.
type ClientOption func(*clientSpec)

// ClientName sets her given name. The default is "Test Client".
func ClientName(name string) ClientOption {
	return func(s *clientSpec) { s.name = name }
}

// ClientEmail sets her address. The default is unique within the process.
func ClientEmail(email string) ClientOption {
	return func(s *clientSpec) { s.email = email }
}

// NoEmail seeds a Client with no address on file (NULL, not empty).
func NoEmail() ClientOption {
	return func(s *clientSpec) { s.noEmail = true }
}

// WithPortalLogin gives her an accepted Portal Account, so Session can
// sign her in.
func WithPortalLogin() ClientOption {
	return func(s *clientSpec) { s.portalLogin = true }
}

// Client seeds a Client at p.
func Client(t *testing.T, db *testdb.DB, p PracticeRecord, opts ...ClientOption) ClientRecord {
	t.Helper()
	n := next()
	spec := clientSpec{name: "Test Client", email: fmt.Sprintf("client-%d@example.com", n)}
	for _, opt := range opts {
		opt(&spec)
	}
	if spec.noEmail {
		spec.email = ""
	}
	c := ClientRecord{
		ID:         testdb.SeedNamedClient(t, db, p.ID, spec.name, spec.email),
		PracticeID: p.ID,
		Name:       spec.name,
		Email:      spec.email,
	}
	if spec.portalLogin {
		c.PortalUID = testdb.PortalUID(fmt.Sprintf("client-%d", n))
		testdb.SeedPortalUser(t, db, c.PortalUID, c.ID)
	}
	return c
}

// EngagementRecord is a seeded Engagement and the Client it is for.
type EngagementRecord struct {
	ID         string
	PracticeID string
	Client     ClientRecord
	Kind       string
	Status     string
}

type engagementSpec struct {
	client    *ClientRecord
	newClient []ClientOption
	kind      string
	status    string
	doulas    []StaffRecord
}

// EngagementOption shapes the Engagement a builder seeds.
type EngagementOption func(*engagementSpec)

// ForClient seeds the Engagement for an existing Client. The default is a
// new Client with the Client builder's defaults.
func ForClient(c ClientRecord) EngagementOption {
	return func(s *engagementSpec) { s.client = &c }
}

// NewClient seeds the Engagement's Client with these options, for a test
// that cares how its Client is named or reached but needs no record of her.
func NewClient(opts ...ClientOption) EngagementOption {
	return func(s *engagementSpec) { s.newClient = opts }
}

// Kind sets the Engagement's kind. The default is KindBirth.
func Kind(kind string) EngagementOption {
	return func(s *engagementSpec) { s.kind = kind }
}

// Status sets the Engagement's status. The default is StatusIntake. A
// completed Engagement also carries the ending reason and birth outcome
// the schema demands of one.
func Status(status string) EngagementOption {
	return func(s *engagementSpec) { s.status = status }
}

// WithDoula attaches d to the Engagement with an open granted Attachment.
// It can be given more than once.
func WithDoula(d StaffRecord) EngagementOption {
	return func(s *engagementSpec) { s.doulas = append(s.doulas, d) }
}

// Engagement seeds an Engagement at p.
func Engagement(t *testing.T, db *testdb.DB, p PracticeRecord, opts ...EngagementOption) EngagementRecord {
	t.Helper()
	spec := engagementSpec{kind: KindBirth, status: StatusIntake}
	for _, opt := range opts {
		opt(&spec)
	}
	var c ClientRecord
	if spec.client != nil {
		c = *spec.client
	} else {
		c = Client(t, db, p, spec.newClient...)
	}
	var id string
	err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, kind, status, ending_reason, birth_outcome)
		 VALUES ($1, $2, $3::engagement_kind, $4::engagement_status, $5, $6) RETURNING id`,
		c.ID, p.ID, spec.kind, spec.status,
		testdb.EndingReasonForStatus(spec.status), testdb.BirthOutcomeForStatus(spec.status),
	).Scan(&id)
	must(t, "seed engagement", err)
	for _, d := range spec.doulas {
		testdb.SeedGrantedAttachment(t, db, id, d.ID)
	}
	return EngagementRecord{ID: id, PracticeID: p.ID, Client: c, Kind: spec.kind, Status: spec.status}
}

// Signer is a record a session can be minted for: a StaffRecord, or a
// ClientRecord seeded WithPortalLogin.
type Signer interface {
	identity() string
}

func (s StaffRecord) identity() string { return s.UID }

func (c ClientRecord) identity() string { return c.PortalUID }

// Session mints a live session for who and returns the value its cookie
// carries, for reqtest.Session. The session carries a second factor, the
// way authntest.SeedSessionAt does, and is minted at clock.Now.
func Session(t *testing.T, db *testdb.DB, who Signer) string {
	t.Helper()
	uid := who.identity()
	if uid == "" {
		// coverage:ignore reason: the failure branch ends the calling test by design; exercising it would need a testing.T substitute this repo has no precedent for
		t.Fatalf("fixture: session for a Client with no portal login; seed her with fixture.WithPortalLogin()")
	}
	return authntest.SeedSessionAt(t, db.App, uid, clock.Now(t.Context()))
}
