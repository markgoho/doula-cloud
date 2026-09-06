package simclock

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"doula-cloud/api/internal/client"
)

// ClockCapacity is how many Stripe Customers one test clock will hold.
// Measured against the Sandbox on #762, not read: the fourth
// `customers create` against a clock was refused. So a run needs
// ceil(Customers on that connected account / ClockCapacity) clocks.
//
// There is no matching ceiling on the number of clocks: 80 were created
// on one connected account during #780's triage with no refusal, well
// above anything docs/simulation/calendar.md produces.
const ClockCapacity = 3

// ClockStatus is what Stripe reports a test clock is doing. It takes
// exactly three values, so a status that is neither ready nor advancing
// is terminal by elimination rather than by a list this code has to keep
// in step with Stripe's.
type ClockStatus string

// Stripe's three test-clock statuses. Only ClockReady means an advance
// has finished; ClockInternalFailure is terminal and no further advance
// will succeed on that clock.
const (
	ClockReady           ClockStatus = "ready"
	ClockAdvancing       ClockStatus = "advancing"
	ClockInternalFailure ClockStatus = "internal_failure"
)

// Clock is one Stripe test clock a run holds: its id, and the real
// instant Stripe deletes it (30 days after creation, measured on #762).
// The expiry is real time, never simulated time -- it is when the object
// stops existing at Stripe, which no offset row can move.
type Clock struct {
	ID           string
	DeletesAfter time.Time
}

// StripeClocks is the Stripe surface a simulation run needs to keep
// Stripe on the same clock the database is on. It is an interface so the
// allocate and advance logic is exercised without a Stripe account;
// StripeAPI is the one implementation that talks to Stripe.
//
// Every method takes the connected account it acts on: a test clock, and
// every Customer on it, belongs to one connected account, and the calls
// are made with the Stripe-Account header set to it.
type StripeClocks interface {
	// CreateClock creates a test clock on accountID frozen at frozenTime
	// -- the run's simulated now, not the real one, so a Practice that
	// arrives three simulated weeks into a run is not three weeks behind
	// every other clock forever.
	CreateClock(ctx context.Context, accountID string, frozenTime time.Time) (Clock, error)
	// CreateCustomer creates a Stripe Customer on accountID against
	// clockID, carrying name and email and nothing else -- the same
	// no-PHI-to-Stripe rule (#78) the product's own Customer creation
	// follows.
	CreateCustomer(ctx context.Context, accountID, clockID, email, name string) (customerID string, err error)
	// AdvanceClock starts advancing clockID to the absolute instant to.
	// It returns as soon as Stripe accepts the request: the advance is
	// asynchronous, and ClockStatus reports when it finished.
	AdvanceClock(ctx context.Context, accountID, clockID string, to time.Time) error
	// ClockStatus reports clockID's current status -- one of ClockReady,
	// ClockAdvancing or ClockInternalFailure.
	ClockStatus(ctx context.Context, accountID, clockID string) (ClockStatus, error)
	// CustomersOnClock lists the ids of every Customer Stripe currently
	// reports as belonging to clockID. A Customer a run allocated onto a
	// clock and that is missing here has lost its clock, which is the
	// drift Advance stops the run over.
	CustomersOnClock(ctx context.Context, accountID, clockID string) ([]string, error)
	// CustomerIsDeleted reports whether customerID no longer exists on
	// accountID. Stripe omits a deleted Customer from a clock's list
	// exactly as it omits one that never had a clock, and only the second
	// is drift -- erasure deletes Customers, so without this an erased
	// Client would stop every jump after her.
	CustomerIsDeleted(ctx context.Context, accountID, customerID string) (bool, error)
}

