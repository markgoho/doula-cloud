package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"

	"doula-cloud/api/internal/testdb"
)

const (
	logPath   = "/api/operator-log"
	noSubject = "null"
)

// act writes one operator_acts row by the signed-in operator, as the
// handler that takes the act would.
func (s *opsStack) act(action, subjectKind, subjectID, details string, at time.Time) {
	s.t.Helper()
	var kind, id any
	if subjectKind != "" {
		kind, id = subjectKind, subjectID
	}
	s.exec(`INSERT INTO operator_acts (operator_id, action, subject_kind, subject_id, details, created_at)
	        SELECT id, $2, $3, $4, $5, $6 FROM operators WHERE identity_uid = $1`,
		opUID, action, kind, id, details, at)
}

func TestOperatorLog_ShowsEachRowNewestFirstWithItsOperatorAndTime(t *testing.T) {
	s := newOpsStack(t, false)
	cookie := s.signedInCookie()
	s.act("step_up_refused", "", "", `{"reason":"stale_sign_in"}`, opsStart.Add(time.Minute))
	s.act("sign_out", "", "", `{}`, opsStart.Add(2*time.Minute))

	page := decodeList(t, s.list(logPath, cookie))

	got, _ := json.Marshal(page.Items)
	want := compactJSON(t, `[
		{"id": `+string(page.Items[0]["id"])+`, "at": "2026-11-02T09:02:00Z", "operator": "Mark Goho", "action": "sign_out", "subject": null, "details": {}},
		{"id": `+string(page.Items[1]["id"])+`, "at": "2026-11-02T09:01:00Z", "operator": "Mark Goho", "action": "step_up_refused", "subject": null, "details": {"reason": "stale_sign_in"}},
		{"id": `+string(page.Items[2]["id"])+`, "at": "2026-11-02T09:00:00Z", "operator": "Mark Goho", "action": "sign_in", "subject": null, "details": {}}
	]`)
	if compactJSON(t, string(got)) != want {
		t.Errorf("items =\n%s\nwant\n%s", got, want)
	}
	if page.HasMore || page.NextCursor != nil {
		t.Errorf("page = %+v, want no more", page)
	}
}

func TestOperatorLog_NamesEachSubjectAndTheOwnerWhoAskedForARefund(t *testing.T) {
	s := newOpsStack(t, false)
	cookie := s.signedInCookie()
	practiceID := testdb.SeedPractice(t, s.db, "Rooted Birth Collective")
	ownerID := testdb.SeedNamedStaffAtPractice(t, s.db, practiceID, "log-owner", "Renata Alvarez", []string{ownerRole}, "employee")
	gone := testdb.SeedPractice(t, s.db, "Gone Births")
	s.exec(`UPDATE practices SET deleted_at = $2 WHERE id = $1`, gone, opsStart)
	const feedbackID = "6f1c2c1e-9a52-4b0e-8a3c-2f9d3c1d6a11"
	const missingStaff = "0b7c1f2a-3d4e-4f50-8a6b-7c8d9e0f1a2b"
	s.act("refund", "practice", practiceID, `{"requestedByStaffId":"`+ownerID+`","requestedOn":"2026-10-30","credits":2,"amountCents":5000,"taxCents":400}`, opsStart.Add(4*time.Minute))
	s.act("founding_grant", "practice", gone, `{"staffCount":3,"credits":30}`, opsStart.Add(3*time.Minute))
	s.act("second_factor_cleared", "staff", missingStaff, `{}`, opsStart.Add(2*time.Minute))
	s.act("feedback_read", "feedback", feedbackID, `{}`, opsStart.Add(time.Minute))

	page := decodeList(t, s.list(logPath, cookie))

	type row struct {
		Subject     json.RawMessage `json:"subject"`
		RequestedBy json.RawMessage `json:"requestedBy"`
	}
	var rows []row
	raw, _ := json.Marshal(page.Items)
	_ = json.Unmarshal(raw, &rows)
	if len(rows) != 5 {
		t.Fatalf("rows = %s, want five", raw)
	}
	wants := []struct{ subject, requestedBy string }{
		{`{"kind":"practice","id":"` + practiceID + `","name":"Rooted Birth Collective","deleted":false}`, `{"id":"` + ownerID + `","name":"Renata Alvarez"}`},
		{`{"kind":"practice","id":"` + gone + `","name":"Gone Births","deleted":true}`, ``},
		{`{"kind":"staff","id":"` + missingStaff + `","name":null,"deleted":false}`, ``},
		{`{"kind":"feedback","id":"` + feedbackID + `","name":null,"deleted":false}`, ``},
		{noSubject, ``},
	}
	for i, want := range wants {
		if string(rows[i].Subject) != want.subject {
			t.Errorf("row %d subject = %s, want %s", i, rows[i].Subject, want.subject)
		}
		if string(rows[i].RequestedBy) != want.requestedBy {
			t.Errorf("row %d requestedBy = %s, want %s", i, rows[i].RequestedBy, want.requestedBy)
		}
	}
}

