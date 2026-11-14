// Package exporttest is what the tests of the Staff CSV downloads share
// (#2871, #2891): reading the file and the Activity rows they left, and the
// three promises every download makes, who may ask, how often, and what its
// one Activity row says.
package exporttest

import (
	"encoding/csv"
	"encoding/json/v2"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// Read is Rows of the response send answers, closing its body.
func Read(t *testing.T, send func() *http.Response) [][]string {
	t.Helper()
	resp := send()
	defer func() { _ = resp.Body.Close() }()
	return Rows(t, resp)
}

// Rows is the CSV a download answered with, header first, requiring a 200.
func Rows(t *testing.T, resp *http.Response) [][]string {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		// coverage:ignore reason: a test helper's own failure path, reached only when the download is wrong
		t.Fatalf("export status = %d, want 200", resp.StatusCode)
	}
	rows, err := csv.NewReader(resp.Body).ReadAll()
	if err != nil {
		// coverage:ignore reason: a test helper's own failure path, reached only when the download is wrong
		t.Fatalf("read export: %v", err)
	}
	return rows
}

// Diffs is the diff of each row of action the Practice holds, oldest first.
func Diffs(t *testing.T, db *testdb.DB, practiceID, action string) []string {
	t.Helper()
	rows, err := db.Admin.QueryContext(t.Context(), `SELECT diff::text FROM activity WHERE practice_id = $1 AND action = $2 ORDER BY created_at`, practiceID, action)
	if err != nil {
		// coverage:ignore reason: a test helper's own failure path, reached only when the download is wrong
		t.Fatalf("read export activity: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var diffs []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			// coverage:ignore reason: a test helper's own failure path, reached only when the download is wrong
			t.Fatalf("scan export activity: %v", err)
		}
		diffs = append(diffs, d)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: a test helper's own failure path, reached only when the download is wrong
		t.Fatalf("read export activity: %v", err)
	}
	return diffs
}

