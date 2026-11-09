package engagement_test

import (
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/oncall"
	"doula-cloud/api/internal/portal"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
	"doula-cloud/api/internal/testdb"
)

// The fixture calendar, named once. A birth due on dueBefore has a
// 37-week, 14-day-grace window of 2026-10-09..2026-11-13; moved to
// dueAfter, the same rule draws 2026-10-30..2026-12-04 (the same
// arithmetic oncall's own TestRoster_WindowIsDerivedAndMovesWithTheDueDate
// pins).
const dueDateField = "dueDate"

const (
	dueBefore = "2026-10-30"
	dueAfter  = "2026-11-20"
)

// dueDateResponseBody decodes both shapes this endpoint answers with: the
// success DTO, and apierr's own envelope's Code and Details.
type dueDateResponseBody struct {
	EngagementID string            `json:"engagementId"`
	DueDate      string            `json:"dueDate"`
	Code         apierr.Code       `json:"code"`
	Details      map[string]string `json:"details"`
}

// putDueDate PUTs raw as the body of a due-date change for engagementID
// as the Staff member at the screen and requires want.
func (e *env) putDueDate(engagementID string, raw []byte, want int) dueDateResponseBody {
	e.T.Helper()
	return reqtest.Expect[dueDateResponseBody](e.T, http.MethodPut, e.urlOf(engagementID, "/due-date"), want, e.SignedIn, reqtest.RawJSON(raw))
}

// changeDueDate is putDueDate on the Engagement with a well-formed body.
func (e *env) changeDueDate(dueDate string, want int) dueDateResponseBody {
	e.T.Helper()
	encoded, err := json.Marshal(map[string]any{dueDateField: dueDate})
	if err != nil {
		e.T.Fatalf("marshal body: %v", err)
	}
	return e.putDueDate(e.Engagement.ID, encoded, want)
}

// storedDueDate is the Engagement's raw column, "" for null.
func (e *env) storedDueDate() string {
	e.T.Helper()
	return readEngagementDueDate(e.T, e.DB, e.Engagement.ID)
}

