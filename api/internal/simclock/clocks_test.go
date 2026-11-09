package simclock_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"doula-cloud/api/internal/simclock"
)

// fakeStripe is an in-memory StripeClocks. It records what a run asked
// Stripe to do -- which is the only way to prove the properties that
// matter here: that every advance was issued before any was waited on,
// that a new clock is made only when no held one has room, and that a
// Customer which has lost its clock stops the run.
type fakeStripe struct {
	mu sync.Mutex

	clockSeq int
	custSeq  int
	// ClockLife is how long a created clock lasts. Stripe's own is 30
	// days; a test that needs an already-expired clock sets its own.
	ClockLife time.Duration

	Created   []time.Time
	Advances  []advance
	Customers map[string][]string // clock id -> customer ids Stripe reports

	// StatusScript, keyed by clock id, is the sequence of statuses
	// ClockStatus returns for that clock, one per call, the last repeating.
	// Unset means ready at once.
	StatusScript map[string][]simclock.ClockStatus
	statusCalls  map[string]int

	// Deleted, keyed by customer id, is what CustomerIsDeleted reports --
	// how a test says a Customer was erased rather than left off a clock.
	Deleted map[string]bool

	// Order records every call in sequence, so a test can prove the
	// advances were all issued before the first status read.
	Order []string

	CreateClockErr    error
	CreateCustomerErr error
	AdvanceErr        error
	StatusErr         error
	ListErr           error
	DeletedErr        error
}

// advance is one AdvanceClock call.
type advance struct {
	ClockID string
	To      time.Time
}

func newFakeStripe() *fakeStripe {
	return &fakeStripe{
		ClockLife:    30 * 24 * time.Hour,
		Customers:    map[string][]string{},
		StatusScript: map[string][]simclock.ClockStatus{},
		statusCalls:  map[string]int{},
		Deleted:      map[string]bool{},
	}
}

func (f *fakeStripe) CreateClock(_ context.Context, _ string, frozenTime time.Time) (simclock.Clock, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.CreateClockErr != nil {
		return simclock.Clock{}, f.CreateClockErr
	}
	f.clockSeq++
	f.Created = append(f.Created, frozenTime)
	id := fmt.Sprintf("clock_%d", f.clockSeq)
	f.Order = append(f.Order, "create:"+id)
	return simclock.Clock{ID: id, DeletesAfter: time.Now().Add(f.ClockLife)}, nil
}

func (f *fakeStripe) CreateCustomer(_ context.Context, _, clockID, _, _ string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.CreateCustomerErr != nil {
		return "", f.CreateCustomerErr
	}
	f.custSeq++
	id := fmt.Sprintf("cus_fake_%d", f.custSeq)
	f.Customers[clockID] = append(f.Customers[clockID], id)
	return id, nil
}

func (f *fakeStripe) AdvanceClock(_ context.Context, _, clockID string, to time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.AdvanceErr != nil {
		return f.AdvanceErr
	}
	f.Advances = append(f.Advances, advance{ClockID: clockID, To: to})
	f.Order = append(f.Order, "advance:"+clockID)
	return nil
}

func (f *fakeStripe) ClockStatus(_ context.Context, _, clockID string) (simclock.ClockStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.StatusErr != nil {
		return "", f.StatusErr
	}
	f.Order = append(f.Order, "status:"+clockID)
	script := f.StatusScript[clockID]
	if len(script) == 0 {
		return simclock.ClockReady, nil
	}
	i := f.statusCalls[clockID]
	f.statusCalls[clockID] = i + 1
	if i >= len(script) {
		i = len(script) - 1
	}
	return script[i], nil
}

// CustomerIsDeleted reports what a test put in Deleted -- absent means
// the Customer still exists, which is the drift case.
func (f *fakeStripe) CustomerIsDeleted(_ context.Context, _, customerID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.DeletedErr != nil {
		return false, f.DeletedErr
	}
	f.Order = append(f.Order, "deleted?:"+customerID)
	return f.Deleted[customerID], nil
}