// RequireRecorded requires the one Activity row of action to name the
// Practice as its subject, the Staff member as its actor and at as its time,
// and its diff to be exactly want and to hold none of the words in never.
func RequireRecorded(t *testing.T, db *testdb.DB, practiceID, staffID, action string, at time.Time, want map[string]any, never ...string) {
	t.Helper()
	var subjectKind, subjectID, actorStaff, diff string
	var actorIsStaff bool
	var created time.Time
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT subject_kind, subject_id::text, (actor_kind = 'staff'), actor_staff_id::text, created_at, diff::text
		   FROM activity WHERE practice_id = $1 AND action = $2`, practiceID, action,
	).Scan(&subjectKind, &subjectID, &actorIsStaff, &actorStaff, &created, &diff); err != nil {
		// coverage:ignore reason: a test helper's own failure path, reached only when the download is wrong
		t.Fatalf("read the export's Activity row: %v", err)
	}
	if subjectKind != "practice" || subjectID != practiceID || !actorIsStaff || actorStaff != staffID || !created.Equal(at) {
		// coverage:ignore reason: a test helper's own failure path, reached only when the download is wrong
		t.Fatalf("row = %s %s by %s at %s, want the Practice by the Staff member at %s", subjectKind, subjectID, actorStaff, created, at)
	}
	var scope map[string]any
	if err := json.Unmarshal([]byte(diff), &scope); err != nil {
		// coverage:ignore reason: a test helper's own failure path, reached only when the download is wrong
		t.Fatalf("diff %s: %v", diff, err)
	}
	if len(scope) != len(want) {
		// coverage:ignore reason: a test helper's own failure path, reached only when the download is wrong
		t.Fatalf("diff = %s, want exactly %v", diff, want)
	}
	for k, v := range want {
		if scope[k] != v {
			// coverage:ignore reason: a test helper's own failure path, reached only when the download is wrong
			t.Fatalf("diff[%s] = %v, want %v (diff %s)", k, scope[k], v, diff)
		}
	}
	for _, word := range never {
		if strings.Contains(strings.ToLower(diff), word) {
			// coverage:ignore reason: a test helper's own failure path, reached only when the download is wrong
			t.Fatalf("diff %s holds %q", diff, word)
		}
	}
}

// RequireOwnerAndAdminOnly requires that an Owner and an Admin are answered
// 200 and a Doula and a contractor Doula 403, and that only the 200s left a
// row. send signs in as the role, sends one request and answers its status
// and how many Activity rows the Practice then holds.
func RequireOwnerAndAdminOnly(t *testing.T, send func(t *testing.T, role fixture.RoleCase) (status, rows int)) {
	t.Helper()
	cases := []struct {
		role fixture.RoleCase
		want int
	}{
		{fixture.OwnerCase, http.StatusOK},
		{fixture.AdminCase, http.StatusOK},
		{fixture.EmployeeDoulaCase, http.StatusForbidden},
		{fixture.ContractorDoulaCase, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.role.Name, func(t *testing.T) {
			status, rows := send(t, tc.role)
			wantRows := 0
			if tc.want == http.StatusOK {
				wantRows = 1
			}
			if status != tc.want || rows != wantRows {
				// coverage:ignore reason: a test helper's own failure path, reached only when the download is wrong
				t.Fatalf("status = %d with %d Activity rows, want %d with %d", status, rows, tc.want, wantRows)
			}
		})
	}
}

// RequireRateLimited requires 30 downloads in an hour to be answered and the
// 31st to be a 429 with the standard headers, and not recorded. rows is how
// many Activity rows the Practice holds.
func RequireRateLimited(t *testing.T, send func() *http.Response, rows func() int) {
	t.Helper()
	for range 30 {
		resp := send()
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			// coverage:ignore reason: a test helper's own failure path, reached only when the download is wrong
			t.Fatalf("status = %d, want 200 inside the limit", resp.StatusCode)
		}
	}

	resp := send()
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusTooManyRequests {
		// coverage:ignore reason: a test helper's own failure path, reached only when the download is wrong
		t.Fatalf("31st status = %d, want 429", resp.StatusCode)
	}
	if resp.Header.Get("RateLimit-Limit") != "30" || resp.Header.Get("RateLimit-Remaining") != "0" {
		// coverage:ignore reason: a test helper's own failure path, reached only when the download is wrong
		t.Fatalf("headers = %v, want RateLimit-Limit 30 and RateLimit-Remaining 0", resp.Header)
	}
	if secs, err := strconv.Atoi(resp.Header.Get("Retry-After")); err != nil || secs <= 0 {
		// coverage:ignore reason: a test helper's own failure path, reached only when the download is wrong
		t.Fatalf("Retry-After = %q, want seconds to wait", resp.Header.Get("Retry-After"))
	}
	if body := reqtest.Decode[apierr.APIError](t, resp); body.Code != apierr.CodeRateLimited {
		// coverage:ignore reason: a test helper's own failure path, reached only when the download is wrong
		t.Fatalf("body = %+v, want RATE_LIMITED", body)
	}
	if got := rows(); got != 30 {
		// coverage:ignore reason: a test helper's own failure path, reached only when the download is wrong
		t.Fatalf("activity rows = %d, want the 30 that were served", got)
	}
}

// RequireFile requires the response to be a CSV named name, counting its
// rows in header countHeader, with the first rate-limit headers of a heavy
// endpoint.
func RequireFile(t *testing.T, resp *http.Response, name, countHeader, count string) {
	t.Helper()
	if got := resp.Header.Get("Content-Type"); got != "text/csv; charset=utf-8" {
		// coverage:ignore reason: a test helper's own failure path, reached only when the download is wrong
		t.Fatalf("Content-Type = %q", got)
	}
	if got := resp.Header.Get("Content-Disposition"); got != `attachment; filename="`+name+`"` {
		// coverage:ignore reason: a test helper's own failure path, reached only when the download is wrong
		t.Fatalf("Content-Disposition = %q", got)
	}
	if got := resp.Header.Get(countHeader); got != count {
		// coverage:ignore reason: a test helper's own failure path, reached only when the download is wrong
		t.Fatalf("%s = %q, want %s", countHeader, got, count)
	}
	if resp.Header.Get("RateLimit-Limit") != "30" || resp.Header.Get("RateLimit-Remaining") != "29" {
		// coverage:ignore reason: a test helper's own failure path, reached only when the download is wrong
		t.Fatalf("headers = %v, want RateLimit-Limit 30 and RateLimit-Remaining 29", resp.Header)
	}
}
