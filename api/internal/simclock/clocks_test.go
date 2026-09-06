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

	"github.com/pressly/goose/v3"

	"doula-cloud/api/db/migrations"
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

	migrating, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open db for migrating: %v", err)
	}
	goose.SetBaseFS(migrations.FS)
	defer goose.SetBaseFS(nil)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("set dialect: %v", err)
	}
	if err := goose.Up(migrating, "."); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	if err := migrating.Close(); err != nil {
		t.Fatalf("close migrating connection: %v", err)
	}
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
		`INSERT INTO practices (name, stripe_connect_account_id) VALUES ($1, $2) RETURNING id`,
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

// seedClient seeds one more Client of an existing Practice.
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

// TestAllocate_MakesACustomerOnAClockAndRecordsTheMapping covers the
// first allocation on a fresh run: no clock exists, so one is created at
// the run's simulated now, the Customer is made on it, and the mapping
// the product reads is written.
func TestAllocate_MakesACustomerOnAClockAndRecordsTheMapping(t *testing.T) {
	ctx := t.Context()
	db := migratedDB(t)
	practiceID, clientID := seedPracticeAndClient(t, db)

	stripe := newFakeStripe()
	runner := simclock.Runner{DB: db, Stripe: stripe}

	customerID, created, err := runner.Allocate(ctx, clientID, testAccount)
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}
	if !created {
		t.Fatal("created = false, want true on a first allocation")
	}
	if len(stripe.Created) != 1 {
		t.Fatalf("clocks created = %d, want 1", len(stripe.Created))
	}

	var mappedPractice, mapped string
	var createdBy sql.NullString
	if err := db.QueryRowContext(ctx,
		`SELECT practice_id, stripe_customer_id, created_by_staff_id FROM client_stripe_customers
		  WHERE client_id = $1 AND stripe_account_id = $2`,
		clientID, testAccount,
	).Scan(&mappedPractice, &mapped, &createdBy); err != nil {
		t.Fatalf("read mapping: %v", err)
	}
	if mapped != customerID {
		t.Fatalf("mapped customer = %q, want %q", mapped, customerID)
	}
	if mappedPractice != practiceID {
		t.Fatalf("mapped practice = %q, want %q", mappedPractice, practiceID)
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
	ctx := t.Context()
	db := migratedDB(t)
	_, clientID := seedPracticeAndClient(t, db)

	stripe := newFakeStripe()
	runner := simclock.Runner{DB: db, Stripe: stripe}

	first, _, err := runner.Allocate(ctx, clientID, testAccount)
	if err != nil {
		t.Fatalf("first Allocate: %v", err)
	}
	second, created, err := runner.Allocate(ctx, clientID, testAccount)
	if err != nil {
		t.Fatalf("second Allocate: %v", err)
	}
	if created {
		t.Fatal("created = true on the second call, want false")
	}
	if second != first {
		t.Fatalf("second Allocate returned %q, want the first's %q", second, first)
	}
	if len(stripe.Created) != 1 {
		t.Fatalf("clocks created = %d, want 1", len(stripe.Created))
	}
}

// TestAllocate_FillsAClockToCapacityBeforeMakingAnother covers the
// measured ceiling: a test clock holds three Customers and refuses a
// fourth, so the fourth Client gets a second clock.
func TestAllocate_FillsAClockToCapacityBeforeMakingAnother(t *testing.T) {
	ctx := t.Context()
	db := migratedDB(t)
	practiceID, first := seedPracticeAndClient(t, db)

	stripe := newFakeStripe()
	runner := simclock.Runner{DB: db, Stripe: stripe}

	clients := []string{first}
	for _, name := range []string{"bea", "cleo", "dot"} {
		clients = append(clients, seedClient(t, db, practiceID, name))
	}
	for _, id := range clients {
		if _, _, err := runner.Allocate(ctx, id, testAccount); err != nil {
			t.Fatalf("Allocate: %v", err)
		}
	}

	if len(stripe.Created) != 2 {
		t.Fatalf("clocks created = %d, want 2 for %d Customers at a capacity of %d",
			len(stripe.Created), len(clients), simclock.ClockCapacity)
	}
	if got := len(stripe.Customers["clock_1"]); got != simclock.ClockCapacity {
		t.Fatalf("customers on the first clock = %d, want %d", got, simclock.ClockCapacity)
	}
	if got := len(stripe.Customers["clock_2"]); got != 1 {
		t.Fatalf("customers on the second clock = %d, want 1", got)
	}
}

