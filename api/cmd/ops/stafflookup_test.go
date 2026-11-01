package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func (s *secondFactorStack) lookup(cookie, email string) *httptest.ResponseRecorder {
	s.t.Helper()
	return s.send(http.MethodGet, "/api/staff?email="+url.QueryEscape(email), cookie, nil, nil)
}

func (s *secondFactorStack) lookups() int {
	s.t.Helper()
	return s.count(`SELECT count(*) FROM operator_acts WHERE action = 'staff_lookup'`)
}

func TestStaffLookup_FindsTheStaffMemberBySignInEmailAndRecordsTheRead(t *testing.T) {
	s := newSecondFactorStack(t)

	w := s.lookup(s.signedInCookie(), "  CLEAR-STAFF-UID@example.com ")

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body %s", w.Code, w.Body)
	}
	var got staffLookupResult
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Staff) != 1 {
		t.Fatalf("staff = %+v, want one", got.Staff)
	}
	found := got.Staff[0]
	if found.StaffID != s.staffID || found.Name != clearStaffName || found.Email != clearStaffUID+"@example.com" {
		t.Errorf("found = %+v", found)
	}
	if len(found.PracticeNames) != 2 || found.PracticeNames[0] != clearPracticeOne || found.PracticeNames[1] != clearPracticeTwo {
		t.Errorf("practices = %v, want %s and %s", found.PracticeNames, clearPracticeOne, clearPracticeTwo)
	}
	var details []byte
	if err := s.db.Admin.QueryRowContext(t.Context(),
		`SELECT details FROM operator_acts WHERE action = 'staff_lookup'`).Scan(&details); err != nil {
		t.Fatalf("read the read row: %v", err)
	}
	var read staffLookupRead
	_ = json.Unmarshal(details, &read)
	if read.Email != "CLEAR-STAFF-UID@example.com" || len(read.StaffIDs) != 1 || read.StaffIDs[0] != s.staffID {
		t.Errorf("read row = %s", details)
	}
}

func TestStaffLookup_ANoMatchShowsNoOneAndRecordsNothing(t *testing.T) {
	s := newSecondFactorStack(t)

	w := s.lookup(s.signedInCookie(), "nobody@example.com")

	if w.Code != http.StatusOK || w.Body.String() != "{\"staff\":[]}\n" {
		t.Fatalf("answer = %d %s, want 200 with no staff", w.Code, w.Body)
	}
	if s.lookups() != 0 {
		t.Errorf("read rows = %d, want 0", s.lookups())
	}
}

func TestStaffLookup_RefusesAMissingOrMalformedEmail(t *testing.T) {
	s := newSecondFactorStack(t)
	cookie := s.signedInCookie()

	for email, want := range map[string]string{"": msgLookupEmail, "not-an-address": msgLookupEmailForm, "Ana <ana@example.com>": msgLookupEmailForm} {
		w := s.lookup(cookie, email)
		if w.Code != http.StatusUnprocessableEntity || decodeError(t, w).Details["email"] != want {
			t.Errorf("lookup(%q) = %d %s, want 422 %q", email, w.Code, w.Body, want)
		}
	}
	if s.lookups() != 0 {
		t.Errorf("read rows = %d, want 0", s.lookups())
	}
}

func TestStaffLookup_RefusesWithoutASession(t *testing.T) {
	s := newSecondFactorStack(t)

	if w := s.lookup("", clearStaffUID+"@example.com"); w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}