func (f *fakeStripe) CustomersOnClock(_ context.Context, _, clockID string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.ListErr != nil {
		return nil, f.ListErr
	}
	f.Order = append(f.Order, "list:"+clockID)
	return f.Customers[clockID], nil
}

// migratedDB is a database with the shim installed *and* the product's
// own migrations applied, in that order -- the state a simulation run
// actually works against. The order is the whole point: the shim has to
// be in place before a migration binds a DEFAULT now().
func migratedDB(t *testing.T) *sql.DB {
	t.Helper()
	db, dsn := freshDB(t)
	if err := simclock.Install(t.Context(), db, superuserRole); err != nil {
		t.Fatalf("Install: %v", err)
	}
	migrate(t, dsn)
	return db
}

const testAccount = "acct_sim_test"

// seedPracticeAndClient seeds one connected Practice and one Client of
// hers, the two rows Allocate reads.
func seedPracticeAndClient(t *testing.T, db *sql.DB) (practiceID, clientID string) {
	t.Helper()
	ctx := t.Context()
	const name, email = "Rooted", "ada@example.com"
	if err := db.QueryRowContext(ctx,
		`INSERT INTO practices (name, stripe_connect_account_id, timezone) VALUES ($1, $2, 'America/New_York') RETURNING id`,
		name, testAccount,
	).Scan(&practiceID); err != nil {
		t.Fatalf("seed practice: %v", err)
	}
	if err := db.QueryRowContext(ctx,
		`INSERT INTO clients (practice_id, given_name, family_name, email) VALUES ($1, $2, 'Client', $3) RETURNING id`,
		practiceID, name, email,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	return practiceID, clientID
}

// seedClient seeds one more Client of an existing Practice. Stays local
// rather than testdb: this package's own fixtures pass a raw *sql.DB
// (the low-privilege app_runtime connection, not testdb.DB's
// Admin/App pair), and set family_name, which no other package's fixture
// needs.
func seedClient(t *testing.T, db *sql.DB, practiceID, name string) string {
	t.Helper()
	var clientID string
	if err := db.QueryRowContext(t.Context(),
		`INSERT INTO clients (practice_id, given_name, family_name, email) VALUES ($1, $2, 'Client', $3) RETURNING id`,
		practiceID, name, name+"@example.com",
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	return clientID
}

// env is what a test in this package starts from: a migrated database with
// the shim installed, one connected Practice with one Client, a fake Stripe,
// and a Runner over them that polls every millisecond. A test says only what
// it changes, with withDefaultPolling, withPollTimeout and withClockLife.
type env struct {
	T          *testing.T
	Ctx        context.Context
	DB         *sql.DB
	PracticeID string
	ClientID   string
	Stripe     *fakeStripe
	Runner     simclock.Runner
}

type envOption func(*env)

// withDefaultPolling leaves the Runner's poll interval unset.
func withDefaultPolling() envOption {
	return func(e *env) { e.Runner.PollInterval = 0 }
}

func withPollTimeout(d time.Duration) envOption {
	return func(e *env) { e.Runner.PollTimeout = d }
}

// withClockLife sets how long the clocks Stripe makes last.
func withClockLife(d time.Duration) envOption {
	return func(e *env) { e.Stripe.ClockLife = d }
}

func newEnv(t *testing.T, opts ...envOption) *env {
	t.Helper()
	db := migratedDB(t)
	practiceID, clientID := seedPracticeAndClient(t, db)
	stripe := newFakeStripe()
	e := &env{
		T: t, Ctx: t.Context(), DB: db, PracticeID: practiceID, ClientID: clientID, Stripe: stripe,
		Runner: simclock.Runner{DB: db, Stripe: stripe, PollInterval: time.Millisecond},
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// allocate gives the Client a Customer on a clock and returns its id.
func (e *env) allocate(clientID string) (customerID string) {
	e.T.Helper()
	customerID, _, err := e.Runner.Allocate(e.Ctx, clientID, testAccount)
	if err != nil {
		e.T.Fatalf("Allocate: %v", err)
	}
	return customerID
}

// allocateFourClients is the Practice's Client and three more, enough to
// fill one clock and start a second.
func (e *env) allocateFourClients() {
	e.T.Helper()
	e.allocate(e.ClientID)
	for _, name := range []string{"bea", "cleo", "dot"} {
		e.allocate(seedClient(e.T, e.DB, e.PracticeID, name))
	}
}

// offset reads the run's simulated delta and the database's simulated now.
func (e *env) offset() (delta time.Duration, simNow time.Time) {
	e.T.Helper()
	var deltaSeconds int64
	if err := e.DB.QueryRowContext(e.Ctx,
		`SELECT extract(epoch FROM delta)::bigint, pg_catalog.now() + delta FROM sim.offset_row`,
	).Scan(&deltaSeconds, &simNow); err != nil {
		e.T.Fatalf("read offset row: %v", err)
	}
	return time.Duration(deltaSeconds) * time.Second, simNow
}

func (e *env) requireOffsetUntouched(why string) {
	e.T.Helper()
	if delta, _ := e.offset(); delta != 0 {
		e.T.Fatalf("offset row delta = %s, want it untouched by %s", delta, why)
	}
}

func (e *env) advance(d time.Duration) {
	e.T.Helper()
	if err := e.Runner.Advance(e.Ctx, d); err != nil {
		e.T.Fatalf("Advance: %v", err)
	}
}

// statusReads counts the status reads the Stripe double saw.
func (e *env) statusReads() (reads int) {
	for _, step := range e.Stripe.Order {
		if strings.HasPrefix(step, "status:") {
			reads++
		}
	}
	return reads
}

// TestAllocate_MakesACustomerOnAClockAndRecordsTheMapping covers the
// first allocation on a fresh run: no clock exists, so one is created at
// the run's simulated now, the Customer is made on it, and the mapping
// the product reads is written.
func TestAllocate_MakesACustomerOnAClockAndRecordsTheMapping(t *testing.T) {
	e := newEnv(t)

	customerID, created, err := e.Runner.Allocate(e.Ctx, e.ClientID, testAccount)
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}
	if !created {
		t.Fatal("created = false, want true on a first allocation")
	}
	if len(e.Stripe.Created) != 1 {
		t.Fatalf("clocks created = %d, want 1", len(e.Stripe.Created))
	}

	var mappedPractice, mapped string
	var createdBy sql.NullString
	if err := e.DB.QueryRowContext(e.Ctx,
		`SELECT practice_id, stripe_customer_id, created_by_staff_id FROM client_stripe_customers
		  WHERE client_id = $1 AND stripe_account_id = $2`,
		e.ClientID, testAccount,
	).Scan(&mappedPractice, &mapped, &createdBy); err != nil {
		t.Fatalf("read mapping: %v", err)
	}
	if mapped != customerID {
		t.Fatalf("mapped customer = %q, want %q", mapped, customerID)
	}
	if mappedPractice != e.PracticeID {
		t.Fatalf("mapped practice = %q, want %q", mappedPractice, e.PracticeID)
	}
	// No Staff caused this Customer to exist, so the audit column says so
	// rather than borrowing someone's id.
	if createdBy.Valid {
		t.Fatalf("created_by_staff_id = %v, want null for a harness allocation", createdBy.String)
	}
}

// TestAllocate_IsANoOpTheSecondTime covers the property #779 depends on:
// it calls Allocate at stand-up and again after any mid-run Client
// creation, so calling it twice for the same Client and account must
// create nothing the second time.
func TestAllocate_IsANoOpTheSecondTime(t *testing.T) {
	e := newEnv(t)

	first := e.allocate(e.ClientID)
	second, created, err := e.Runner.Allocate(e.Ctx, e.ClientID, testAccount)
	if err != nil {
		t.Fatalf("second Allocate: %v", err)
	}
	if created {
		t.Fatal("created = true on the second call, want false")
	}
	if second != first {
		t.Fatalf("second Allocate returned %q, want the first's %q", second, first)
	}
	if len(e.Stripe.Created) != 1 {
		t.Fatalf("clocks created = %d, want 1", len(e.Stripe.Created))
	}
}

// TestAllocate_FillsAClockToCapacityBeforeMakingAnother covers the
// measured ceiling: a test clock holds three Customers and refuses a
// fourth, so the fourth Client gets a second clock.
func TestAllocate_FillsAClockToCapacityBeforeMakingAnother(t *testing.T) {
	e := newEnv(t)

	e.allocateFourClients()

	if len(e.Stripe.Created) != 2 {
		t.Fatalf("clocks created = %d, want 2 for 4 Customers at a capacity of %d",
			len(e.Stripe.Created), simclock.ClockCapacity)
	}
	if got := len(e.Stripe.Customers["clock_1"]); got != simclock.ClockCapacity {
		t.Fatalf("customers on the first clock = %d, want %d", got, simclock.ClockCapacity)
	}
	if got := len(e.Stripe.Customers["clock_2"]); got != 1 {
		t.Fatalf("customers on the second clock = %d, want 1", got)
	}
}

// TestAllocate_Refuses proves what a run is told when the Client or the
// account it names cannot be allocated against: an account no Practice is
// connected to rather than a mapping row with no Practice behind it; the
// same refusal the invoice path makes for a Client with no email, since an
// empty string is never sent to Stripe as one; and a client id that is not
// in this database, named rather than silently allocated against.
func TestAllocate_Refuses(t *testing.T) {
	for _, tc := range []struct {
		name    string
		account string
		client  func(e *env) string
		refused func(err error) bool
	}{
		{
			name:    "an account no Practice is connected to",
			account: "acct_nobody",
			client:  func(e *env) string { return e.ClientID },
			refused: func(err error) bool { return errors.Is(err, simclock.ErrNoPractice) },
		},
		{
			name:    "a Client with no email",
			account: testAccount,
			client: func(e *env) string {
				var clientID string
				if err := e.DB.QueryRowContext(e.Ctx,
					`INSERT INTO clients (practice_id, given_name) VALUES ($1, 'Noemail') RETURNING id`, e.PracticeID,
				).Scan(&clientID); err != nil {
					e.T.Fatalf("seed client: %v", err)
				}
				return clientID
			},
			refused: func(err error) bool { return err != nil && strings.Contains(err.Error(), "has no email") },
		},
		{
			name:    "an unknown Client",
			account: testAccount,
			client:  func(*env) string { return "00000000-0000-0000-0000-000000000000" },
			refused: func(err error) bool { return err != nil && strings.Contains(err.Error(), "no client") },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			_, _, err := e.Runner.Allocate(e.Ctx, tc.client(e), tc.account)
			if !tc.refused(err) {
				t.Fatalf("Allocate: got %v, want a refusal", err)
			}
		})
	}
}

// TestAllocate_ReportsAStripeFailure proves a Stripe failure on either
// call -- the clock or the Customer -- is reported rather than recorded
// as an allocation that did not happen.
func TestAllocate_ReportsAStripeFailure(t *testing.T) {
	failing := errors.New("stripe: fake failure")

	for name, set := range map[string]func(*fakeStripe){
		"creating the clock":    func(f *fakeStripe) { f.CreateClockErr = failing },
		"creating the customer": func(f *fakeStripe) { f.CreateCustomerErr = failing },
	} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			set(e.Stripe)
			if _, _, err := e.Runner.Allocate(e.Ctx, e.ClientID, testAccount); !errors.Is(err, failing) {
				t.Fatalf("Allocate: got %v, want the Stripe failure", err)
			}
		})
	}
}

