package billing_test

import (
	"errors"
	"testing"
	"time"

	"doula-cloud/api/internal/billing"
)

func TestFakeStripeClient_AnswersThePlatformAccountAndItsError(t *testing.T) {
	f := billing.NewFakeStripeClient()
	f.PlatformAccount = "acct_fake"
	if got, err := f.PlatformAccountID(t.Context()); got != "acct_fake" || err != nil {
		t.Errorf("PlatformAccountID() = %q, %v", got, err)
	}
	f.PlatformAccountErr = errors.New("down")
	if _, err := f.PlatformAccountID(t.Context()); err == nil {
		t.Error("PlatformAccountID() = nil error, want the set one")
	}
}

func TestFakeStripeClient_CountsUndeliveredEventsUpToTheLimitAndRecordsTheWindow(t *testing.T) {
	f := billing.NewFakeStripeClient()
	f.Undelivered = 5
	since := time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)

	if got, err := f.UndeliveredEventCount(t.Context(), since, 3); got != 3 || err != nil {
		t.Errorf("UndeliveredEventCount() = %d, %v; want 3", got, err)
	}
	f.UndeliveredEventsErr = errors.New("down")
	if _, err := f.UndeliveredEventCount(t.Context(), since, 3); err == nil {
		t.Error("UndeliveredEventCount() = nil error, want the set one")
	}
	if len(f.UndeliveredSince) != 2 || !f.UndeliveredSince[0].Equal(since) {
		t.Errorf("UndeliveredSince = %v", f.UndeliveredSince)
	}
}
