package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const (
	pilotAddress    = "ana@example.com"
	pilotPath       = "/api/pilot-allowlist"
	fieldAddress    = "address"
	anotherOperator = "Someone Else"
)

func pilotBody(address any) map[string]any {
	return map[string]any{fieldAddress: address}
}

func (s *opsStack) addPilotAddress(cookie, key string, body any) *httptest.ResponseRecorder {
	s.t.Helper()
	header := http.Header{}
	if key != "" {
		header.Set(headerIdempotencyKey, key)
	}
	return s.send(http.MethodPost, pilotPath, cookie, header, body)
}

func (s *opsStack) pilotRows() int {
	s.t.Helper()
	return s.count(`SELECT count(*) FROM pilot_allowlist`)
}

func (s *opsStack) pilotActs() int {
	s.t.Helper()
	return s.count(`SELECT count(*) FROM operator_acts WHERE action = 'pilot_address_added'`)
}

func TestPilotAddressAdd_AddsTheAddressAndRecordsWhoAddedIt(t *testing.T) {
	s := newOpsStack(t, false)
	cookie := s.signedInCookie()
	s.now = s.now.Add(time.Minute)

	w := s.addPilotAddress(cookie, "pilot-key-1", pilotBody("  Ana@Example.com "))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", w.Code, w.Body)
	}
	want := `{"address":"ana@example.com","addedAt":"2026-11-02T09:01:00Z"}`
	if got := strings.TrimSpace(w.Body.String()); got != want {
		t.Errorf("body = %s\nwant %s", got, want)
	}

	var actID string
	var subjectKind *string
	var details []byte
	var stepUpAt *time.Time
	var createdAt time.Time
	if err := s.db.Admin.QueryRowContext(t.Context(),
		`SELECT a.id, a.subject_kind, a.details, a.step_up_at, a.created_at
		   FROM operator_acts a JOIN operators o ON o.id = a.operator_id
		  WHERE o.identity_uid = $1 AND a.action = 'pilot_address_added'`, opUID,
	).Scan(&actID, &subjectKind, &details, &stepUpAt, &createdAt); err != nil {
		t.Fatalf("read the act: %v", err)
	}
	if subjectKind != nil || stepUpAt != nil || !createdAt.Equal(s.now) {
		t.Errorf("act = (subject %v, step_up_at %v, created_at %s), want no subject, no step-up, taken at %s",
			subjectKind, stepUpAt, createdAt, s.now)
	}
	if string(details) != `{"address": "ana@example.com", "requestKey": "pilot-key-1"}` {
		t.Errorf("act details = %s", details)
	}
	if n := s.count(`SELECT count(*) FROM pilot_allowlist WHERE address = $1 AND operator_act_id = $2 AND added_at = $3`,
		pilotAddress, actID, s.now); n != 1 {
		t.Errorf("allowlist rows keyed to the act = %d, want 1", n)
	}
}

func TestPilotAddressAdd_ARetryUnderTheSameKeyIsTheSameAdd(t *testing.T) {
	s := newOpsStack(t, false)
	cookie := s.signedInCookie()

	first := s.addPilotAddress(cookie, "pilot-key-1", pilotBody(pilotAddress))
	s.now = s.now.Add(time.Minute)
	second := s.addPilotAddress(cookie, "pilot-key-1", pilotBody("ANA@example.com"))

	if second.Code != http.StatusOK || second.Body.String() != first.Body.String() {
		t.Errorf("retry = %d %s, want 200 %s", second.Code, second.Body, first.Body)
	}
	if s.pilotActs() != 1 || s.pilotRows() != 1 {
		t.Errorf("acts = %d, rows = %d, want 1 and 1", s.pilotActs(), s.pilotRows())
	}
}

func TestPilotAddressAdd_RefusesAKeyAlreadyUsedForAnotherAddress(t *testing.T) {
	s := newOpsStack(t, false)
	cookie := s.signedInCookie()
	s.addPilotAddress(cookie, "pilot-key-1", pilotBody(pilotAddress))

	w := s.addPilotAddress(cookie, "pilot-key-1", pilotBody("bea@example.com"))

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body %s", w.Code, w.Body)
	}
	if got := decodeError(t, w).Message; got != msgPilotKeyReused {
		t.Errorf("message = %q, want %q", got, msgPilotKeyReused)
	}
	if s.pilotActs() != 1 || s.pilotRows() != 1 {
		t.Errorf("acts = %d, rows = %d, want 1 and 1", s.pilotActs(), s.pilotRows())
	}
}