// clockTablesSQL is the run's own record of the clocks it holds. It lives
// in the sim schema, alongside the offset row, for the same reason the
// offset row does: it is sandbox-only state that must never reach a
// deployed database, and the sim schema is the one place that is true of
// by construction.
//
// It is separate from installSQL, and IF NOT EXISTS, because a run
// resumed against a kept volume already has the schema: Install skips
// installSQL entirely in that case and these must still be reached.
//
// pg_catalog.now() is spelled out on both defaults. An unqualified now()
// under the migrating role's search_path is sim.now(), and both of these
// columns are real-time facts -- when a clock was made, and when Stripe
// deletes it -- that no offset may move.
const clockTablesSQL = `
CREATE TABLE IF NOT EXISTS sim.test_clocks (
    id text PRIMARY KEY,
    stripe_account_id text NOT NULL,
    deletes_after timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT pg_catalog.now()
);

CREATE TABLE IF NOT EXISTS sim.test_clock_customers (
    stripe_customer_id text PRIMARY KEY,
    clock_id text NOT NULL REFERENCES sim.test_clocks (id),
    stripe_account_id text NOT NULL,
    client_id uuid NOT NULL,
    created_at timestamptz NOT NULL DEFAULT pg_catalog.now()
);

CREATE INDEX IF NOT EXISTS test_clock_customers_clock
    ON sim.test_clock_customers (clock_id);
`

// Runner holds the clocks one simulation run has, and is what moves them.
// Both of its operations are on it rather than free functions because
// they share the same two dependencies and the same waiting policy.
//
// PollInterval and PollTimeout govern the wait in Advance; both fall back
// to defaults when zero, so a caller that has no opinion passes neither.
type Runner struct {
	DB     *sql.DB
	Stripe StripeClocks

	PollInterval time.Duration
	PollTimeout  time.Duration
}

// defaultPollInterval and defaultPollTimeout: an advance takes roughly
// six seconds plus about 0.4 s per clock (#762), so a second between
// polls costs at most one wasted second, and two minutes is far past any
// advance that is going to finish at all.
const (
	defaultPollInterval = time.Second
	defaultPollTimeout  = 2 * time.Minute
)

// ErrNoPractice is returned by Allocate when no Practice is connected to
// the Stripe account it was given -- the run asked for a Customer on an
// account this database knows nothing about, which is a mistake in the
// caller rather than a state to recover from.
var ErrNoPractice = errors.New("simclock: no practice is connected to that stripe account")

// Allocate gives clientID a Stripe Customer on accountID, made against a
// test clock the run holds, and records the mapping the product reads
// (client_stripe_customers). Called twice for the same Client and
// account, the second call is a no-op: it returns the Customer the first
// made and reports created false.
//
// This is what keeps every test-only concern out of api/. The product
// resolves a Client's Customer from the mapping and creates one only when
// there is none; a run writes the mapping first, so the product finds a
// Customer and no code path in the BFF ever names a test clock.
//
// A clock holding fewer than ClockCapacity Customers and not yet expired
// is reused; when none has room, a new clock is created at the run's
// current simulated now and registered. #779 calls this at stand-up and
// again after any mid-run Client creation.
func (r Runner) Allocate(ctx context.Context, clientID, accountID string) (customerID string, created bool, err error) {
	if existing, found, err := r.mappedCustomer(ctx, clientID, accountID); err != nil || found {
		return existing, false, err
	}

	practiceID, err := r.practiceFor(ctx, accountID)
	if err != nil {
		return "", false, err
	}
	name, email, err := r.clientContact(ctx, clientID)
	if err != nil {
		return "", false, err
	}
	simNow, err := r.simNow(ctx)
	if err != nil {
		// coverage:ignore reason: requires a database without the shim installed, which every caller of Allocate has already ruled out
		return "", false, err
	}

	clockID, err := r.clockWithRoom(ctx, accountID, simNow)
	if err != nil {
		return "", false, err
	}

	customerID, err = r.Stripe.CreateCustomer(ctx, accountID, clockID, email, name)
	if err != nil {
		return "", false, fmt.Errorf("simclock: create customer on clock %s: %w", clockID, err)
	}

	if err := r.record(ctx, recorded{
		PracticeID: practiceID,
		ClientID:   clientID,
		AccountID:  accountID,
		ClockID:    clockID,
		CustomerID: customerID,
	}); err != nil {
		// coverage:ignore reason: DB write failure, not exercised by unit tests
		return "", false, err
	}
	return customerID, true, nil
}

