package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/legal"
	"doula-cloud/api/internal/testdb"
)

// The history these tests publish from, dated from opsStart (November 2,
// 2026). The shipped one holds no material version, and must not be
// given a made-up one.
//
// Privacy 2026-12-15 is material and 43 days out, so it may be published.
// Privacy 2026-10-27 is not material, so it emails no one.
// Terms 2026-11-20 is material and 18 days out, too soon.
// Terms 2026-12-20 raises the price 48 days out: too soon for a rise.
// Terms 2027-01-15 raises the price 74 days out, so it may be published.
var legalFixture = []legal.Document{
	{Name: "Terms of Service", Path: "/terms", Versions: []legal.Version{
		{Effective: "2026-09-29", Change: "First version.", CreditPriceCents: 2000},
		{Effective: "2026-11-20", Material: true, Change: "Says how a Credit is refunded.", CreditPriceCents: 2000},
		{Effective: "2026-12-20", Material: true, Change: "A Credit costs $45.", CreditPriceCents: 4500, PriceRise: "Card fees went up."},
		{Effective: "2027-01-15", Material: true, Change: "A Credit costs $50.", CreditPriceCents: 5000, PriceRise: "Text messages cost more to send."},
	}},
	{Name: "Privacy Policy", Path: "/privacy", Versions: []legal.Version{
		{Effective: "2026-09-29", Change: "First version."},
		{Effective: "2026-10-27", Change: "Corrects a typing error."},
		{Effective: "2026-12-15", Material: true, Change: "Names a new processor of your data."},
	}},
}

type legalStack struct {
	*opsStack
	stripe *billing.FakeStripeClient
}

func newLegalStack(t *testing.T) *legalStack {
	t.Helper()
	s := &legalStack{opsStack: &opsStack{t: t, db: testdb.New(t), now: opsStart}, stripe: billing.NewFakeStripeClient()}
	s.handler, _ = routes(Deps{
		DB:       s.db.Ops,
		Now:      func() time.Time { return s.now },
		Verifier: tokens,
		Origins:  []string{testOrigin},
		Stripe:   s.stripe,
		Legal:    legalFixture,
	})
	return s
}

// seedOwners is four Owners to email: Ana and Bea at Harbor, Ana and Dee
// at Willow. A Doula, a Practice in its deletion window and a deleted
// Practice are not emailed.
func (s *legalStack) seedOwners() {
	s.t.Helper()
	owner := []string{ownerRole}
	harbor := testdb.SeedPractice(s.t, s.db, "Harbor Doulas")
	ana := testdb.SeedStaffAtPractice(s.t, s.db, harbor, "ana", owner, "employee")
	testdb.SeedStaffAtPractice(s.t, s.db, harbor, "bea", []string{ownerRole, "admin"}, "employee")
	testdb.SeedStaffAtPractice(s.t, s.db, harbor, "cal", []string{"doula"}, "contractor")
	willow := testdb.SeedPractice(s.t, s.db, "Willow Birth")
	testdb.SeedStaffAtPractice(s.t, s.db, willow, "dee", owner, "employee")
	leaving := testdb.SeedPractice(s.t, s.db, "Leaving Practice")
	testdb.SeedStaffAtPractice(s.t, s.db, leaving, "eve", owner, "employee")
	gone := testdb.SeedPractice(s.t, s.db, "Gone Practice")
	testdb.SeedStaffAtPractice(s.t, s.db, gone, "fay", owner, "employee")
	for _, statement := range []string{
		`INSERT INTO practice_memberships (practice_id, staff_id, roles, employment_type) VALUES ('` + willow + `', '` + ana + `', '{owner}', 'employee')`,
		`UPDATE practices SET deletion_requested_at = now(), deletion_finalize_at = now() + interval '30 days' WHERE id = '` + leaving + `'`,
		`UPDATE practices SET deletion_requested_at = now(), deletion_finalize_at = now(), deleted_at = now() WHERE id = '` + gone + `'`,
	} {
		if _, err := s.db.Admin.ExecContext(s.t.Context(), statement); err != nil {
			s.t.Fatalf("seed owners: %v", err)
		}
	}
}

