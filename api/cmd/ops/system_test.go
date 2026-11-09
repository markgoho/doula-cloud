package main

import (
	"encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/stripemode"
)

const systemPath = "/api/system"

type systemStack struct {
	*opsStack
	stripe *billing.FakeStripeClient
}

func newSystemStack(t *testing.T) *systemStack {
	t.Helper()
	stripe := billing.NewFakeStripeClient()
	stripe.PlatformAccount = "acct_platform_1"
	return &systemStack{opsStack: stackWith(t, Deps{Stripe: stripe, StripeMode: stripemode.Test}), stripe: stripe}
}

func (s *systemStack) read(cookie string) *httptest.ResponseRecorder {
	s.t.Helper()
	return s.send(http.MethodGet, systemPath, cookie, nil, nil)
}

func (s *systemStack) view() systemView {
	s.t.Helper()
	w := s.read(s.signedInCookie())
	if w.Code != http.StatusOK {
		s.t.Fatalf("status = %d, want 200; body %s", w.Code, w.Body)
	}
	var v systemView
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		s.t.Fatalf("body %s: %v", w.Body, err)
	}
	return v
}

func TestSystem_RefusedWithoutASession(t *testing.T) {
	s := newSystemStack(t)

	if w := s.read(""); w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestSystem_NamesTheStripeModeAndThePlatformAccount(t *testing.T) {
	s := newSystemStack(t)

	v := s.view()

	if v.Stripe.Mode != "test" || v.Stripe.PlatformAccountID == nil || *v.Stripe.PlatformAccountID != "acct_platform_1" {
		t.Errorf("stripe = %+v, want test mode on acct_platform_1", v.Stripe)
	}
}

func TestSystem_CountsTheEventsStripeDidNotDeliverInTheLastSevenDays(t *testing.T) {
	s := newSystemStack(t)
	s.stripe.Undelivered = 3

	v := s.view()

	if v.Stripe.Undelivered == nil || v.Stripe.Undelivered.Count != 3 || v.Stripe.Undelivered.AtLeast {
		t.Errorf("undelivered = %+v, want 3 and not a lower bound", v.Stripe.Undelivered)
	}
	if want := opsStart.AddDate(0, 0, -7); len(s.stripe.UndeliveredSince) != 1 || !s.stripe.UndeliveredSince[0].Equal(want) {
		t.Errorf("window started %v, want %v", s.stripe.UndeliveredSince, want)
	}
}

func TestSystem_SaysAtLeastWhenStripeHasMoreUndeliveredEventsThanItCounts(t *testing.T) {
	s := newSystemStack(t)
	s.stripe.Undelivered = undeliveredCountLimit + 40

	v := s.view()

	if v.Stripe.Undelivered == nil || v.Stripe.Undelivered.Count != undeliveredCountLimit || !v.Stripe.Undelivered.AtLeast {
		t.Errorf("undelivered = %+v, want %d as a lower bound", v.Stripe.Undelivered, undeliveredCountLimit)
	}
}

func TestSystem_LeavesStripeFieldsEmptyWhenStripeCannotBeReadAndStillAnswers(t *testing.T) {
	s := newSystemStack(t)
	s.stripe.PlatformAccountErr = errors.New("stripe down")
	s.stripe.UndeliveredEventsErr = errors.New("stripe down")

	v := s.view()

	if v.Stripe.PlatformAccountID != nil || v.Stripe.Undelivered != nil || v.Stripe.Mode != "test" {
		t.Errorf("stripe = %+v, want the mode only", v.Stripe)
	}
	if len(v.Webhooks) != 4 {
		t.Errorf("webhooks = %d, want the database half intact", len(v.Webhooks))
	}
}