// recorded is one allocation, as it is written to both places that have
// to know about it: the run's own clock bookkeeping, and the mapping the
// product reads.
type recorded struct {
	PracticeID string
	ClientID   string
	AccountID  string
	ClockID    string
	CustomerID string
}

// record writes both rows in one transaction: the run's own record of
// which clock this Customer sits on, and the product-visible mapping.
// Either both survive a crash or neither does -- a mapping row with no
// clock record behind it would leave a Customer the drift guard is blind
// to.
//
// created_by_staff_id is left null deliberately: no Staff caused this
// Customer to exist, a simulation run did, and a null is the honest
// answer to "who did this" rather than a borrowed Staff id.
func (r Runner) record(ctx context.Context, a recorded) error {
	tx, err := r.DB.BeginTx(ctx, nil)
	// coverage:ignore reason: requires a broken connection, not exercised by unit tests
	if err != nil {
		// coverage:ignore reason: requires a broken connection, not exercised by unit tests
		return fmt.Errorf("simclock: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO sim.test_clock_customers (stripe_customer_id, clock_id, stripe_account_id, client_id)
		 VALUES ($1, $2, $3, $4)`,
		a.CustomerID, a.ClockID, a.AccountID, a.ClientID,
	); err != nil {
		// coverage:ignore reason: DB write failure, not exercised by unit tests
		return fmt.Errorf("simclock: record customer on clock: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO client_stripe_customers (practice_id, client_id, stripe_account_id, stripe_customer_id)
		 VALUES ($1, $2, $3, $4)`,
		a.PracticeID, a.ClientID, a.AccountID, a.CustomerID,
	); err != nil {
		// coverage:ignore reason: DB write failure, not exercised by unit tests
		return fmt.Errorf("simclock: record customer mapping: %w", err)
	}
	// coverage:ignore reason: commit failure, not exercised by unit tests
	if err := tx.Commit(); err != nil {
		// coverage:ignore reason: commit failure, not exercised by unit tests
		return fmt.Errorf("simclock: commit allocation: %w", err)
	}
	return nil
}

// mappedCustomer reports the Customer already mapped to clientID on
// accountID, if there is one. This is what makes Allocate a no-op the
// second time.
func (r Runner) mappedCustomer(ctx context.Context, clientID, accountID string) (string, bool, error) {
	var id string
	err := r.DB.QueryRowContext(ctx,
		`SELECT stripe_customer_id FROM client_stripe_customers
		  WHERE client_id = $1 AND stripe_account_id = $2`,
		clientID, accountID,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", false, fmt.Errorf("simclock: read customer mapping: %w", err)
	}
	return id, true, nil
}

// practiceFor resolves the Practice connected to accountID -- the
// mapping row carries it, because that is what row-level security
// matches on.
func (r Runner) practiceFor(ctx context.Context, accountID string) (string, error) {
	var id string
	err := r.DB.QueryRowContext(ctx,
		`SELECT id FROM practices WHERE stripe_connect_account_id = $1`, accountID,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("%w: %s", ErrNoPractice, accountID)
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", fmt.Errorf("simclock: resolve practice: %w", err)
	}
	return id, nil
}

// clientContact reads the only two Client fields that ever reach Stripe:
// her legal name and her email (#78). Nothing else is selected, so
// nothing else can be sent.
func (r Runner) clientContact(ctx context.Context, clientID string) (name, email string, err error) {
	var givenName string
	var familyName, clientEmail sql.NullString
	err = r.DB.QueryRowContext(ctx,
		`SELECT given_name, family_name, email FROM clients WHERE id = $1`, clientID,
	).Scan(&givenName, &familyName, &clientEmail)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", fmt.Errorf("simclock: no client %s", clientID)
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", "", fmt.Errorf("simclock: read client contact: %w", err)
	}
	if !clientEmail.Valid || clientEmail.String == "" {
		return "", "", fmt.Errorf("simclock: client %s has no email", clientID)
	}
	return client.LegalName(givenName, familyName.String), clientEmail.String, nil
}

