package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/testdb"
)

const (
	pilotGatePath     = "/api/pilot-gate"
	pilotGateOpenPath = "/api/pilot-gate/openings"
)

// closedGateStack is an ops stack whose pilot gate is closed, as the
// deployed database is until January; the testdb template opens it.
func closedGateStack(t *testing.T) *opsStack {
	t.Helper()
	s := newOpsStack(t, false)
	testdb.ClosePilotGate(t, s.db)
	return s
}

func (s *opsStack) openPilotGate(cookie, key string, body any) *httptest.ResponseRecorder {
	s.t.Helper()
	header := http.Header{}
	if key != "" {
		header.Set(headerIdempotencyKey, key)
	}
	return s.send(http.MethodPost, pilotGateOpenPath, cookie, header, body)
}

func (s *opsStack) gateActs() int {
	s.t.Helper()
	return s.count(`SELECT count(*) FROM operator_acts WHERE action = 'pilot_gate_opened'`)
}

func (s *opsStack) gateIsOpen() bool {
	s.t.Helper()
	return s.count(`SELECT count(*) FROM pilot_gate WHERE opened_at IS NOT NULL`) == 1
}

func (s *opsStack) admits(address string) bool {
	s.t.Helper()
	var admitted bool
	if err := s.db.App.QueryRowContext(s.t.Context(), `SELECT pilot_admits($1)`, address).Scan(&admitted); err != nil {
		s.t.Fatalf("ask pilot_admits: %v", err)
	}
	return admitted
}

func TestPilotGateOpen_OpensTheGateForEveryoneAndRecordsWhoOpenedIt(t *testing.T) {
	s := closedGateStack(t)
	cookie := s.steppedUpCookie()
	steppedUp := s.now
	s.now = s.now.Add(time.Minute)
	if s.admits("anyone@example.com") {
		t.Fatal("a closed gate admits an address not on the allowlist")
	}

	w := s.openPilotGate(cookie, "gate-key-1", map[string]any{})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", w.Code, w.Body)
	}
	if want := `{"openedAt":"2026-11-02T09:01:00Z"}`; strings.TrimSpace(w.Body.String()) != want {
		t.Errorf("body = %s\nwant %s", w.Body, want)
	}
	var actID string
	var subjectKind *string
	var details []byte
	var stepUpAt *time.Time
	var createdAt time.Time
	if err := s.db.Admin.QueryRowContext(t.Context(),
		`SELECT a.id, a.subject_kind, a.details, a.step_up_at, a.created_at
		   FROM operator_acts a JOIN operators o ON o.id = a.operator_id
		  WHERE o.identity_uid = $1 AND a.action = 'pilot_gate_opened'`, opUID,
	).Scan(&actID, &subjectKind, &details, &stepUpAt, &createdAt); err != nil {
		t.Fatalf("read the act: %v", err)
	}
	if subjectKind != nil || stepUpAt == nil || !stepUpAt.Equal(steppedUp) || !createdAt.Equal(s.now) {
		t.Errorf("act = (subject %v, step_up_at %v, created_at %s), want no subject, stepped up at %s, taken at %s",
			subjectKind, stepUpAt, createdAt, steppedUp, s.now)
	}
	if string(details) != `{"requestKey": "gate-key-1"}` {
		t.Errorf("act details = %s", details)
	}
	if n := s.count(`SELECT count(*) FROM pilot_gate WHERE operator_act_id = $1 AND opened_at = $2`, actID, s.now); n != 1 {
		t.Errorf("gate rows keyed to the act = %d, want 1", n)
	}
	if !s.admits("anyone@example.com") {
		t.Error("an open gate does not admit every address")
	}
	if s.stepUpAt(cookie) != nil {
		t.Error("the step-up is not spent")
	}
}

func TestPilotGateOpen_ARetryUnderTheSameKeyIsTheSameOpening(t *testing.T) {
	s := closedGateStack(t)
	first := s.openPilotGate(s.steppedUpCookie(), "gate-key-1", map[string]any{})
	s.now = s.now.Add(time.Minute)

	second := s.openPilotGate(s.steppedUpCookie(), "gate-key-1", map[string]any{})

	if second.Code != http.StatusOK || second.Body.String() != first.Body.String() {
		t.Errorf("retry = %d %s, want 200 %s", second.Code, second.Body, first.Body)
	}
	if s.gateActs() != 1 {
		t.Errorf("acts = %d, want 1", s.gateActs())
	}
}