func legalVersionPath(document, version string) string {
	return "/api/legal-versions/" + document + "/" + version
}

func (s *legalStack) publish(cookie, key, document, version string, body any) *httptest.ResponseRecorder {
	s.t.Helper()
	header := http.Header{}
	if key != "" {
		header.Set(headerIdempotencyKey, key)
	}
	return s.send(http.MethodPost, legalVersionPath(document, version)+"/publications", cookie, header, body)
}

func (s *legalStack) publications() int {
	s.t.Helper()
	return s.count(`SELECT count(*) FROM legal_version_publications`)
}

func (s *legalStack) publishActs() int {
	s.t.Helper()
	return s.count(`SELECT count(*) FROM operator_acts WHERE action = 'legal_version_published'`)
}

func (s *legalStack) queued() int {
	s.t.Helper()
	return s.count(`SELECT count(*) FROM legal_notice_outbox`)
}

func decodeInto[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %s: %v", w.Body, err)
	}
	return v
}

func TestLegalPublish_EmailsTheOwnersTheCheckPageCounted(t *testing.T) {
	s := newLegalStack(t)
	s.seedOwners()
	cookie := s.steppedUpCookie()
	steppedUp := s.now
	s.now = s.now.Add(time.Minute)

	check := s.send(http.MethodGet, legalVersionPath("privacy", "2026-12-15"), cookie, nil, nil)
	want := `{"document":"privacy","documentName":"Privacy Policy","version":"2026-12-15","change":"Names a new processor of your data.","material":true,"owners":4,"publishedAt":null}`
	if check.Code != http.StatusOK || strings.TrimSpace(check.Body.String()) != want {
		t.Fatalf("check = %d %s\nwant 200 %s", check.Code, check.Body, want)
	}

	w := s.publish(cookie, "legal-key-1", "privacy", "2026-12-15", map[string]any{})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", w.Code, w.Body)
	}
	want = `{"document":"privacy","documentName":"Privacy Policy","version":"2026-12-15","material":true,"publishedAt":"2026-11-02T09:01:00Z","queued":4}`
	if got := strings.TrimSpace(w.Body.String()); got != want {
		t.Errorf("body = %s\nwant %s", got, want)
	}
	if s.queued() != 4 {
		t.Errorf("outbox rows = %d, want the 4 the check page counted", s.queued())
	}
	var actID, details string
	var subjectKind *string
	var stepUpAt *time.Time
	var createdAt time.Time
	if err := s.db.Admin.QueryRowContext(t.Context(),
		`SELECT a.id, a.subject_kind, a.details::text, a.step_up_at, a.created_at
		   FROM operator_acts a JOIN operators o ON o.id = a.operator_id
		  WHERE o.identity_uid = $1 AND a.action = 'legal_version_published'`, opUID,
	).Scan(&actID, &subjectKind, &details, &stepUpAt, &createdAt); err != nil {
		t.Fatalf("read the act: %v", err)
	}
	if subjectKind != nil || stepUpAt == nil || !stepUpAt.Equal(steppedUp) || !createdAt.Equal(s.now) {
		t.Errorf("act = (subject %v, step_up_at %v, created_at %s), want no subject, stepped up at %s, taken at %s",
			subjectKind, stepUpAt, createdAt, steppedUp, s.now)
	}
	if want := `{"version": "2026-12-15", "document": "privacy", "material": true, "requestKey": "legal-key-1"}`; details != want {
		t.Errorf("act details = %s\nwant %s", details, want)
	}
	if n := s.count(`SELECT count(*) FROM legal_version_publications
	                  WHERE operator_act_id = $1 AND document = 'privacy' AND version = '2026-12-15' AND material AND published_at = $2`,
		actID, s.now); n != 1 {
		t.Errorf("publications keyed to the act = %d, want 1", n)
	}
	if s.stepUpAt(cookie) != nil {
		t.Error("the step-up is not spent")
	}
}