// simNow is the run's current simulated instant: real now, shifted by the
// offset row. Both halves of the run are dated by it -- a clock is
// created frozen here, and Advance moves every clock to the same value.
func (r Runner) simNow(ctx context.Context) (time.Time, error) {
	var now time.Time
	if err := r.DB.QueryRowContext(ctx,
		`SELECT pg_catalog.now() + delta FROM sim.offset_row`,
	).Scan(&now); err != nil {
		// coverage:ignore reason: requires a database without the shim installed
		return time.Time{}, fmt.Errorf("simclock: read simulated now: %w", err)
	}
	return now, nil
}

// clockWithRoom returns a held clock on accountID that has room for
// another Customer and has not expired, creating and registering a new
// one when none does. Oldest first, so a run fills clocks in the order it
// made them rather than scattering Customers across all of them.
func (r Runner) clockWithRoom(ctx context.Context, accountID string, simNow time.Time) (string, error) {
	var id string
	err := r.DB.QueryRowContext(ctx,
		`SELECT c.id FROM sim.test_clocks c
		  WHERE c.stripe_account_id = $1
		    AND c.deletes_after > pg_catalog.now()
		    AND (SELECT count(*) FROM sim.test_clock_customers cc WHERE cc.clock_id = c.id) < $2
		  ORDER BY c.created_at
		  LIMIT 1`,
		accountID, ClockCapacity,
	).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", fmt.Errorf("simclock: find clock with room: %w", err)
	}

	clock, err := r.Stripe.CreateClock(ctx, accountID, simNow)
	if err != nil {
		return "", fmt.Errorf("simclock: create test clock: %w", err)
	}
	if _, err := r.DB.ExecContext(ctx,
		`INSERT INTO sim.test_clocks (id, stripe_account_id, deletes_after) VALUES ($1, $2, $3)`,
		clock.ID, accountID, clock.DeletesAfter,
	); err != nil {
		// coverage:ignore reason: DB write failure, not exercised by unit tests
		return "", fmt.Errorf("simclock: register test clock: %w", err)
	}
	return clock.ID, nil
}

// heldClock is one row of the run's clock bookkeeping.
type heldClock struct {
	ID           string
	AccountID    string
	DeletesAfter time.Time
}

// Advance moves the whole run's clock forward by delta: the offset row
// the database reads through sim.now(), and every test clock the run
// holds, to the same simulated instant. It is one operation on purpose.
// Splitting the offset advance from the Stripe advance is exactly how the
// drift this exists to prevent gets built -- an invoice due date computed
// by Stripe against one clock, compared by the app against a differently
// offset now(), is a bug a run would report as a product defect.
//
// In order: it refuses a delta that does not move forward; it refuses to
// run at all when any held clock is past its recorded deletes_after, and
// names them; it issues an advance to every clock before waiting on any
// of them, because advances are asynchronous; it waits until every clock
// reports ready; it checks for drift; and only then does it move the
// offset row.
//
// The offset row moves last so that a Stripe failure -- a refused
// advance, a clock that never lands -- leaves both halves of the run
// where they were rather than the database ahead of Stripe. That is the
// same drift, arrived at by failing instead of by design, and it is the
// one ordering that cannot produce it.
func (r Runner) Advance(ctx context.Context, delta time.Duration) error {
	if delta <= 0 {
		return fmt.Errorf("simclock: advance delta must move forward, got %s", delta)
	}

	clocks, err := r.heldClocks(ctx)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return err
	}
	if err := refuseExpired(clocks, time.Now()); err != nil {
		return err
	}

	now, err := r.simNow(ctx)
	if err != nil {
		// coverage:ignore reason: requires a database without the shim installed
		return err
	}
	target := now.Add(delta)

	// Every advance is issued before any is waited on: each takes roughly
	// six seconds at Stripe, so issuing them in series and waiting in
	// series would cost the run six seconds per clock instead of six
	// seconds total.
	for _, c := range clocks {
		if err := r.Stripe.AdvanceClock(ctx, c.AccountID, c.ID, target); err != nil {
			return fmt.Errorf("simclock: advance clock %s: %w", c.ID, err)
		}
	}
	if err := r.waitForReady(ctx, clocks); err != nil {
		return err
	}
	if err := r.checkForDrift(ctx, clocks); err != nil {
		return err
	}
	return r.moveOffset(ctx, delta)
}