// readEngagementDueDate is the raw column, bypassing the API; "" for null.
func readEngagementDueDate(t *testing.T, db *testdb.DB, engagementID string) string {
	t.Helper()
	var due *string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT due_date::text FROM engagements WHERE id = $1`, engagementID,
	).Scan(&due); err != nil {
		t.Fatalf("read engagement due date: %v", err)
	}
	if due == nil {
		return ""
	}
	return *due
}

// TestChangeDueDateHandler_RolesMayChange is ADR-0015's role table for
// the Engagement's mutable facts, read the same way kind's own test
// reads it (refuseFactWrite): an Owner, an Admin and an employee Doula
// may change the due date; a contractor Doula and a member with no role
// are refused at the endpoint, and nothing is written for them. Every
// allowed change leaves one Activity row naming who changed it, from
// what and to what (readFactDiff fails on a row with no actor; the row's
// created_at is the when).
func TestChangeDueDateHandler_RolesMayChange(t *testing.T) {
	eachFactWriter(t, func(t *testing.T, e *env, wantOK bool) {
		body := e.changeDueDate(dueAfter, okOrForbidden(wantOK))

		if !wantOK {
			if got := e.storedDueDate(); got != dueBefore {
				t.Fatalf("due date = %q, want left unchanged at %s", got, dueBefore)
			}
			if n := e.countActivity("due_date_changed"); n != 0 {
				t.Fatalf("due_date_changed entries = %d, want 0", n)
			}
			return
		}
		if body.DueDate != dueAfter || body.EngagementID != e.Engagement.ID {
			t.Fatalf("body = %+v, want %s on %s", body, dueAfter, e.Engagement.ID)
		}
		if got := e.storedDueDate(); got != dueAfter {
			t.Fatalf("due date = %q, want %s", got, dueAfter)
		}
		if n := e.countActivity("due_date_changed"); n != 1 {
			t.Fatalf("due_date_changed entries = %d, want 1", n)
		}
		assertDiff(t, readFactDiff(t, e.DB, e.Engagement.ID, "due_date_changed"), map[string]any{
			"dueDateBefore": dueBefore,
			"dueDateAfter":  dueAfter,
		})
	}, withEngagement(fixture.DueDate(dueBefore)))
}

// TestChangeDueDateHandler_FirstDueDate covers the nullable column: an
// Engagement with no due date yet is given one, and the Activity row
// records the before side as JSON null rather than leaving it out.
func TestChangeDueDateHandler_FirstDueDate(t *testing.T) {
	e := newOwnerEnv(t)

	body := e.changeDueDate(dueAfter, http.StatusOK)

	if body.DueDate != dueAfter {
		t.Fatalf("body = %+v, want %s", body, dueAfter)
	}
	assertDiff(t, readFactDiff(t, e.DB, e.Engagement.ID, "due_date_changed"), map[string]any{
		"dueDateBefore": nil,
		"dueDateAfter":  dueAfter,
	})
}

// TestChangeDueDateHandler_NoOp proves the PUT is naturally idempotent:
// re-sending the date the Engagement already holds writes nothing and
// raises no second Activity row (docs/api-design.md rule 4).
func TestChangeDueDateHandler_NoOp(t *testing.T) {
	e := newOwnerEnv(t, withEngagement(fixture.DueDate(dueBefore)))

	for range 2 {
		if body := e.changeDueDate(dueAfter, http.StatusOK); body.DueDate != dueAfter {
			t.Fatalf("body = %+v", body)
		}
	}
	if n := e.countActivity("due_date_changed"); n != 1 {
		t.Fatalf("due_date_changed entries = %d, want 1 -- the resend writes nothing", n)
	}

	e.changeDueDate(dueBefore, http.StatusOK)

	if n := e.countActivity("due_date_changed"); n != 2 {
		t.Fatalf("due_date_changed entries = %d, want 2", n)
	}
}

// TestChangeDueDateHandler_Validation proves a due date is a calendar
// day, YYYY-MM-DD, and nothing else: an empty value (this endpoint sets a
// date, it never clears one), an impossible day and free text are each
// refused with a 400 naming the dueDate field, and none writes a thing.
func TestChangeDueDateHandler_Validation(t *testing.T) {
	cases := []struct{ name, dueDate string }{
		{"empty", "   "},
		{"impossible day", "2027-02-30"},
		{"free text", "next week"},
		{"an instant", "2027-03-01T00:00:00Z"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newOwnerEnv(t, withEngagement(fixture.DueDate(dueBefore)))

			body := e.changeDueDate(tc.dueDate, http.StatusBadRequest)

			if body.Code != apierr.CodeInvalidArgument || body.Details["dueDate"] != engagement.MsgDueDateMalformed {
				t.Fatalf("body = %+v, want %s naming dueDate", body, apierr.CodeInvalidArgument)
			}
			if got := e.storedDueDate(); got != dueBefore {
				t.Fatalf("due date = %q, want left unchanged", got)
			}
			if n := e.countActivity("due_date_changed"); n != 0 {
				t.Fatalf("due_date_changed entries = %d, want 0", n)
			}
		})
	}
}

// TestChangeDueDateHandler_RefusesARequestItCannotRead covers the doors a
// request can fail at before any date is weighed.
func TestChangeDueDateHandler_RefusesARequestItCannotRead(t *testing.T) {
	e := newOwnerEnv(t, withEngagement(fixture.DueDate(dueBefore)))

	e.refusesAnUnreadableRequest(t, http.MethodPut, "/due-date", reqtest.JSON(map[string]any{dueDateField: dueAfter}))
}

// newCrossReadServer mounts this package beside the two other readers of
// the due date -- oncall (the On-call window) and portal (the Client's
// own read) -- on one mux, the way main.go mounts them together.
func newCrossReadServer(t *testing.T, db *testdb.DB) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	engagement.Mount(g, ir)
	oncall.Mount(g, ir, &tasknudge.FakeEnqueuer{})
	portal.Mount(g, db.App)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// getAs reads url as session into T, failing on any status but 200.
func getAs[T any](t *testing.T, session, url string) T {
	t.Helper()
	return reqtest.Expect[T](t, http.MethodGet, url, http.StatusOK, reqtest.Session(session))
}

// TestChangeDueDateHandler_EveryReaderFollows is #2013's third AC: what
// reads the due date reads the new one on its next read, through its own
// endpoint -- the Staff Engagement read behind the summary card, the
// On-call window (derived, never stored, so it moves with no write of
// its own), and the Client's portal read of her care.
func TestChangeDueDateHandler_EveryReaderFollows(t *testing.T) {
	e := newOwnerEnv(t)
	doula := e.Colleague()
	client := fixture.Client(t, e.DB, e.Practice, fixture.WithPortalLogin())
	booked := fixture.Engagement(t, e.DB, e.Practice, fixture.ForClient(client), fixture.DueDate(dueBefore), fixture.WithDoula(doula))
	srv := newCrossReadServer(t, e.DB)
	staffSession := fixture.Session(t, e.DB, e.Staff)
	clientSession := fixture.Session(t, e.DB, client)
	engagementURL := srv.URL + "/api/practices/" + e.Practice.ID + "/engagements/" + booked.ID

	before := getAs[oncall.EngagementOnCall](t, staffSession, engagementURL+"/on-call")
	if before.Window == nil || *before.Window != (oncall.Window{Start: "2026-10-09", End: "2026-11-13"}) {
		t.Fatalf("window before = %+v, want 2026-10-09..2026-11-13", before.Window)
	}

	reqtest.ExpectStatus(t, http.MethodPut, engagementURL+"/due-date", http.StatusOK, reqtest.Session(staffSession),
		reqtest.JSON(map[string]any{dueDateField: dueAfter}))

	if d := getAs[engagement.Detail](t, staffSession, engagementURL); d.DueDate == nil || *d.DueDate != dueAfter {
		t.Fatalf("staff detail dueDate = %v, want %s", d.DueDate, dueAfter)
	}
	after := getAs[oncall.EngagementOnCall](t, staffSession, engagementURL+"/on-call")
	if after.Window == nil || *after.Window != (oncall.Window{Start: "2026-10-30", End: "2026-12-04"}) {
		t.Fatalf("window after = %+v, want 2026-10-30..2026-12-04", after.Window)
	}
	if p := getAs[portal.Detail](t, clientSession, srv.URL+"/api/portal/engagements/"+booked.ID); p.DueDate == nil || *p.DueDate != dueAfter {
		t.Fatalf("portal dueDate = %v, want %s", p.DueDate, dueAfter)
	}
}