func TestLegalPublish_ANotMaterialVersionEmailsNoOne(t *testing.T) {
	s := newLegalStack(t)
	s.seedOwners()
	cookie := s.steppedUpCookie()

	check := decodeInto[legalVersionCheck](t, s.send(http.MethodGet, legalVersionPath("privacy", "2026-10-27"), cookie, nil, nil))
	if check.Material || check.Owners != 0 {
		t.Errorf("check = %+v, want not material and no Owners", check)
	}
	w := s.publish(cookie, "legal-key-1", "privacy", "2026-10-27", map[string]any{})

	if p := decodeInto[legalPublication](t, w); w.Code != http.StatusOK || p.Material || p.Queued != 0 {
		t.Fatalf("publish = %d %s, want 200 with nothing queued", w.Code, w.Body)
	}
	if s.publications() != 1 || s.queued() != 0 {
		t.Errorf("publications = %d, outbox rows = %d, want 1 and 0", s.publications(), s.queued())
	}
}

func TestLegalPublish_ARetryUnderTheSameKeyIsTheSamePublication(t *testing.T) {
	s := newLegalStack(t)
	s.seedOwners()
	first := s.publish(s.steppedUpCookie(), "legal-key-1", "privacy", "2026-12-15", map[string]any{})
	s.now = s.now.Add(time.Minute)

	second := s.publish(s.steppedUpCookie(), "legal-key-1", "privacy", "2026-12-15", map[string]any{})

	if second.Code != http.StatusOK || second.Body.String() != first.Body.String() {
		t.Errorf("retry = %d %s, want 200 %s", second.Code, second.Body, first.Body)
	}
	if s.publishActs() != 1 || s.queued() != 4 {
		t.Errorf("acts = %d, outbox rows = %d, want 1 and 4", s.publishActs(), s.queued())
	}
}

func TestLegalPublish_RefusesAKeyUsedForAnotherVersion(t *testing.T) {
	s := newLegalStack(t)
	s.publish(s.steppedUpCookie(), "legal-key-1", "privacy", "2026-10-27", map[string]any{})

	w := s.publish(s.steppedUpCookie(), "legal-key-1", "privacy", "2026-12-15", map[string]any{})

	if w.Code != http.StatusUnprocessableEntity || decodeError(t, w).Message != msgLegalKeyReused {
		t.Fatalf("status = %d, body %s; want 422 %q", w.Code, w.Body, msgLegalKeyReused)
	}
	if s.publications() != 1 || s.publishActs() != 1 {
		t.Errorf("publications = %d, acts = %d, want 1 and 1", s.publications(), s.publishActs())
	}
}

func TestLegalPublish_RefusesAVersionAlreadyPublished(t *testing.T) {
	s := newLegalStack(t)
	s.seedOwners()
	s.now = time.Date(2026, 11, 3, 2, 0, 0, 0, time.UTC)
	s.publish(s.steppedUpCookie(), "legal-key-1", "privacy", "2026-12-15", map[string]any{})
	s.now = s.now.Add(time.Hour)
	cookie := s.steppedUpCookie()

	w := s.publish(cookie, "legal-key-2", "privacy", "2026-12-15", map[string]any{})

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body %s", w.Code, w.Body)
	}
	want := "This version was already published on November 2, 2026, and nothing has changed."
	if got := decodeError(t, w).Message; got != want {
		t.Errorf("message = %q\nwant %q", got, want)
	}
	if s.publishActs() != 1 || s.queued() != 4 {
		t.Errorf("acts = %d, outbox rows = %d, want 1 and 4", s.publishActs(), s.queued())
	}
	if s.stepUpAt(cookie) == nil {
		t.Error("a refused publication spent the step-up")
	}
	check := decodeInto[legalVersionCheck](t, s.send(http.MethodGet, legalVersionPath("privacy", "2026-12-15"), cookie, nil, nil))
	if check.PublishedAt == nil || !check.PublishedAt.Equal(time.Date(2026, 11, 3, 2, 0, 0, 0, time.UTC)) {
		t.Errorf("check publishedAt = %v, want the first publication", check.PublishedAt)
	}
}

