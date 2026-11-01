package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// assertReplay sends act's retry twice after the first answer was lost:
// once with the code already spent, once on a new step-up. Each answers
// what the first did, writes no row, and leaves the session's stamp as
// it found it. A new key with no code is still refused.
func assertReplay(t *testing.T, s *opsStack, cookie string, act func(key string) *httptest.ResponseRecorder, acts func() int) {
	t.Helper()
	first := act("replay-key")
	if first.Code != http.StatusOK {
		t.Fatalf("first = %d; body %s", first.Code, first.Body)
	}
	if s.stepUpAt(cookie) != nil {
		t.Fatal("the first act did not spend its code")
	}
	before := acts()
	s.now = s.now.Add(time.Minute)

	spent := act("replay-key")

	if spent.Code != http.StatusOK || spent.Body.String() != first.Body.String() {
		t.Fatalf("retry with no code = %d %s, want the first answer %s", spent.Code, spent.Body, first.Body)
	}
	if acts() != before {
		t.Errorf("acts after the retry = %d, want %d", acts(), before)
	}

	if w := s.stepUp(cookie, "fresh-totp-token"); w.Code != http.StatusNoContent {
		t.Fatalf("step up again = %d", w.Code)
	}
	fresh := act("replay-key")

	if fresh.Code != http.StatusOK || fresh.Body.String() != first.Body.String() {
		t.Fatalf("retry on a new code = %d %s, want the first answer %s", fresh.Code, fresh.Body, first.Body)
	}
	if s.stepUpAt(cookie) == nil {
		t.Error("the replay spent the new code")
	}
	if acts() != before {
		t.Errorf("acts after the second retry = %d, want %d", acts(), before)
	}

	s.now = s.now.Add(stepUpLimit)
	if w := act("another-key"); w.Code != http.StatusForbidden {
		t.Errorf("a new key with no code = %d, want 403", w.Code)
	}
}

func TestRefund_ARetryAfterALostAnswerNeedsNoNewCode(t *testing.T) {
	s := newRefundStack(t)
	cookie := s.steppedUpCookie()

	assertReplay(t, s.opsStack, cookie,
		func(key string) *httptest.ResponseRecorder { return s.refund(cookie, key, s.order(1)) },
		func() int { return s.count(`SELECT count(*) FROM operator_acts`) })

	if calls := s.stripe.RefundCalls(); len(calls) != 1 {
		t.Errorf("stripe calls = %d, want 1", len(calls))
	}
}

func TestPilotGateOpen_ARetryAfterALostAnswerNeedsNoNewCode(t *testing.T) {
	s := closedGateStack(t)
	cookie := s.steppedUpCookie()

	assertReplay(t, s, cookie,
		func(key string) *httptest.ResponseRecorder { return s.openPilotGate(cookie, key, map[string]any{}) },
		func() int { return s.count(`SELECT count(*) FROM operator_acts`) })
}

func TestLegalPublish_ARetryAfterALostAnswerNeedsNoNewCode(t *testing.T) {
	s := newLegalStack(t)
	s.seedOwners()
	cookie := s.steppedUpCookie()

	assertReplay(t, s.opsStack, cookie,
		func(key string) *httptest.ResponseRecorder {
			return s.publish(cookie, key, "privacy", "2026-12-15", map[string]any{})
		},
		func() int { return s.count(`SELECT count(*) FROM operator_acts`) })

	if s.queued() != 4 {
		t.Errorf("outbox rows = %d, want 4", s.queued())
	}
}

func TestSecondFactorClear_ARetryAfterALostAnswerNeedsNoNewCode(t *testing.T) {
	s := newSecondFactorStack(t)
	cookie := s.steppedUpCookie()

	assertReplay(t, s.opsStack, cookie,
		func(key string) *httptest.ResponseRecorder { return s.clear(cookie, key, noFields) },
		func() int { return s.count(`SELECT count(*) FROM operator_acts`) })
}