// TestAllocate_RefusesAnAccountNoPracticeIsConnectedTo proves a run that
// names an account this database knows nothing about is told so, rather
// than writing a mapping row with no Practice behind it.
func TestAllocate_RefusesAnAccountNoPracticeIsConnectedTo(t *testing.T) {
	db := migratedDB(t)
	_, clientID := seedPracticeAndClient(t, db)

	runner := simclock.Runner{DB: db, Stripe: newFakeStripe()}
	_, _, err := runner.Allocate(t.Context(), clientID, "acct_nobody")
	if !errors.Is(err, simclock.ErrNoPractice) {
		t.Fatalf("Allocate against an unknown account: got %v, want ErrNoPractice", err)
	}
}

// TestAllocate_RefusesAClientWithNoEmail proves the same refusal the
// invoice path makes: an empty string is never sent to Stripe as an email.
func TestAllocate_RefusesAClientWithNoEmail(t *testing.T) {
	db := migratedDB(t)
	practiceID, _ := seedPracticeAndClient(t, db)

	var clientID string
	if err := db.QueryRowContext(t.Context(),
		`INSERT INTO clients (practice_id, given_name) VALUES ($1, 'Noemail') RETURNING id`, practiceID,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}

	runner := simclock.Runner{DB: db, Stripe: newFakeStripe()}
	_, _, err := runner.Allocate(t.Context(), clientID, testAccount)
	if err == nil || !strings.Contains(err.Error(), "has no email") {
		t.Fatalf("Allocate for a Client with no email: got %v, want a no-email refusal", err)
	}
}

// TestAllocate_ReportsAnUnknownClient proves a client id that is not in
// this database is named rather than silently allocated against.
func TestAllocate_ReportsAnUnknownClient(t *testing.T) {
	db := migratedDB(t)
	seedPracticeAndClient(t, db)

	runner := simclock.Runner{DB: db, Stripe: newFakeStripe()}
	_, _, err := runner.Allocate(t.Context(), "00000000-0000-0000-0000-000000000000", testAccount)
	if err == nil || !strings.Contains(err.Error(), "no client") {
		t.Fatalf("Allocate for an unknown Client: got %v, want a no-such-client refusal", err)
	}
}

// TestAllocate_ReportsAStripeFailure proves a Stripe failure on either
// call -- the clock or the Customer -- is reported rather than recorded
// as an allocation that did not happen.
func TestAllocate_ReportsAStripeFailure(t *testing.T) {
	failing := errors.New("stripe: fake failure")

	t.Run("creating the clock", func(t *testing.T) {
		ctx := t.Context()
		db := migratedDB(t)
		_, clientID := seedPracticeAndClient(t, db)
		stripe := newFakeStripe()
		stripe.CreateClockErr = failing
		runner := simclock.Runner{DB: db, Stripe: stripe}
		if _, _, err := runner.Allocate(ctx, clientID, testAccount); !errors.Is(err, failing) {
			t.Fatalf("Allocate: got %v, want the Stripe failure", err)
		}
	})

	t.Run("creating the customer", func(t *testing.T) {
		ctx := t.Context()
		db := migratedDB(t)
		_, clientID := seedPracticeAndClient(t, db)
		stripe := newFakeStripe()
		stripe.CreateCustomerErr = failing
		runner := simclock.Runner{DB: db, Stripe: stripe}
		if _, _, err := runner.Allocate(ctx, clientID, testAccount); !errors.Is(err, failing) {
			t.Fatalf("Allocate: got %v, want the Stripe failure", err)
		}
	})
}

