package staffauth_test

import (
	"database/sql"
	"net/http"
	"strings"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/feedback"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// feedbackURL is POST /api/staff/feedback, the neighboring pre-Practice
// route to PUT /api/staff/work-state: no {practiceId} in the path.
func (e *env) feedbackURL() string { return e.URL("/api/staff/feedback") }

// sendFeedback posts body as the Staff member at the screen and requires
// the 201.
func (e *env) sendFeedback(body string, opts ...reqtest.Option) staffauth.FeedbackResponse {
	e.T.Helper()
	return reqtest.Expect[staffauth.FeedbackResponse](e.T, http.MethodPost, e.feedbackURL(), http.StatusCreated,
		append([]reqtest.Option{e.SignedIn, reqtest.RawJSON([]byte(body))}, opts...)...)
}

// readFeedbackRow reads the one column set every test here checks, as
// the superuser Admin connection -- feedback carries no SELECT grant for
// app_runtime at all (00118), so this is the only way a test can look at
// what was written.
func readFeedbackRow(t *testing.T, db *testdb.DB, id string) (kind, text, browser string, practiceID, roles sql.NullString) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT kind::text, text, browser, practice_id::text, array_to_string(roles, ',')
		   FROM feedback WHERE id = $1`,
		id,
	).Scan(&kind, &text, &browser, &practiceID, &roles); err != nil {
		t.Fatalf("read feedback row %s: %v", id, err)
	}
	return kind, text, browser, practiceID, roles
}

// TestFeedback_Refuses covers what the send is turned away for once the
// request is readable: a kind that is missing or unknown, text over the limit, a
// practiceId that is not a uuid, and (#1523's own AC) a practiceId the
// sender does not hold -- it is checked against her membership, never
// trusted, and nothing is stored for it.
func TestFeedback_Refuses(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
		// detailKey and detail name the field the refusal says was at fault.
		detailKey, detail string
		// practiceID, when set, builds the body's practiceId.
		practiceID func(e *env) string
	}{
		{name: "a missing kind", body: `{"text":"something"}`, want: http.StatusBadRequest,
			detailKey: "kind", detail: staffauth.MsgFeedbackKindNeeded},
		{name: "an unknown kind", body: `{"kind":"a_kind_that_does_not_exist"}`, want: http.StatusBadRequest},
		{name: "text over five thousand characters",
			body: `{"kind":"idea_or_request","text":"` + strings.Repeat("a", feedback.MaxTextRunes+1) + `"}`, want: http.StatusBadRequest,
			detailKey: "text", detail: staffauth.MsgFeedbackTextTooLong},
		{name: "a practice id that is not a uuid", body: `{"kind":"idea_or_request","practiceId":"not-a-uuid"}`, want: http.StatusBadRequest},
		{name: "a Practice she does not hold", body: `{"kind":"idea_or_request","practiceId":"%s"}`, want: http.StatusForbidden,
			practiceID: func(e *env) string {
				return fixture.Practice(e.T, e.DB, fixture.PracticeName("A Different Practice")).ID
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			body := tc.body
			if tc.practiceID != nil {
				body = strings.Replace(body, "%s", tc.practiceID(e), 1)
			}

			out := apierrtest.Expect(t, http.MethodPost, e.feedbackURL(), tc.want, e.SignedIn, reqtest.RawJSON([]byte(body)))

			if tc.detailKey != "" && out.Details[tc.detailKey] != tc.detail {
				t.Fatalf("details[%s] = %q, want %q", tc.detailKey, out.Details[tc.detailKey], tc.detail)
			}
			if n := testdb.Scalar[int](t, e.DB.Admin, `SELECT count(*) FROM feedback`); n != 0 {
				t.Fatalf("feedback rows = %d, want a refusal to store nothing", n)
			}
		})
	}
}

// TestFeedback_EmptyTextFromAccountIsAllowed is #1523's own AC ("Empty
// free text is allowed") plus the /account shape: no practiceId in the
// body leaves practice_id and roles both NULL on the stored row. It also
// proves the browser is parsed from User-Agent and the raw header itself
// never asked for.
func TestFeedback_EmptyTextFromAccountIsAllowed(t *testing.T) {
	e := newEnv(t)

	out := e.sendFeedback(`{"kind":"something_else","text":"","page":{"url":"/account","route":{"id":"/account"}},"appBuild":"abc1234","screenWidth":390}`,
		reqtest.Header("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.6 Safari/605.1.15"))
	if out.ID == "" {
		t.Fatal("response id is empty")
	}

	kind, text, browser, practiceID, roles := readFeedbackRow(t, e.DB, out.ID)
	if kind != "something_else" {
		t.Errorf("kind = %q, want %q", kind, "something_else")
	}
	if text != "" {
		t.Errorf("text = %q, want empty", text)
	}
	if browser != "Safari 17" {
		t.Errorf("browser = %q, want %q -- family and major version only, never the raw header", browser, "Safari 17")
	}
	if practiceID.Valid {
		t.Errorf("practice_id = %q, want NULL -- sent from /account", practiceID.String)
	}
	if roles.Valid {
		t.Errorf("roles = %q, want NULL -- sent from /account", roles.String)
	}
}

// TestFeedback_RecordsPracticeAndHerOwnRoles proves the roles rider: sent
// from under practices/[practiceId], the row carries that Practice and
// the caller's real roles read off practice_memberships -- never a role
// the request itself might have claimed. A real membership with no roles
// assigned yet stores practice_id with roles as an empty array, not NULL
// (the distinction staffRolesAt draws): NULL is reserved for "no
// {practiceId} in the request at all".
func TestFeedback_RecordsPracticeAndHerOwnRoles(t *testing.T) {
	cases := []struct {
		name      string
		staff     []fixture.StaffOption
		wantRoles string
	}{
		{"her roles", []fixture.StaffOption{fixture.Roles(ownerRole, adminRole)}, "owner,admin"},
		{"a membership with no roles", []fixture.StaffOption{fixture.NoRole()}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(tc.staff...))

			out := e.sendFeedback(`{"kind":"not_working","practiceId":"` + e.Practice.ID + `"}`)

			_, _, _, practiceID, roles := readFeedbackRow(t, e.DB, out.ID)
			if !practiceID.Valid || practiceID.String != e.Practice.ID {
				t.Errorf("practice_id = %v, want %q", practiceID, e.Practice.ID)
			}
			if !roles.Valid || roles.String != tc.wantRoles {
				t.Errorf("roles = %v, want %q as a valid array, not NULL", roles, tc.wantRoles)
			}
		})
	}
}

// TestFeedback_QueuesTheIssueOutboxRow is #1524's own AC: both send
// handlers enqueue the outbox row in the same transaction that saves the
// piece of Feedback, so a successful send always leaves exactly one
// pending row behind it.
func TestFeedback_QueuesTheIssueOutboxRow(t *testing.T) {
	e := newEnv(t)

	out := e.sendFeedback(`{"kind":"idea_or_request"}`)

	status := testdb.Scalar[string](t, e.DB.Admin, `SELECT status::text FROM feedback_issue_outbox WHERE feedback_id = $1`, out.ID)
	if status != statusPending {
		t.Errorf("outbox status = %q, want pending", status)
	}
}

// TestFeedback_RefusesTwentyFirstRequestInAnHour is #1523's own sizing:
// 20 per hour, per sender.
func TestFeedback_RefusesTwentyFirstRequestInAnHour(t *testing.T) {
	e := newEnv(t)

	const budget = 20
	for range budget {
		e.sendFeedback(`{"kind":"something_else"}`)
	}

	out := apierrtest.Expect(t, http.MethodPost, e.feedbackURL(), http.StatusTooManyRequests, e.SignedIn, reqtest.RawJSON([]byte(`{"kind":"something_else"}`)))
	if out.Code != apierr.CodeRateLimited {
		t.Errorf("code = %q, want %q", out.Code, apierr.CodeRateLimited)
	}
}