// TestAdvance_MovesTheOffsetRowAndEveryClockTogether is the whole point
// of the ticket: one operation moves the database's simulated now and
// every Stripe test clock to the same instant. It also proves the clock
// created for a Client is frozen at the run's simulated now rather than
// the real one, which is what keeps a Practice that arrives mid-run in
// step with the rest.
func TestAdvance_MovesTheOffsetRowAndEveryClockTogether(t *testing.T) {
	e := newEnv(t)
	e.allocateFourClients()

	const week = 7 * 24 * time.Hour
	e.advance(week)

	delta, simNow := e.offset()
	if delta != week {
		t.Fatalf("offset row delta = %s, want %s", delta, week)
	}

	if len(e.Stripe.Advances) != 2 {
		t.Fatalf("advances = %d, want one per held clock", len(e.Stripe.Advances))
	}
	target := e.Stripe.Advances[0].To
	for _, a := range e.Stripe.Advances {
		if !a.To.Equal(target) {
			t.Fatalf("clock %s advanced to %v, want every clock on %v", a.ClockID, a.To, target)
		}
	}
	// The clocks and the database landed on the same instant: that is the
	// drift this ticket exists to prevent.
	if diff := simNow.Sub(target); diff > time.Second || diff < -time.Second {
		t.Fatalf("database simulated now is %s from the clocks' target, want them together", diff)
	}
}