func TestPilotGateOpen_RefusesAGateAlreadyOpen(t *testing.T) {
	s := closedGateStack(t)
	s.now = time.Date(2027, 1, 2, 1, 0, 0, 0, time.UTC)
	s.openPilotGate(s.steppedUpCookie(), "gate-key-1", map[string]any{})
	s.now = s.now.AddDate(0, 0, 3)

	w := s.openPilotGate(s.steppedUpCookie(), "gate-key-2", map[string]any{})

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body %s", w.Code, w.Body)
	}
	want := "The pilot gate is already open. It was opened on January 1, 2027, and nothing has changed."
	if got := decodeError(t, w).Message; got != want {
		t.Errorf("message = %q\nwant %q", got, want)
	}
	if s.gateActs() != 1 {
		t.Errorf("acts = %d, want 1", s.gateActs())
	}
}

// The gate the testdb template opens has no act behind it, the shape a
// gate opened before ops had the act takes; it is refused all the same.
func TestPilotGateOpen_RefusesAGateOpenedWithNoActOnRecord(t *testing.T) {
	s := newOpsStack(t, false)

	w := s.openPilotGate(s.steppedUpCookie(), "gate-key-1", map[string]any{})

	if w.Code != http.StatusConflict || !strings.HasPrefix(decodeError(t, w).Message, "The pilot gate is already open.") {
		t.Fatalf("status = %d, body %s; want the already-open refusal", w.Code, w.Body)
	}
	if s.gateActs() != 0 {
		t.Errorf("acts = %d, want none", s.gateActs())
	}
}

func TestPilotGateOpen_RefusesAStaleOrMissingStepUp(t *testing.T) {
	s := closedGateStack(t)
	stale := s.steppedUpCookie()
	s.now = s.now.Add(stepUpLimit)

	for name, cookie := range map[string]string{"stale": stale, "missing": s.signedInCookie()} {
		w := s.openPilotGate(cookie, "gate-key-"+name, map[string]any{})
		if w.Code != http.StatusForbidden || decodeError(t, w).Code != apierr.CodeSecondFactorRequired {
			t.Errorf("%s: status = %d, body %s; want 403 %s", name, w.Code, w.Body, apierr.CodeSecondFactorRequired)
		}
	}
	if s.gateIsOpen() || s.gateActs() != 0 {
		t.Errorf("open = %v, acts = %d, want closed with none", s.gateIsOpen(), s.gateActs())
	}
}

func TestPilotGateOpen_RefusesWithoutASession(t *testing.T) {
	s := closedGateStack(t)

	w := s.openPilotGate("", "gate-key-1", map[string]any{})

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
	if s.gateIsOpen() || s.gateActs() != 0 {
		t.Errorf("open = %v, acts = %d, want closed with none", s.gateIsOpen(), s.gateActs())
	}
}

func TestPilotGateOpen_TakesNoActorAndNoOtherField(t *testing.T) {
	s := closedGateStack(t)

	for _, field := range []string{"openedBy", "openerName"} {
		w := s.openPilotGate(s.steppedUpCookie(), "gate-key-"+field, map[string]any{field: anotherOperator})
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", field, w.Code)
			continue
		}
		if got := decodeError(t, w).Message; got != msgGateNoFields {
			t.Errorf("%s: message = %q, want %q", field, got, msgGateNoFields)
		}
	}
	if s.gateIsOpen() || s.gateActs() != 0 {
		t.Errorf("open = %v, acts = %d, want closed with none", s.gateIsOpen(), s.gateActs())
	}
}

func TestPilotGateOpen_NeedsAnIdempotencyKey(t *testing.T) {
	s := closedGateStack(t)

	w := s.openPilotGate(s.steppedUpCookie(), "", map[string]any{})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	if got := decodeError(t, w).Message; got != "Send an Idempotency-Key header with the opening." {
		t.Errorf("message = %q, want %q", got, "Send an Idempotency-Key header with the opening.")
	}
	if s.gateIsOpen() {
		t.Error("the gate opened")
	}
}

func TestPilotGateRead_SaysWhetherTheGateIsOpen(t *testing.T) {
	s := closedGateStack(t)
	cookie := s.signedInCookie()

	closed := s.send(http.MethodGet, pilotGatePath, cookie, nil, nil)
	if closed.Code != http.StatusOK || strings.TrimSpace(closed.Body.String()) != `{"openedAt":null}` {
		t.Fatalf("closed = %d %s, want 200 {\"openedAt\":null}", closed.Code, closed.Body)
	}

	s.openPilotGate(s.steppedUpCookie(), "gate-key-1", map[string]any{})
	open := s.send(http.MethodGet, pilotGatePath, cookie, nil, nil)
	if want := `{"openedAt":"2026-11-02T09:00:00Z"}`; open.Code != http.StatusOK || strings.TrimSpace(open.Body.String()) != want {
		t.Errorf("open = %d %s, want 200 %s", open.Code, open.Body, want)
	}
}

func TestPilotGateRead_RefusesWithoutASession(t *testing.T) {
	s := closedGateStack(t)

	if w := s.send(http.MethodGet, pilotGatePath, "", nil, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}