// heldClocks reads every clock the run holds. They survive a stop and
// restart because they are rows: a resumed run reads the same clocks
// rather than starting a fresh set and walking into an expired or full
// one.
func (r Runner) heldClocks(ctx context.Context) ([]heldClock, error) {
	rows, err := r.DB.QueryContext(ctx,
		`SELECT id, stripe_account_id, deletes_after FROM sim.test_clocks ORDER BY created_at`)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("simclock: list held clocks: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []heldClock
	for rows.Next() {
		var c heldClock
		if err := rows.Scan(&c.ID, &c.AccountID, &c.DeletesAfter); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("simclock: scan held clock: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("simclock: iterate held clocks: %w", err)
	}
	return out, nil
}

// refuseExpired stops the run before anything moves when a clock is past
// the instant Stripe deletes it. Advancing the offset row and then
// finding half the Stripe side gone is worse than not starting: the names
// are in the error so the operator knows which clocks the world lost, and
// a world whose clocks have expired has reached the end of its life
// (#763) rather than hit a transient failure.
func refuseExpired(clocks []heldClock, now time.Time) error {
	var expired []string
	for _, c := range clocks {
		if !c.DeletesAfter.After(now) {
			expired = append(expired, c.ID)
		}
	}
	if len(expired) == 0 {
		return nil
	}
	sort.Strings(expired)
	return fmt.Errorf("simclock: %d test clock(s) expired, refusing to advance: %s",
		len(expired), strings.Join(expired, ", "))
}

// moveOffset shifts the offset row by delta -- the database half of the
// jump, run once every clock has already landed on the same instant.
func (r Runner) moveOffset(ctx context.Context, delta time.Duration) error {
	if _, err := r.DB.ExecContext(ctx,
		`UPDATE sim.offset_row SET delta = delta + make_interval(secs => $1)`,
		delta.Seconds(),
	); err != nil {
		// coverage:ignore reason: requires a database without the shim installed
		return fmt.Errorf("simclock: move offset row: %w", err)
	}
	return nil
}

// waitForReady polls every clock until each reports ready. An advance is
// asynchronous, and reading a date off Stripe while a clock is still
// advancing is the drift case in its most direct form, so this returns
// only when every clock has landed.
func (r Runner) waitForReady(ctx context.Context, clocks []heldClock) error {
	interval := r.PollInterval
	if interval <= 0 {
		interval = defaultPollInterval
	}
	timeout := r.PollTimeout
	if timeout <= 0 {
		timeout = defaultPollTimeout
	}
	deadline := time.Now().Add(timeout)

	for _, c := range clocks {
		for {
			status, err := r.Stripe.ClockStatus(ctx, c.AccountID, c.ID)
			if err != nil {
				return fmt.Errorf("simclock: read status of clock %s: %w", c.ID, err)
			}
			if status == ClockReady {
				break
			}
			if status != ClockAdvancing {
				return fmt.Errorf("simclock: clock %s reported status %q", c.ID, status)
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("simclock: clock %s still advancing after %s", c.ID, timeout)
			}
			select {
			case <-ctx.Done():
				// coverage:ignore reason: requires a cancelled context mid-poll, not exercised by unit tests
				return fmt.Errorf("simclock: waiting on clock %s: %w", c.ID, ctx.Err())
			case <-time.After(interval):
			}
		}
	}
	return nil
}

// checkForDrift stops the run when a Customer it is responsible for is
// no longer on a test clock. A Customer with a null test_clock is dated
// by real time while everything around it is dated by the run's, so every
// date read off it afterwards is wrong in a way that reads as a product
// defect.
//
// Two things make a Customer suspect: Stripe no longer lists it on the
// clock the run put it on, or the product made it -- a Client invoiced
// before she was allocated one -- so it never had a clock at all. That is
// one list call per clock, about eight reads for run one, and no read per
// Customer while nothing is wrong.
//
// A suspect is only drift if it still exists. Stripe omits a *deleted*
// Customer from a clock's list exactly as it omits an unclocked one, and
// erasure deletes Customers, so an erased Client would otherwise make
// every later jump refuse. Each suspect therefore costs one more read to
// tell the two apart -- paid only when there is something to explain.
func (r Runner) checkForDrift(ctx context.Context, clocks []heldClock) error {
	suspects, err := r.suspectCustomers(ctx, clocks)
	if err != nil {
		return err
	}

	var adrift []string
	for _, s := range suspects {
		deleted, err := r.Stripe.CustomerIsDeleted(ctx, s.AccountID, s.CustomerID)
		if err != nil {
			return fmt.Errorf("simclock: check customer %s: %w", s.CustomerID, err)
		}
		if !deleted {
			adrift = append(adrift, s.CustomerID)
		}
	}
	if len(adrift) == 0 {
		return nil
	}
	sort.Strings(adrift)
	return fmt.Errorf("simclock: %d customer(s) are no longer on a test clock: %s",
		len(adrift), strings.Join(adrift, ", "))
}

// suspect is one Customer that may have drifted, and the connected
// account to ask Stripe about it on.
type suspect struct {
	CustomerID string
	AccountID  string
}

// suspectCustomers gathers every Customer that is not demonstrably on a
// clock: the ones Stripe no longer lists on the clock the run put them
// on, and the ones mapped on an account the run holds clocks for that the
// run never put on a clock at all.
func (r Runner) suspectCustomers(ctx context.Context, clocks []heldClock) ([]suspect, error) {
	var out []suspect
	for _, c := range clocks {
		onClock, err := r.Stripe.CustomersOnClock(ctx, c.AccountID, c.ID)
		if err != nil {
			return nil, fmt.Errorf("simclock: list customers on clock %s: %w", c.ID, err)
		}
		still := make(map[string]bool, len(onClock))
		for _, id := range onClock {
			still[id] = true
		}
		recordedIDs, err := r.customersOn(ctx, c.ID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return nil, err
		}
		for _, id := range recordedIDs {
			if !still[id] {
				out = append(out, suspect{CustomerID: id, AccountID: c.AccountID})
			}
		}
	}

	unclocked, err := r.unclockedCustomers(ctx, clocks)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, err
	}
	return append(out, unclocked...), nil
}

// unclockedCustomers names any Customer mapped on an account the run
// holds clocks for that the run never put on a clock. That is a Customer
// the *product* made -- a Client invoiced before she was allocated one --
// and it has a null test_clock by construction, so it drifts the moment
// the run jumps. It is caught here rather than left to whoever calls
// Advance, for the same reason the Stripe-side check is.
func (r Runner) unclockedCustomers(ctx context.Context, clocks []heldClock) ([]suspect, error) {
	accounts := make([]string, 0, len(clocks))
	seen := map[string]bool{}
	for _, c := range clocks {
		if !seen[c.AccountID] {
			seen[c.AccountID] = true
			accounts = append(accounts, c.AccountID)
		}
	}
	if len(accounts) == 0 {
		// coverage:ignore reason: Advance returns before this on a run holding no clocks
		return nil, nil
	}

	rows, err := r.DB.QueryContext(ctx,
		`SELECT m.stripe_customer_id, m.stripe_account_id FROM client_stripe_customers m
		  WHERE m.stripe_account_id = ANY($1)
		    AND NOT EXISTS (
		        SELECT 1 FROM sim.test_clock_customers cc
		         WHERE cc.stripe_customer_id = m.stripe_customer_id)`,
		accounts,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("simclock: list customers with no clock: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []suspect
	for rows.Next() {
		var s suspect
		if err := rows.Scan(&s.CustomerID, &s.AccountID); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("simclock: scan customer with no clock: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("simclock: iterate customers with no clock: %w", err)
	}
	return out, nil
}

// customersOn lists the Customers the run put on clockID.
func (r Runner) customersOn(ctx context.Context, clockID string) ([]string, error) {
	rows, err := r.DB.QueryContext(ctx,
		`SELECT stripe_customer_id FROM sim.test_clock_customers WHERE clock_id = $1`, clockID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("simclock: list customers on clock: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("simclock: scan customer on clock: %w", err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("simclock: iterate customers on clock: %w", err)
	}
	return out, nil
}