// TestAdvance_IssuesEveryAdvanceBeforeWaitingOnAny proves the ordering
// that makes a jump cost one wait rather than one per clock: advances are
// asynchronous at Stripe, so all are issued and only then are they waited
// on.
func TestAdvance_IssuesEveryAdvanceBeforeWaitingOnAny(t *testing.T) {
	e := newEnv(t)
	e.allocateFourClients()
	// The first clock takes two polls to land, so a wait that came before
	// the second advance would be visible in the order below.
	e.Stripe.StatusScript["clock_1"] = []simclock.ClockStatus{simclock.ClockAdvancing, simclock.ClockReady}

	e.advance(time.Hour)

	var advances, firstStatus int
	for i, step := range e.Stripe.Order {
		switch {
		case strings.HasPrefix(step, "advance:"):
			advances++
			if firstStatus > 0 {
				t.Fatalf("advance issued at step %d, after a status read at step %d: %v", i, firstStatus, e.Stripe.Order)
			}
		case strings.HasPrefix(step, "status:") && firstStatus == 0:
			firstStatus = i
		}
	}
	if advances != 2 {
		t.Fatalf("advances issued = %d, want 2", advances)
	}
}

// TestAdvance_WaitsUntilEveryClockIsReady proves it does not return while
// a clock is still advancing -- reading a date off Stripe mid-advance is
// the drift case in its most direct form.
func TestAdvance_WaitsUntilEveryClockIsReady(t *testing.T) {
	e := newEnv(t)
	e.allocate(e.ClientID)
	e.Stripe.StatusScript["clock_1"] = []simclock.ClockStatus{
		simclock.ClockAdvancing, simclock.ClockAdvancing, simclock.ClockReady,
	}

	e.advance(time.Hour)

	if reads := e.statusReads(); reads != 3 {
		t.Fatalf("status reads = %d, want 3 -- it must poll until ready", reads)
	}
}