// TestAdvance_MovesTheOffsetRowAndEveryClockTogether is the whole point
// of the ticket: one operation moves the database's simulated now and
// every Stripe test clock to the same instant. It also proves the clock
// created for a Client is frozen at the run's simulated now rather than
// the real one, which is what keeps a Practice that arrives mid-run in
// step with the rest.
func TestAdvance_MovesTheOffsetRowAndEveryClockTogether(t *testing.T) {
	ctx := t.Context()
	db := migratedDB(t)
	practiceID, first := seedPracticeAndClient(t, db)

	stripe := newFakeStripe()
	runner := simclock.Runner{DB: db, Stripe: stripe, PollInterval: time.Millisecond}

	for _, id := range []string{first, seedClient(t, db, practiceID, "bea"), seedClient(t, db, practiceID, "cleo"), seedClient(t, db, practiceID, "dot")} {
		if _, _, err := runner.Allocate(ctx, id, testAccount); err != nil {
			t.Fatalf("Allocate: %v", err)
		}
	}

	const week = 7 * 24 * time.Hour
	if err := runner.Advance(ctx, week); err != nil {
		t.Fatalf("Advance: %v", err)
	}

	var deltaSeconds int64
	var simNow time.Time
	if err := db.QueryRowContext(ctx,
		`SELECT extract(epoch FROM delta)::bigint, pg_catalog.now() + delta FROM sim.offset_row`,
	).Scan(&deltaSeconds, &simNow); err != nil {
		t.Fatalf("read offset row: %v", err)
	}
	if delta := time.Duration(deltaSeconds) * time.Second; delta != week {
		t.Fatalf("offset row delta = %s, want %s", delta, week)
	}

	if len(stripe.Advances) != 2 {
		t.Fatalf("advances = %d, want one per held clock", len(stripe.Advances))
	}
	target := stripe.Advances[0].To
	for _, a := range stripe.Advances {
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
	ctx := t.Context()
	db := migratedDB(t)
	practiceID, first := seedPracticeAndClient(t, db)

	stripe := newFakeStripe()
	runner := simclock.Runner{DB: db, Stripe: stripe, PollInterval: time.Millisecond}
	for _, id := range []string{first, seedClient(t, db, practiceID, "bea"), seedClient(t, db, practiceID, "cleo"), seedClient(t, db, practiceID, "dot")} {
		if _, _, err := runner.Allocate(ctx, id, testAccount); err != nil {
			t.Fatalf("Allocate: %v", err)
		}
	}
	// The first clock takes two polls to land, so a wait that came before
	// the second advance would be visible in the order below.
	stripe.StatusScript["clock_1"] = []simclock.ClockStatus{simclock.ClockAdvancing, simclock.ClockReady}

	if err := runner.Advance(ctx, time.Hour); err != nil {
		t.Fatalf("Advance: %v", err)
	}

	var advances, firstStatus int
	for i, step := range stripe.Order {
		switch {
		case strings.HasPrefix(step, "advance:"):
			advances++
			if firstStatus > 0 {
				t.Fatalf("advance issued at step %d, after a status read at step %d: %v", i, firstStatus, stripe.Order)
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
	ctx := t.Context()
	db := migratedDB(t)
	_, clientID := seedPracticeAndClient(t, db)

	stripe := newFakeStripe()
	runner := simclock.Runner{DB: db, Stripe: stripe, PollInterval: time.Millisecond}
	if _, _, err := runner.Allocate(ctx, clientID, testAccount); err != nil {
		t.Fatalf("Allocate: %v", err)
	}
	stripe.StatusScript["clock_1"] = []simclock.ClockStatus{
		simclock.ClockAdvancing, simclock.ClockAdvancing, simclock.ClockReady,
	}

	if err := runner.Advance(ctx, time.Hour); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	var reads int
	for _, step := range stripe.Order {
		if strings.HasPrefix(step, "status:") {
			reads++
		}
	}
	if reads != 3 {
		t.Fatalf("status reads = %d, want 3 -- it must poll until ready", reads)
	}
}

// TestAdvance_StopsOnAClockThatFailed proves a clock Stripe reports as
// having failed internally stops the run rather than being polled until
// the timeout.
func TestAdvance_StopsOnAClockThatFailed(t *testing.T) {
	ctx := t.Context()
	db := migratedDB(t)
	_, clientID := seedPracticeAndClient(t, db)

	stripe := newFakeStripe()
	runner := simclock.Runner{DB: db, Stripe: stripe, PollInterval: time.Millisecond}
	if _, _, err := runner.Allocate(ctx, clientID, testAccount); err != nil {
		t.Fatalf("Allocate: %v", err)
	}
	stripe.StatusScript["clock_1"] = []simclock.ClockStatus{simclock.ClockInternalFailure}

	err := runner.Advance(ctx, time.Hour)
	if err == nil || !strings.Contains(err.Error(), string(simclock.ClockInternalFailure)) {
		t.Fatalf("Advance: got %v, want the failed status reported", err)
	}
}

// TestAdvance_GivesUpOnAClockThatNeverLands proves the wait is bounded:
// a clock that stays advancing is named rather than hanging the run.
func TestAdvance_GivesUpOnAClockThatNeverLands(t *testing.T) {
	ctx := t.Context()
	db := migratedDB(t)
	_, clientID := seedPracticeAndClient(t, db)

	stripe := newFakeStripe()
	runner := simclock.Runner{DB: db, Stripe: stripe, PollInterval: time.Millisecond, PollTimeout: 10 * time.Millisecond}
	if _, _, err := runner.Allocate(ctx, clientID, testAccount); err != nil {
		t.Fatalf("Allocate: %v", err)
	}
	stripe.StatusScript["clock_1"] = []simclock.ClockStatus{simclock.ClockAdvancing}

	err := runner.Advance(ctx, time.Hour)
	if err == nil || !strings.Contains(err.Error(), "still advancing") {
		t.Fatalf("Advance: got %v, want a timeout naming the clock", err)
	}
}

// TestAdvance_RefusesADeltaThatDoesNotMoveForward proves the guard on the
// one thing Stripe itself refuses: an advance target that is not after a
// clock's current frozen time.
func TestAdvance_RefusesADeltaThatDoesNotMoveForward(t *testing.T) {
	db := migratedDB(t)
	runner := simclock.Runner{DB: db, Stripe: newFakeStripe()}
	for _, delta := range []time.Duration{0, -time.Hour} {
		if err := runner.Advance(t.Context(), delta); err == nil {
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
	ctx := t.Context()
	db := migratedDB(t)
	_, clientID := seedPracticeAndClient(t, db)

	stripe := newFakeStripe()
	stripe.ClockLife = -time.Minute // already past its deletes_after
	runner := simclock.Runner{DB: db, Stripe: stripe, PollInterval: time.Millisecond}
	if _, _, err := runner.Allocate(ctx, clientID, testAccount); err != nil {
		t.Fatalf("Allocate: %v", err)
	}

	err := runner.Advance(ctx, time.Hour)
	if err == nil || !strings.Contains(err.Error(), "clock_1") {
		t.Fatalf("Advance: got %v, want a refusal naming the expired clock", err)
	}

	var deltaSeconds int64
	if err := db.QueryRowContext(ctx,
		`SELECT extract(epoch FROM delta)::bigint FROM sim.offset_row`).Scan(&deltaSeconds); err != nil {
		t.Fatalf("read offset row: %v", err)
	}
	if deltaSeconds != 0 {
		t.Fatalf("offset row delta = %ds, want it untouched by a refused advance", deltaSeconds)
	}
	if len(stripe.Advances) != 0 {
		t.Fatalf("advances issued = %d, want 0", len(stripe.Advances))
	}
}

// TestAdvance_StopsTheRunWhenACustomerHasLostItsClock is the drift guard.
// A Customer with a null test_clock is dated by real time while
// everything around it is dated by the run's, so every date read off it
// afterwards is wrong in a way that reads as a product defect. The
// Customer is named.
func TestAdvance_StopsTheRunWhenACustomerHasLostItsClock(t *testing.T) {
	ctx := t.Context()
	db := migratedDB(t)
	_, clientID := seedPracticeAndClient(t, db)

	stripe := newFakeStripe()
	runner := simclock.Runner{DB: db, Stripe: stripe, PollInterval: time.Millisecond}
	customerID, _, err := runner.Allocate(ctx, clientID, testAccount)
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}
	// Stripe no longer reports the Customer as being on the clock.
	stripe.Customers["clock_1"] = nil

	err = runner.Advance(ctx, time.Hour)
	if err == nil || !strings.Contains(err.Error(), customerID) {
		t.Fatalf("Advance: got %v, want the adrift Customer named", err)
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
			ctx := t.Context()
			db := migratedDB(t)
			_, clientID := seedPracticeAndClient(t, db)
			stripe := newFakeStripe()
			runner := simclock.Runner{DB: db, Stripe: stripe, PollInterval: time.Millisecond}
			if _, _, err := runner.Allocate(ctx, clientID, testAccount); err != nil {
				t.Fatalf("Allocate: %v", err)
			}
			set(stripe)
			if err := runner.Advance(ctx, time.Hour); !errors.Is(err, failing) {
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
	ctx := t.Context()
	db := migratedDB(t)
	practiceID, first := seedPracticeAndClient(t, db)

	before := simclock.Runner{DB: db, Stripe: newFakeStripe(), PollInterval: time.Millisecond}
	firstCustomer, _, err := before.Allocate(ctx, first, testAccount)
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}

	// A second Runner, over a Stripe double that has never seen a clock --
	// everything it knows about the run's clocks comes off the database.
	fake := newFakeStripe()
	fake.Customers["clock_1"] = []string{firstCustomer}
	// Past the ids the first run's double handed out, so the two doubles
	// do not both name their first Customer the same thing.
	fake.custSeq = 10
	resumed := simclock.Runner{DB: db, Stripe: fake, PollInterval: time.Millisecond}
	second := seedClient(t, db, practiceID, "bea")
	if _, _, err := resumed.Allocate(ctx, second, testAccount); err != nil {
		t.Fatalf("Allocate after restart: %v", err)
	}
	if len(fake.Created) != 0 {
		t.Fatalf("clocks created after restart = %d, want 0 -- the held clock has room", len(fake.Created))
	}

	if err := resumed.Advance(ctx, time.Hour); err != nil {
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
	ctx := t.Context()
	db := migratedDB(t)
	_, clientID := seedPracticeAndClient(t, db)

	stripe := newFakeStripe()
	runner := simclock.Runner{DB: db, Stripe: stripe}
	if _, _, err := runner.Allocate(ctx, clientID, testAccount); err != nil {
		t.Fatalf("Allocate: %v", err)
	}

	if err := runner.Advance(ctx, time.Hour); err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if len(stripe.Advances) != 1 {
		t.Fatalf("advances = %d, want 1", len(stripe.Advances))
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
	ctx := t.Context()
	db := migratedDB(t)
	_, clientID := seedPracticeAndClient(t, db)

	stripe := newFakeStripe()
	runner := simclock.Runner{DB: db, Stripe: stripe, PollInterval: time.Millisecond}
	if _, _, err := runner.Allocate(ctx, clientID, testAccount); err != nil {
		t.Fatalf("Allocate: %v", err)
	}
	stripe.AdvanceErr = errors.New("stripe: fake failure")

	if err := runner.Advance(ctx, time.Hour); err == nil {
		t.Fatal("Advance: got nil, want the Stripe failure")
	}

	var deltaSeconds int64
	if err := db.QueryRowContext(ctx,
		`SELECT extract(epoch FROM delta)::bigint FROM sim.offset_row`).Scan(&deltaSeconds); err != nil {
		t.Fatalf("read offset row: %v", err)
	}
	if deltaSeconds != 0 {
		t.Fatalf("offset row delta = %ds, want it untouched by a failed advance", deltaSeconds)
	}
}

// TestAdvance_AnErasedCustomerIsNotDrift proves the distinction the drift
// guard has to make. Stripe omits a deleted Customer from a clock's list
// exactly as it omits one that never had a clock, and erasure deletes
// Customers -- so without this an erased Client would stop every jump
// after her.
func TestAdvance_AnErasedCustomerIsNotDrift(t *testing.T) {
	ctx := t.Context()
	db := migratedDB(t)
	_, clientID := seedPracticeAndClient(t, db)

	stripe := newFakeStripe()
	runner := simclock.Runner{DB: db, Stripe: stripe, PollInterval: time.Millisecond}
	customerID, _, err := runner.Allocate(ctx, clientID, testAccount)
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}
	// Erased: gone from the clock's list, and gone from Stripe.
	stripe.Customers["clock_1"] = nil
	stripe.Deleted[customerID] = true

	if err := runner.Advance(ctx, time.Hour); err != nil {
		t.Fatalf("Advance: %v", err)
	}
}

// TestAdvance_StopsTheRunWhenTheProductMadeACustomerWithNoClock covers
// the other half of the drift guard: a Customer the product made, because
// a Client was invoiced before she was allocated one, has no clock at all
// and is dated by real time from the first jump onwards.
func TestAdvance_StopsTheRunWhenTheProductMadeACustomerWithNoClock(t *testing.T) {
	ctx := t.Context()
	db := migratedDB(t)
	practiceID, clientID := seedPracticeAndClient(t, db)

	stripe := newFakeStripe()
	runner := simclock.Runner{DB: db, Stripe: stripe, PollInterval: time.Millisecond}
	if _, _, err := runner.Allocate(ctx, clientID, testAccount); err != nil {
		t.Fatalf("Allocate: %v", err)
	}

	// A second Client, whose mapping the product wrote when it raised her
	// first Invoice -- so there is no clock behind it.
	unallocated := seedClient(t, db, practiceID, "bea")
	if _, err := db.ExecContext(ctx,
		`INSERT INTO client_stripe_customers (practice_id, client_id, stripe_account_id, stripe_customer_id)
		 VALUES ($1, $2, $3, $4)`,
		practiceID, unallocated, testAccount, "cus_made_by_the_product",
	); err != nil {
		t.Fatalf("seed product-made customer: %v", err)
	}

	err := runner.Advance(ctx, time.Hour)
	if err == nil || !strings.Contains(err.Error(), "cus_made_by_the_product") {
		t.Fatalf("Advance: got %v, want the unclocked Customer named", err)
	}
}
