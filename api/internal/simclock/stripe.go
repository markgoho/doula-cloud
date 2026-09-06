package simclock

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/stripe/stripe-go/v86"
)

// StripeAPI is the real StripeClocks, backed by stripe-go's test-helper
// endpoints. It lives in this package rather than in api/internal/payments
// on purpose: payments is what the deployed BFF binary is built from, and
// nothing in it may name a test clock. Nothing in cmd/api imports
// simclock -- only cmd/simclock does -- so the test_clock parameter below
// is unreachable from a deployed configuration by construction, not by a
// runtime guard.
type StripeAPI struct {
	client *stripe.Client
}

// NewStripeAPI builds a StripeAPI from a Stripe secret API key. The key is
// always a Sandbox one: test clocks do not exist in live mode, and Stripe
// refuses the call outright with a live key.
func NewStripeAPI(apiKey string) *StripeAPI {
	return &StripeAPI{client: stripe.NewClient(apiKey)}
}

// CreateClock creates a test clock on accountID frozen at frozenTime and
// reports when Stripe will delete it. Stripe deletes a clock 30 real days
// after creation and reports the instant itself, so the expiry is read
// back rather than computed here.
func (s *StripeAPI) CreateClock(ctx context.Context, accountID string, frozenTime time.Time) (Clock, error) {
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	clock, err := s.client.V1TestHelpersTestClocks.Create(ctx, &stripe.TestHelpersTestClockCreateParams{
		Params:     stripe.Params{StripeAccount: stripe.String(accountID)},
		FrozenTime: new(frozenTime.Unix()),
	})
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	if err != nil {
		return Clock{}, fmt.Errorf("simclock: stripe create test clock: %w", err)
	}
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	return Clock{ID: clock.ID, DeletesAfter: time.Unix(clock.DeletesAfter, 0).UTC()}, nil
}

// CreateCustomer creates a Stripe Customer on accountID against clockID,
// carrying the Client's name and email and nothing else -- the same
// no-PHI-to-Stripe rule (#78) the product's own Customer creation
// follows. test_clock is settable only at creation, which is the whole
// reason a run makes the Customer rather than the product.
func (s *StripeAPI) CreateCustomer(ctx context.Context, accountID, clockID, email, name string) (string, error) {
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	cust, err := s.client.V1Customers.Create(ctx, &stripe.CustomerCreateParams{
		Params:    stripe.Params{StripeAccount: stripe.String(accountID)},
		Email:     stripe.String(email),
		Name:      stripe.String(name),
		TestClock: stripe.String(clockID),
	})
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	if err != nil {
		return "", fmt.Errorf("simclock: stripe create customer on clock: %w", err)
	}
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	return cust.ID, nil
}

// AdvanceClock starts advancing clockID to the absolute instant to, and
// returns as soon as Stripe accepts the request. Stripe's advance takes
// an absolute frozen time rather than a delta, which is what lets every
// clock -- including one created halfway through a run -- land on exactly
// the same instant.
func (s *StripeAPI) AdvanceClock(ctx context.Context, accountID, clockID string, to time.Time) error {
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	_, err := s.client.V1TestHelpersTestClocks.Advance(ctx, clockID, &stripe.TestHelpersTestClockAdvanceParams{
		Params:     stripe.Params{StripeAccount: stripe.String(accountID)},
		FrozenTime: new(to.Unix()),
	})
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	if err != nil {
		return fmt.Errorf("simclock: stripe advance test clock: %w", err)
	}
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	return nil
}

// ClockStatus reports clockID's current status.
func (s *StripeAPI) ClockStatus(ctx context.Context, accountID, clockID string) (ClockStatus, error) {
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	clock, err := s.client.V1TestHelpersTestClocks.Retrieve(ctx, clockID, &stripe.TestHelpersTestClockRetrieveParams{
		Params: stripe.Params{StripeAccount: stripe.String(accountID)},
	})
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	if err != nil {
		return "", fmt.Errorf("simclock: stripe retrieve test clock: %w", err)
	}
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	return ClockStatus(clock.Status), nil
}

// CustomerIsDeleted reports whether customerID no longer exists on
// accountID. Stripe answers this two ways -- a Customer it still has a
// record of comes back with deleted true, and one it has forgotten
// entirely comes back as a 404 -- and both mean the same thing here.
func (s *StripeAPI) CustomerIsDeleted(ctx context.Context, accountID, customerID string) (bool, error) {
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	cust, err := s.client.V1Customers.Retrieve(ctx, customerID, &stripe.CustomerRetrieveParams{
		Params: stripe.Params{StripeAccount: stripe.String(accountID)},
	})
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	if err != nil {
		var stripeErr *stripe.Error
		// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
		if errors.As(err, &stripeErr) && stripeErr.Code == stripe.ErrorCodeResourceMissing {
			return true, nil
		}
		// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
		return false, fmt.Errorf("simclock: stripe retrieve customer: %w", err)
	}
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	return cust.Deleted, nil
}

// CustomersOnClock lists every Customer Stripe reports as belonging to
// clockID. Stripe omits Customers that have a clock from an unfiltered
// list, so the filter is what makes this the exact set to compare the
// run's own record against.
func (s *StripeAPI) CustomersOnClock(ctx context.Context, accountID, clockID string) ([]string, error) {
	params := &stripe.CustomerListParams{
		TestClock: stripe.String(clockID),
	}
	params.StripeAccount = stripe.String(accountID)

	var ids []string
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	for cust, err := range s.client.V1Customers.List(ctx, params).All(ctx) {
		// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
		if err != nil {
			return nil, fmt.Errorf("simclock: stripe list customers on clock: %w", err)
		}
		// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
		ids = append(ids, cust.ID)
	}
	// coverage:ignore reason: requires a real Stripe API key and network access, not exercised by unit tests
	return ids, nil
}
