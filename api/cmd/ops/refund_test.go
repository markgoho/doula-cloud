package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/testdb"
)

// refundStack is opsStack with a Practice holding one purchase lot of
// five $20.00 Credits and $6.86 of tax, and its Owner.
type refundStack struct {
	*opsStack
	stripe     *billing.FakeStripeClient
	practiceID string
	ownerID    string
	lotID      string
}

const (
	refundPracticeName = "Lakeside Birth Collective"
	refundOwnerName    = "Renata Alvarez"
	refundRequestedOn  = "2026-10-30"
)

func newRefundStack(t *testing.T) *refundStack {
	t.Helper()
	s := &refundStack{opsStack: &opsStack{t: t, db: testdb.New(t), now: opsStart}, stripe: billing.NewFakeStripeClient()}
	s.handler, _ = routes(Deps{
		DB:       s.db.Ops,
		Now:      func() time.Time { return s.now },
		Verifier: tokens,
		Origins:  []string{testOrigin},
		Stripe:   s.stripe,
	})
	s.practiceID = testdb.SeedPractice(t, s.db, refundPracticeName)
	s.ownerID = testdb.SeedNamedStaffAtPractice(t, s.db, s.practiceID, "refund-owner", refundOwnerName, []string{ownerRole}, "employee")
	if err := s.db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO credit_ledger (practice_id, origin, quantity, unit_price_cents, tax_cents, stripe_payment_intent_id, created_at)
		 VALUES ($1, 'purchase', 5, 2000, 686, 'pi_lakeside', $2) RETURNING id`,
		s.practiceID, opsStart.AddDate(0, -2, 0)).Scan(&s.lotID); err != nil {
		t.Fatalf("seed purchase: %v", err)
	}
	return s
}

func (s *opsStack) send(method, path, cookie string, header http.Header, body any) *httptest.ResponseRecorder {
	s.t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	}
	r := httptest.NewRequestWithContext(s.t.Context(), method, path, reader)
	if cookie != "" {
		r.Header = withCookie(cookie)
	}
	maps.Copy(r.Header, header)
	w := httptest.NewRecorder()
	s.handler.ServeHTTP(w, r)
	return w
}

func (s *refundStack) check(cookie, query string) *httptest.ResponseRecorder {
	s.t.Helper()
	return s.send(http.MethodGet, "/api/practices/"+s.practiceID+"/refund"+query, cookie, nil, nil)
}

func (s *refundStack) refund(cookie, key string, body any) *httptest.ResponseRecorder {
	s.t.Helper()
	header := http.Header{}
	if key != "" {
		header.Set(headerIdempotencyKey, key)
	}
	return s.send(http.MethodPost, "/api/practices/"+s.practiceID+"/refunds", cookie, header, body)
}

func (s *refundStack) order(credits int) refundRequest {
	return refundRequest{Credits: credits, RequestedBy: s.ownerID, RequestedOn: refundRequestedOn}
}

func TestRefundCheck_StatesThePracticeItsOwnersAndWhatComesBack(t *testing.T) {
	s := newRefundStack(t)

	w := s.check(s.signedInCookie(), "?credits=2")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", w.Code, w.Body)
	}
	want := `{"practiceName":"` + refundPracticeName + `","owners":[{"staffId":"` + s.ownerID + `","name":"` + refundOwnerName +
		`"}],"quote":{"credits":2,"amountCents":4274,"taxCents":274,"maxCredits":5}}`
	if got := strings.TrimSpace(w.Body.String()); got != want {
		t.Errorf("body = %s\nwant %s", got, want)
	}
}

func TestRefundCheck_WithNoCountQuotesTheWholePurchase(t *testing.T) {
	s := newRefundStack(t)

	w := s.check(s.signedInCookie(), "")

	if !strings.Contains(w.Body.String(), `"quote":{"credits":5,"amountCents":10686,"taxCents":686,"maxCredits":5}`) {
		t.Errorf("body = %s, want the whole lot quoted", w.Body)
	}
}

func TestRefundCheck_SaysWhenNothingCanBeRefunded(t *testing.T) {
	s := newRefundStack(t)
	other := testdb.SeedPractice(t, s.db, "Nothing Bought")

	w := s.send(http.MethodGet, "/api/practices/"+other+"/refund", s.signedInCookie(), nil, nil)

	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"owners":[],"quote":null`) {
		t.Errorf("status = %d, body = %s, want 200 with no Owners and no quote", w.Code, w.Body)
	}
}

func TestRefundCheck_RefusesMoreThanThePurchaseHolds(t *testing.T) {
	s := newRefundStack(t)

	w := s.check(s.signedInCookie(), "?credits=6")

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body %s", w.Code, w.Body)
	}
	if got := decodeError(t, w); got.Details["credits"] != "Number of Credits must be 5 or fewer" {
		t.Errorf("details = %v", got.Details)
	}
}

func TestRefundCheck_RefusesACountThatIsNotAWholeNumber(t *testing.T) {
	s := newRefundStack(t)

	for _, query := range []string{"?credits=two", "?credits=0"} {
		if w := s.check(s.signedInCookie(), query); w.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: status = %d, want 422", query, w.Code)
		}
	}
}

func TestRefundCheck_AnUnknownPracticeIsNotFound(t *testing.T) {
	s := newRefundStack(t)
	cookie := s.signedInCookie()

	for _, id := range []string{malformedPracticeID, unknownPracticeID} {
		if w := s.send(http.MethodGet, "/api/practices/"+id+"/refund", cookie, nil, nil); w.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", id, w.Code)
		}
	}
}

func TestRefundCheck_RefusesNoSession(t *testing.T) {
	s := newRefundStack(t)

	if w := s.check("", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestRefund_GivesTheMoneyBackAndRecordsWhoAndWhy(t *testing.T) {
	s := newRefundStack(t)
	cookie := s.steppedUpCookie()
	steppedUp := s.now
	s.now = s.now.Add(time.Minute)

	w := s.refund(cookie, "refund-key-1", s.order(2))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", w.Code, w.Body)
	}
	var got refundResult
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	want := refundResult{
		PracticeName: refundPracticeName, Credits: 2, AmountCents: 4274, TaxCents: 274,
		RequestedByName: refundOwnerName, RequestedOn: refundRequestedOn,
	}
	if got != want {
		t.Errorf("result = %+v\nwant %+v", got, want)
	}

	calls := s.stripe.RefundCalls()
	if len(calls) != 1 || calls[0].PaymentIntentID != "pi_lakeside" || calls[0].AmountCents != 4274 || calls[0].IdempotencyKey != "refund-key-1" {
		t.Fatalf("stripe calls = %+v, want one of 4274 cents on pi_lakeside under the request's key", calls)
	}

	var actID, subjectKind, subjectID string
	var details []byte
	var stepUpAt, createdAt time.Time
	if err := s.db.Admin.QueryRowContext(t.Context(),
		`SELECT a.id, a.subject_kind, a.subject_id, a.details, a.step_up_at, a.created_at
		   FROM operator_acts a JOIN operators o ON o.id = a.operator_id
		  WHERE o.identity_uid = $1 AND a.action = 'refund'`, opUID,
	).Scan(&actID, &subjectKind, &subjectID, &details, &stepUpAt, &createdAt); err != nil {
		t.Fatalf("read the refund act: %v", err)
	}
	if subjectKind != "practice" || subjectID != s.practiceID || !stepUpAt.Equal(steppedUp) || !createdAt.Equal(s.now) {
		t.Errorf("act = (%s/%s, step_up_at %s, created_at %s), want practice/%s stepped up at %s, taken at %s",
			subjectKind, subjectID, stepUpAt, createdAt, s.practiceID, steppedUp, s.now)
	}
	var gotDetails map[string]any
	_ = json.Unmarshal(details, &gotDetails)
	if gotDetails["requestedByStaffId"] != s.ownerID || gotDetails["requestedOn"] != refundRequestedOn ||
		gotDetails["credits"] != float64(2) || gotDetails["amountCents"] != float64(4274) {
		t.Errorf("act details = %s, want the requesting Owner, the request date, 2 Credits and 4274 cents", details)
	}

	if n := s.count(`SELECT count(*) FROM credit_ledger
		WHERE origin = 'refund' AND practice_id = $1 AND drawn_lot_id = $2 AND quantity = -2 AND operator_act_id = $3 AND created_at = $4`,
		s.practiceID, s.lotID, actID, s.now); n != 1 {
		t.Errorf("refund ledger rows keyed to the act = %d, want 1", n)
	}

	var diff []byte
	if err := s.db.Admin.QueryRowContext(t.Context(),
		`SELECT diff FROM activity
		  WHERE practice_id = $1 AND subject_kind = 'practice' AND subject_id = $1 AND action = 'credits_refunded'
		    AND actor_kind = 'operator' AND actor_operator_act_id = $2 AND created_at = $3`,
		s.practiceID, actID, s.now).Scan(&diff); err != nil {
		t.Fatalf("read the Activity row: %v", err)
	}
	if string(diff) != `{"credits": 2, "taxCents": 274, "amountCents": 4274}` {
		t.Errorf("activity diff = %s", diff)
	}

	if at := s.stepUpAt(cookie); at != nil {
		t.Errorf("step_up_at = %v after the act, want it spent", at)
	}
}

func TestRefund_ARetryUnderTheSameKeyIsTheSameRefund(t *testing.T) {
	s := newRefundStack(t)
	cookie := s.steppedUpCookie()
	first := s.refund(cookie, "refund-key-retry", s.order(1))
	if first.Code != http.StatusOK {
		t.Fatalf("first = %d; body %s", first.Code, first.Body)
	}
	if w := s.stepUp(cookie, "fresh-totp-token"); w.Code != http.StatusNoContent {
		t.Fatalf("step up again = %d", w.Code)
	}

	second := s.refund(cookie, "refund-key-retry", s.order(1))

	if second.Code != http.StatusOK || second.Body.String() != first.Body.String() {
		t.Fatalf("retry = %d %s, want the first answer %s", second.Code, second.Body, first.Body)
	}
	if calls := s.stripe.RefundCalls(); len(calls) != 1 {
		t.Errorf("stripe calls = %d, want 1", len(calls))
	}
	if n := s.count(`SELECT count(*) FROM operator_acts WHERE action = 'refund'`); n != 1 {
		t.Errorf("refund acts = %d, want 1", n)
	}
	if n := s.count(`SELECT count(*) FROM activity WHERE action = 'credits_refunded'`); n != 1 {
		t.Errorf("Activity rows = %d, want 1", n)
	}
}

func TestRefund_RefusesNoSession(t *testing.T) {
	s := newRefundStack(t)

	w := s.refund("", "refund-key", s.order(1))

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
	if len(s.stripe.RefundCalls()) != 0 {
		t.Error("Stripe was called without a session")
	}
}

func TestRefund_RefusesASessionThatHasNotSteppedUp(t *testing.T) {
	s := newRefundStack(t)

	w := s.refund(s.signedInCookie(), "refund-key", s.order(1))

	if w.Code != http.StatusForbidden || decodeError(t, w).Code != apierr.CodeSecondFactorRequired {
		t.Fatalf("status = %d, body %s, want 403 SECOND_FACTOR_REQUIRED", w.Code, w.Body)
	}
}

func TestRefund_RefusesAStaleStepUp(t *testing.T) {
	s := newRefundStack(t)
	cookie := s.steppedUpCookie()
	s.now = s.now.Add(stepUpLimit)

	w := s.refund(cookie, "refund-key", s.order(1))

	if w.Code != http.StatusForbidden || decodeError(t, w).Code != apierr.CodeSecondFactorRequired {
		t.Fatalf("status = %d, body %s, want 403 SECOND_FACTOR_REQUIRED", w.Code, w.Body)
	}
	if len(s.stripe.RefundCalls()) != 0 || s.count(`SELECT count(*) FROM operator_acts WHERE action = 'refund'`) != 0 {
		t.Error("a stale step-up moved money or recorded an act")
	}
}

func TestRefund_OneStepUpAdmitsOneRefund(t *testing.T) {
	s := newRefundStack(t)
	cookie := s.steppedUpCookie()
	if w := s.refund(cookie, "refund-key-a", s.order(1)); w.Code != http.StatusOK {
		t.Fatalf("first = %d; body %s", w.Code, w.Body)
	}

	w := s.refund(cookie, "refund-key-b", s.order(1))

	if w.Code != http.StatusForbidden {
		t.Fatalf("second refund on one step-up = %d, want 403", w.Code)
	}
}

func TestRefund_RefusesARequestWithNoKey(t *testing.T) {
	s := newRefundStack(t)

	w := s.refund(s.steppedUpCookie(), "", s.order(1))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestRefund_TakesNoActorFromTheRequest(t *testing.T) {
	s := newRefundStack(t)
	body := map[string]any{"credits": 1, "requestedBy": s.ownerID, "requestedOn": refundRequestedOn, "operator": "Someone Else"} //nolint:goconst // the request DTO's own field names

	w := s.refund(s.steppedUpCookie(), "refund-key", body)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for a body naming an actor; body %s", w.Code, w.Body)
	}
}

func TestRefund_RefusesABodyOfTheWrongShape(t *testing.T) {
	s := newRefundStack(t)
	cookie := s.steppedUpCookie()

	for _, body := range []any{map[string]any{"credits": "two"}, "not an object"} {
		if w := s.refund(cookie, "refund-key", body); w.Code != http.StatusBadRequest {
			t.Errorf("%v: status = %d, want 400", body, w.Code)
		}
	}
}

func TestRefund_SaysWhatToCorrectInEachAnswer(t *testing.T) {
	s := newRefundStack(t)
	cookie := s.steppedUpCookie()
	other := testdb.SeedStaffAtPractice(t, s.db, s.practiceID, "refund-doula", []string{doulaRole}, "employee")

	cases := []struct {
		name  string
		body  refundRequest
		field string
		want  string
	}{
		{"no Credits", refundRequest{RequestedBy: s.ownerID, RequestedOn: refundRequestedOn}, "credits", "Enter the number of Credits to refund, 1 or more"},
		{"no Owner", refundRequest{Credits: 1, RequestedOn: refundRequestedOn}, "requestedBy", "Choose the Owner who asked for the refund"},
		{"not an Owner", refundRequest{Credits: 1, RequestedBy: other, RequestedOn: refundRequestedOn}, "requestedBy", "Choose the Owner who asked for the refund"},
		{"no date", refundRequest{Credits: 1, RequestedBy: s.ownerID}, "requestedOn", "Enter the date the request arrived"},
		{"not a date", refundRequest{Credits: 1, RequestedBy: s.ownerID, RequestedOn: "30/10/2026"}, "requestedOn", "Enter the date the request arrived"},
		{"a future date", refundRequest{Credits: 1, RequestedBy: s.ownerID, RequestedOn: "2026-11-03"}, "requestedOn", "Date the request arrived must be today or in the past"},
		{"too many", s.order(6), "credits", "Number of Credits must be 5 or fewer"},
	}
	for _, c := range cases {
		w := s.refund(cookie, "refund-key-"+c.name, c.body)
		if w.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: status = %d, want 422; body %s", c.name, w.Code, w.Body)
			continue
		}
		if got := decodeError(t, w).Details[c.field]; got != c.want {
			t.Errorf("%s: details[%s] = %q, want %q", c.name, c.field, got, c.want)
		}
	}
	if s.stepUpAt(cookie) == nil {
		t.Error("a refused answer spent the step-up")
	}
	if len(s.stripe.RefundCalls()) != 0 || s.count(`SELECT count(*) FROM operator_acts WHERE action = 'refund'`) != 0 {
		t.Error("a refused answer moved money or recorded an act")
	}
}

func TestRefund_SaysWhenNothingCanBeRefunded(t *testing.T) {
	s := newRefundStack(t)
	cookie := s.steppedUpCookie()
	if _, err := s.db.Admin.ExecContext(t.Context(), `UPDATE credit_ledger SET created_at = $2 WHERE id = $1`,
		s.lotID, opsStart.AddDate(-billing.RefundWindowYears, 0, -1)); err != nil {
		t.Fatalf("age the lot: %v", err)
	}

	w := s.refund(cookie, "refund-key", s.order(1))

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body %s", w.Code, w.Body)
	}
	if got := decodeError(t, w).Message; got != msgNothingRefundable {
		t.Errorf("message = %q", got)
	}
}

func TestRefund_AStripeRefusalChangesNothing(t *testing.T) {
	s := newRefundStack(t)
	s.stripe.RefundPaymentErr = errors.New("card network down")
	cookie := s.steppedUpCookie()

	w := s.refund(cookie, "refund-key", s.order(1))

	if w.Code != http.StatusBadGateway || decodeError(t, w).Message != msgStripeRefused {
		t.Fatalf("status = %d, body %s, want 502 saying Stripe refused", w.Code, w.Body)
	}
	if s.count(`SELECT count(*) FROM operator_acts WHERE action = 'refund'`) != 0 ||
		s.count(`SELECT count(*) FROM credit_ledger WHERE origin = 'refund'`) != 0 ||
		s.count(`SELECT count(*) FROM activity WHERE action = 'credits_refunded'`) != 0 {
		t.Error("a refund Stripe refused left an act, a ledger row or an Activity row")
	}
	if s.stepUpAt(cookie) == nil {
		t.Error("a refund Stripe refused spent the step-up")
	}
}

func TestRefund_AnUnknownPracticeIsNotFound(t *testing.T) {
	s := newRefundStack(t)
	cookie := s.steppedUpCookie()

	for _, id := range []string{malformedPracticeID, unknownPracticeID} {
		header := http.Header{headerIdempotencyKey: {"refund-key-" + id}}
		if w := s.send(http.MethodPost, "/api/practices/"+id+"/refunds", cookie, header, s.order(1)); w.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", id, w.Code)
		}
	}
}

func TestRefundAnswers_ARequestDateIsCheckedAgainstTodayInNewYork(t *testing.T) {
	req := refundRequest{Credits: 1, RequestedBy: "6f1c2a52-0d6e-4a64-8f5e-6d1b0c1e9a10"}
	for _, tc := range []struct {
		name, on string
		now      time.Time
		future   bool
	}{
		{"8pm, today", "2026-10-30", time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC), false},
		{"8pm, tomorrow", "2026-10-31", time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC), true},
		{"11:59pm, today", "2026-10-30", time.Date(2026, 10, 31, 3, 59, 0, 0, time.UTC), false},
		{"11:59pm, tomorrow", "2026-10-31", time.Date(2026, 10, 31, 3, 59, 0, 0, time.UTC), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req.RequestedOn = tc.on
			got := answerErrors(req, tc.now)["requestedOn"]
			if want := map[bool]string{true: msgDateInTheFuture}[tc.future]; got != want {
				t.Errorf("requestedOn %s at %s = %q, want %q", tc.on, tc.now, got, want)
			}
		})
	}
}