// TestAdvance_StopsTheRun proves each way a run is told to stop rather
// than carry on with its clocks and its database on different times: a
// clock Stripe reports as failed internally is not polled until the
// timeout; a clock that stays advancing is named rather than hanging the
// run; and a Customer is named when it is dated by real time while the
// rest of the run is dated by its own. A Customer with a null test_clock
// reads every date wrong in a way that looks like a product defect, and
// the product makes one when a Client is invoiced before she is allocated
// a clock.
func TestAdvance_StopsTheRun(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []envOption
		// arrange sets up the refusal and returns what the error names.
		arrange func(e *env) (named string)
	}{
		{
			name: "a clock that failed internally",
			arrange: func(e *env) string {
				e.allocate(e.ClientID)
				e.Stripe.StatusScript["clock_1"] = []simclock.ClockStatus{simclock.ClockInternalFailure}
				return string(simclock.ClockInternalFailure)
			},
		},
		{
			name: "a clock that never lands",
			opts: []envOption{withPollTimeout(10 * time.Millisecond)},
			arrange: func(e *env) string {
				e.allocate(e.ClientID)
				e.Stripe.StatusScript["clock_1"] = []simclock.ClockStatus{simclock.ClockAdvancing}
				return "still advancing"
			},
		},
		{
			name: "a Customer that has lost its clock",
			arrange: func(e *env) string {
				customerID := e.allocate(e.ClientID)
				// Stripe no longer reports the Customer as being on the clock.
				e.Stripe.Customers["clock_1"] = nil
				return customerID
			},
		},
		{
			name: "a Customer the product made with no clock",
			arrange: func(e *env) string {
				e.allocate(e.ClientID)
				// A second Client, whose mapping the product wrote when it raised her
				// first Invoice -- so there is no clock behind it.
				unallocated := seedClient(e.T, e.DB, e.PracticeID, "bea")
				if _, err := e.DB.ExecContext(e.Ctx,
					`INSERT INTO client_stripe_customers (practice_id, client_id, stripe_account_id, stripe_customer_id)
					 VALUES ($1, $2, $3, $4)`,
					e.PracticeID, unallocated, testAccount, "cus_made_by_the_product",
				); err != nil {
					e.T.Fatalf("seed product-made customer: %v", err)
				}
				return "cus_made_by_the_product"
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, tc.opts...)
			named := tc.arrange(e)

			err := e.Runner.Advance(e.Ctx, time.Hour)
			if err == nil || !strings.Contains(err.Error(), named) {
				t.Fatalf("Advance: got %v, want it to name %q", err, named)
			}
		})
	}
}