func TestLegalPublish_RefusesWhatTheTermsDoNotAllow(t *testing.T) {
	cases := []struct {
		name, document, version, want string
		stripeErr                     error
	}{
		{"a material version fewer than 30 days out", docTerms, "2026-11-20",
			"A material version has to take effect at least 30 days after the emails are sent, and this one takes effect on November 20, 2026. Nothing has changed.", nil},
		{"a price rise fewer than 60 days out", docTerms, "2026-12-20",
			"A version that raises the price of a Credit has to take effect at least 60 days after the emails are sent, and this one takes effect on December 20, 2026. Nothing has changed.", nil},
		{"a price no Stripe Price charges", docTerms, "2027-01-15",
			"No Stripe Price charges the $50.00 a Credit costs in this version. Create it first (docs/environment.md, \"A rise in the price of a Credit\"). Nothing has changed.",
			errors.New("billing: no active Price of 5000 cents")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newLegalStack(t)
			s.seedOwners()
			s.stripe.CheckCreditPriceErr = c.stripeErr
			cookie := s.steppedUpCookie()

			w := s.publish(cookie, "legal-key-1", c.document, c.version, map[string]any{})

			if w.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422; body %s", w.Code, w.Body)
			}
			if got := decodeError(t, w).Message; got != c.want {
				t.Errorf("message = %q\nwant %q", got, c.want)
			}
			if s.publications() != 0 || s.queued() != 0 || s.publishActs() != 0 {
				t.Errorf("publications = %d, outbox rows = %d, acts = %d, want none", s.publications(), s.queued(), s.publishActs())
			}
			if s.stepUpAt(cookie) == nil {
				t.Error("a refused publication spent the step-up")
			}
		})
	}
}

func TestLegalPublish_PublishesAPriceRiseAStripePriceCharges(t *testing.T) {
	s := newLegalStack(t)
	s.seedOwners()

	w := s.publish(s.steppedUpCookie(), "legal-key-1", "terms", "2027-01-15", map[string]any{})

	if p := decodeInto[legalPublication](t, w); w.Code != http.StatusOK || p.Queued != 4 {
		t.Fatalf("publish = %d %s, want 200 with 4 queued", w.Code, w.Body)
	}
}

func TestLegalPublish_RefusesAVersionNotInTheHistory(t *testing.T) {
	s := newLegalStack(t)
	cookie := s.steppedUpCookie()

	for _, path := range [][2]string{{"terms", "2027-02-01"}, {"cookies", "2026-12-15"}} {
		read := s.send(http.MethodGet, legalVersionPath(path[0], path[1]), cookie, nil, nil)
		act := s.publish(cookie, "legal-key-1", path[0], path[1], map[string]any{})
		for name, w := range map[string]*httptest.ResponseRecorder{"read": read, "act": act} {
			if w.Code != http.StatusNotFound || decodeError(t, w).Message != msgNoSuchVersion {
				t.Errorf("%s %v = %d %s, want 404 %q", name, path, w.Code, w.Body, msgNoSuchVersion)
			}
		}
	}
	if s.publications() != 0 {
		t.Errorf("publications = %d, want none", s.publications())
	}
}