func TestPilotAddressAdd_RefusesAnAddressAlreadyOnTheList(t *testing.T) {
	s := newOpsStack(t, false)
	cookie := s.signedInCookie()
	s.addPilotAddress(cookie, "pilot-key-1", pilotBody(pilotAddress))
	s.now = s.now.AddDate(0, 0, 3)

	w := s.addPilotAddress(s.signedInCookie(), "pilot-key-2", pilotBody("Ana@Example.com"))

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body %s", w.Code, w.Body)
	}
	want := "ana@example.com is already on the pilot allowlist. It was added on 2 November 2026, and nothing has changed."
	if got := decodeError(t, w).Message; got != want {
		t.Errorf("message = %q\nwant %q", got, want)
	}
	if s.pilotActs() != 1 || s.pilotRows() != 1 {
		t.Errorf("acts = %d, rows = %d, want 1 and 1", s.pilotActs(), s.pilotRows())
	}
}

// An address added at 01:00 UTC on 3 November is still 2 November in
// New York, which is the date the refusal says.
func TestPilotAddressAdd_TheRefusalDatesTheAddInNewYork(t *testing.T) {
	s := newOpsStack(t, false)
	s.now = time.Date(2026, 11, 3, 1, 0, 0, 0, time.UTC)
	cookie := s.signedInCookie()
	s.addPilotAddress(cookie, "pilot-key-1", pilotBody(pilotAddress))

	w := s.addPilotAddress(cookie, "pilot-key-2", pilotBody(pilotAddress))

	if got := decodeError(t, w).Message; !strings.Contains(got, "added on 2 November 2026") {
		t.Errorf("message = %q, want the New York date", got)
	}
}

func TestPilotAddressAdd_RefusesAnAddressThatIsNotOne(t *testing.T) {
	s := newOpsStack(t, false)
	cookie := s.signedInCookie()

	for name, address := range map[string]string{
		"empty":            "  ",
		"no at sign":       "ana.example.com",
		"nothing after at": "ana@",
	} {
		w := s.addPilotAddress(cookie, "pilot-key-"+name, pilotBody(address))
		if w.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: status = %d, want 422", name, w.Code)
			continue
		}
		e := decodeError(t, w)
		if e.Details[fieldAddress] != msgPilotAddressFormat {
			t.Errorf("%s: details = %v, want address: %q", name, e.Details, msgPilotAddressFormat)
		}
	}
	if s.pilotActs() != 0 || s.pilotRows() != 0 {
		t.Errorf("acts = %d, rows = %d, want none", s.pilotActs(), s.pilotRows())
	}
}

func TestPilotAddressAdd_TakesNoActorAndNoOtherField(t *testing.T) {
	s := newOpsStack(t, false)
	cookie := s.signedInCookie()

	for name, body := range map[string]any{
		"addedBy":  map[string]any{fieldAddress: pilotAddress, "addedBy": anotherOperator},
		"actor":    map[string]any{fieldAddress: pilotAddress, "actor": anotherOperator},
		"not text": pilotBody(7),
	} {
		w := s.addPilotAddress(cookie, "pilot-key-"+name, body)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", name, w.Code)
			continue
		}
		if got := decodeError(t, w).Message; got != msgPilotOtherField {
			t.Errorf("%s: message = %q, want %q", name, got, msgPilotOtherField)
		}
	}
	if s.pilotActs() != 0 || s.pilotRows() != 0 {
		t.Errorf("acts = %d, rows = %d, want none", s.pilotActs(), s.pilotRows())
	}
}

func TestPilotAddressAdd_NeedsAnIdempotencyKey(t *testing.T) {
	s := newOpsStack(t, false)

	w := s.addPilotAddress(s.signedInCookie(), "", pilotBody(pilotAddress))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	if got := decodeError(t, w).Message; got != msgPilotKeyRequired {
		t.Errorf("message = %q, want %q", got, msgPilotKeyRequired)
	}
	if s.pilotRows() != 0 {
		t.Errorf("rows = %d, want none", s.pilotRows())
	}
}

func TestPilotAddressAdd_RefusesWithoutASession(t *testing.T) {
	s := newOpsStack(t, false)

	w := s.addPilotAddress("", "pilot-key-1", pilotBody(pilotAddress))

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
	if s.pilotActs() != 0 || s.pilotRows() != 0 {
		t.Errorf("acts = %d, rows = %d, want none", s.pilotActs(), s.pilotRows())
	}
}