// TestAdvance_RefusesADeltaThatDoesNotMoveForward proves the guard on the
// one thing Stripe itself refuses: an advance target that is not after a
// clock's current frozen time.
func TestAdvance_RefusesADeltaThatDoesNotMoveForward(t *testing.T) {
	e := newEnv(t)
	for _, delta := range []time.Duration{0, -time.Hour} {
		if err := e.Runner.Advance(e.Ctx, delta); err == nil {
			t.Fatalf("Advance(%s) was accepted, want a refusal", delta)
		}
	}
}

// TestAdvance_RefusesToRunWhenAClockHasExpired proves the hard limit on a
// world's life: a clock is deleted 30 real days after it is made, and
// advancing the database past a clock that no longer exists would leave
// the two halves of the run on different times. It names the clocks, and
// it refuses *before* the offset row moves.
func TestAdvance_RefusesToRunWhenAClockHasExpired(t *testing.T) {
	// Already past its deletes_after.
	e := newEnv(t, withClockLife(-time.Minute))
	e.allocate(e.ClientID)

	err := e.Runner.Advance(e.Ctx, time.Hour)
	if err == nil || !strings.Contains(err.Error(), "clock_1") {
		t.Fatalf("Advance: got %v, want a refusal naming the expired clock", err)
	}

	e.requireOffsetUntouched("a refused advance")
	if len(e.Stripe.Advances) != 0 {
		t.Fatalf("advances issued = %d, want 0", len(e.Stripe.Advances))
	}
}

// TestAdvance_ReportsAStripeFailure proves a failure on any of the three
// Stripe calls a jump makes is reported rather than swallowed.
func TestAdvance_ReportsAStripeFailure(t *testing.T) {
	failing := errors.New("stripe: fake failure")

	for name, set := range map[string]func(*fakeStripe){
		"issuing the advance":   func(f *fakeStripe) { f.AdvanceErr = failing },
		"reading the status":    func(f *fakeStripe) { f.StatusErr = failing },
		"listing the customers": func(f *fakeStripe) { f.ListErr = failing },
		// A Customer missing from its clock's list is what makes the run
		// ask Stripe whether it was deleted, so this case has to make one
		// missing before it can fail that read.
		"checking whether a customer was deleted": func(f *fakeStripe) {
			f.Customers["clock_1"] = nil
			f.DeletedErr = failing
		},
	} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			e.allocate(e.ClientID)
			set(e.Stripe)
			if err := e.Runner.Advance(e.Ctx, time.Hour); !errors.Is(err, failing) {
				t.Fatalf("Advance: got %v, want the Stripe failure", err)
			}
		})
	}
}