func TestLegalPublish_RefusesAStaleOrMissingStepUp(t *testing.T) {
	s := newLegalStack(t)
	s.seedOwners()
	stale := s.steppedUpCookie()
	s.now = s.now.Add(stepUpLimit)

	for name, cookie := range map[string]string{"stale": stale, "missing": s.signedInCookie()} {
		w := s.publish(cookie, "legal-key-"+name, "privacy", "2026-12-15", map[string]any{})
		if w.Code != http.StatusForbidden || decodeError(t, w).Code != apierr.CodeSecondFactorRequired {
			t.Errorf("%s: status = %d, body %s; want 403 %s", name, w.Code, w.Body, apierr.CodeSecondFactorRequired)
		}
	}
	if s.publications() != 0 || s.queued() != 0 || s.publishActs() != 0 {
		t.Errorf("publications = %d, outbox rows = %d, acts = %d, want none", s.publications(), s.queued(), s.publishActs())
	}
}

func TestLegalPublish_RefusesWithoutASession(t *testing.T) {
	s := newLegalStack(t)

	for _, w := range []*httptest.ResponseRecorder{
		s.publish("", "legal-key-1", "privacy", "2026-12-15", map[string]any{}),
		s.send(http.MethodGet, legalVersionPath("privacy", "2026-12-15"), "", nil, nil),
		s.send(http.MethodGet, legalVersionsPath, "", nil, nil),
	} {
		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", w.Code)
		}
	}
	if s.publications() != 0 {
		t.Errorf("publications = %d, want none", s.publications())
	}
}

func TestLegalPublish_TakesNoActorAndNoOtherField(t *testing.T) {
	s := newLegalStack(t)

	for _, field := range []string{"publishedBy", "material"} {
		w := s.publish(s.steppedUpCookie(), "legal-key-"+field, "privacy", "2026-12-15", map[string]any{field: anotherOperator})
		if w.Code != http.StatusBadRequest || decodeError(t, w).Message != msgLegalNoFields {
			t.Errorf("%s: status = %d, body %s; want 400 %q", field, w.Code, w.Body, msgLegalNoFields)
		}
	}
	if s.publications() != 0 {
		t.Errorf("publications = %d, want none", s.publications())
	}
}

func TestLegalPublish_NeedsAnIdempotencyKey(t *testing.T) {
	s := newLegalStack(t)

	w := s.publish(s.steppedUpCookie(), "", "privacy", "2026-12-15", map[string]any{})

	if w.Code != http.StatusBadRequest || decodeError(t, w).Message != msgLegalKeyRequired {
		t.Fatalf("status = %d, body %s; want 400 %q", w.Code, w.Body, msgLegalKeyRequired)
	}
	if s.publications() != 0 {
		t.Errorf("publications = %d, want none", s.publications())
	}
}

const (
	legalVersionsPath = "/api/legal-versions"
	docTerms          = "terms"
)

func TestLegalVersions_ListsEachVersionNewestFirstWithItsPublication(t *testing.T) {
	s := newLegalStack(t)
	cookie := s.steppedUpCookie()
	s.publish(cookie, "legal-key-1", "privacy", "2026-10-27", map[string]any{})

	w := s.send(http.MethodGet, legalVersionsPath, cookie, nil, nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	list := decodeInto[legalVersionList](t, w)
	var got []string
	for _, v := range list.Items {
		published := "-"
		if v.PublishedAt != nil {
			published = v.PublishedAt.Format(time.RFC3339)
		}
		got = append(got, v.Document+" "+v.Version+" "+published)
	}
	want := []string{
		"terms 2027-01-15 -", "terms 2026-12-20 -", "terms 2026-11-20 -", "terms 2026-09-29 -",
		"privacy 2026-12-15 -", "privacy 2026-10-27 2026-11-02T09:00:00Z", "privacy 2026-09-29 -",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("list =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if first := list.Items[0]; first.DocumentName != "Terms of Service" || first.Change != "A Credit costs $50." || !first.Material {
		t.Errorf("first item = %+v", first)
	}
}