func TestOperatorLog_ARefundWhoseOwnerIsNotOnFileStillShowsTheId(t *testing.T) {
	s := newOpsStack(t, false)
	cookie := s.signedInCookie()
	practiceID := testdb.SeedPractice(t, s.db, "Rooted Birth Collective")
	const missingOwner = "0b7c1f2a-3d4e-4f50-8a6b-7c8d9e0f1a2b"
	s.act("refund", "practice", practiceID, `{"requestedByStaffId":"`+missingOwner+`","requestedOn":"2026-10-30","credits":1,"amountCents":2500,"taxCents":0}`, opsStart.Add(time.Minute))

	page := decodeList(t, s.list(logPath, cookie))

	if got := string(page.Items[0]["requestedBy"]); got != `{"id":"`+missingOwner+`","name":null}` {
		t.Errorf("requestedBy = %s, want the id with no name", got)
	}
}

func TestOperatorLog_PagesByCursor(t *testing.T) {
	s := newOpsStack(t, false)
	cookie := s.signedInCookie()
	for i := range operatorLogPageSize {
		s.act("sign_out", "", "", `{}`, opsStart.Add(time.Duration(i+1)*time.Second))
	}

	first := decodeList(t, s.list(logPath, cookie))
	if len(first.Items) != operatorLogPageSize || !first.HasMore || first.NextCursor == nil {
		t.Fatalf("first page has %d rows, hasMore %v, cursor %v; want a full page and more", len(first.Items), first.HasMore, first.NextCursor)
	}
	second := decodeList(t, s.list(logPath+"?cursor="+url.QueryEscape(*first.NextCursor), cookie))

	if len(second.Items) != 1 || second.HasMore || second.NextCursor != nil {
		t.Fatalf("second page = %+v, want the one sign-in row and no more", second)
	}
	if got := string(second.Items[0]["action"]); got != `"sign_in"` {
		t.Errorf("second page row = %s, want the oldest row, the sign-in", got)
	}
}

func TestOperatorLog_RefusesACursorItDidNotWrite(t *testing.T) {
	s := newOpsStack(t, false)

	w := s.list(logPath+"?cursor=nonsense", s.signedInCookie())

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestOperatorLog_ViewingItWritesNoRow(t *testing.T) {
	s := newOpsStack(t, false)
	cookie := s.signedInCookie()
	before := s.count(`SELECT count(*) FROM operator_acts`)

	decodeList(t, s.list(logPath, cookie))

	if after := s.count(`SELECT count(*) FROM operator_acts`); after != before {
		t.Errorf("operator_acts went from %d to %d rows, want no change", before, after)
	}
}

func TestOperatorLog_NeedsASession(t *testing.T) {
	s := newOpsStack(t, false)

	if w := s.list(logPath, ""); w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}