// TestAdvance_HeldClocksSurviveAStopAndRestart proves a resumed run works
// against the clocks the first one made, rather than starting a fresh set
// and walking into an expired or full one. A second Runner over the same
// database is exactly what a restarted process is.
func TestAdvance_HeldClocksSurviveAStopAndRestart(t *testing.T) {
	e := newEnv(t)
	firstCustomer := e.allocate(e.ClientID)

	// A second Runner, over a Stripe double that has never seen a clock --
	// everything it knows about the run's clocks comes off the database.
	fake := newFakeStripe()
	fake.Customers["clock_1"] = []string{firstCustomer}
	// Past the ids the first run's double handed out, so the two doubles
	// do not both name their first Customer the same thing.
	fake.custSeq = 10
	resumed := simclock.Runner{DB: e.DB, Stripe: fake, PollInterval: time.Millisecond}
	second := seedClient(t, e.DB, e.PracticeID, "bea")
	if _, _, err := resumed.Allocate(e.Ctx, second, testAccount); err != nil {
		t.Fatalf("Allocate after restart: %v", err)
	}
	if len(fake.Created) != 0 {
		t.Fatalf("clocks created after restart = %d, want 0 -- the held clock has room", len(fake.Created))
	}

	if err := resumed.Advance(e.Ctx, time.Hour); err != nil {
		t.Fatalf("Advance after restart: %v", err)
	}
	if len(fake.Advances) != 1 || fake.Advances[0].ClockID != "clock_1" {
		t.Fatalf("advances = %+v, want the clock the first run made", fake.Advances)
	}
}

// TestAdvance_UsesTheDefaultWaitWhenNoneIsSet covers the fallback a
// caller with no opinion about polling gets: a clock that is ready at
// once never waits, so the defaults cost nothing.
func TestAdvance_UsesTheDefaultWaitWhenNoneIsSet(t *testing.T) {
	e := newEnv(t, withDefaultPolling())
	e.allocate(e.ClientID)

	e.advance(time.Hour)

	if len(e.Stripe.Advances) != 1 {
		t.Fatalf("advances = %d, want 1", len(e.Stripe.Advances))
	}
}

// TestNewStripeAPI_SatisfiesTheClockSurface proves the real adapter is
// the StripeClocks a Runner takes. Every method on it needs a Stripe
// account and network access, so this is as far as a unit test goes --
// the calls themselves are proved against the Sandbox.
func TestNewStripeAPI_SatisfiesTheClockSurface(t *testing.T) {
	var api simclock.StripeClocks = simclock.NewStripeAPI("sk_test_not_a_real_key")
	if _, ok := api.(*simclock.StripeAPI); !ok {
		t.Fatalf("NewStripeAPI returned %T, want *simclock.StripeAPI", api)
	}
}

// TestAdvance_LeavesNothingMovedWhenStripeRefuses proves the ordering
// that stops a failed jump becoming the very drift the jump exists to
// prevent: the offset row moves only once every clock has landed, so a
// Stripe refusal leaves both halves of the run where they were.
func TestAdvance_LeavesNothingMovedWhenStripeRefuses(t *testing.T) {
	e := newEnv(t)
	e.allocate(e.ClientID)
	e.Stripe.AdvanceErr = errors.New("stripe: fake failure")

	if err := e.Runner.Advance(e.Ctx, time.Hour); err == nil {
		t.Fatal("Advance: got nil, want the Stripe failure")
	}

	e.requireOffsetUntouched("a failed advance")
}

// TestAdvance_AnErasedCustomerIsNotDrift proves the distinction the drift
// guard has to make. Stripe omits a deleted Customer from a clock's list
// exactly as it omits one that never had a clock, and erasure deletes
// Customers -- so without this an erased Client would stop every jump
// after her.
func TestAdvance_AnErasedCustomerIsNotDrift(t *testing.T) {
	e := newEnv(t)
	customerID := e.allocate(e.ClientID)
	// Erased: gone from the clock's list, and gone from Stripe.
	e.Stripe.Customers["clock_1"] = nil
	e.Stripe.Deleted[customerID] = true

	e.advance(time.Hour)
}
