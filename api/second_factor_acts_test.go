package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// secondFactorActCase is one of the five acts #1532 names (ADR-0026's
// amendment for #1492), as a request the real route table answers.
type secondFactorActCase struct {
	act    string
	method string
	// path builds the request path from the seeded Practice, Client and
	// Staff member.
	path func(practiceID, clientID, staffID string) string
	body any
}

var secondFactorActCases = []secondFactorActCase{
	{
		act: "practice_export", method: http.MethodGet,
		path: func(p, _, _ string) string { return "/api/practices/" + p + "/export" },
	},
	{
		act: "practice_deletion", method: http.MethodPost,
		path: func(p, _, _ string) string { return "/api/practices/" + p + "/deletion" },
	},
	{
		act: "client_erasure", method: http.MethodPost,
		path: func(p, c, _ string) string { return "/api/practices/" + p + "/clients/" + c + "/erasure" },
	},
	{
		act: "staff_vouch", method: http.MethodPost,
		path: func(p, _, s string) string { return "/api/practices/" + p + "/staff/" + s + "/mfa-recovery/vouch" },
	},
	{
		act: "mfa_required_switch", method: http.MethodPut,
		path: func(p, _, _ string) string { return "/api/practices/" + p + "/mfa-required" },
		body: map[string]bool{"required": true},
	},
}

// TestSecondFactorActs_RefusedWithoutASecondFactor is #1532's first and
// fifth acceptance boxes, driven through the real route table: each of
// the five acts refuses an Owner whose session shows no second factor
// with SECOND_FACTOR_REQUIRED, and records who tried and when. The same
// five, asked by the same Owner with a second factor, are not refused
// by the seam.
//
// Today the Practice boundary still refuses an Owner with no second
// factor on every other route (#1531 removes that); these five reach
// the act's own refusal instead, so this spec holds before and after
// #1531 lands.
func TestSecondFactorActs_RefusedWithoutASecondFactor(t *testing.T) {
	for _, tc := range secondFactorActCases {
		for _, secondFactor := range []bool{false, true} {
			name := tc.act + "/without a second factor"
			if secondFactor {
				name = tc.act + "/with a second factor"
			}
			t.Run(name, func(t *testing.T) {
				db := testdb.New(t)
				const ownerUID = "owner-second-factor-act"
				practiceID, ownerID := testdb.SeedStaffAtNewPractice(t, db, ownerUID, []string{"owner"}, "employee")
				clientID := testdb.SeedNamedClient(t, db, practiceID, "Sarah", "sarah@example.test")
				memberID := testdb.SeedStaffAtPractice(t, db, practiceID, "member-lost-factor", []string{roleDoula}, "employee")

				deps := testDeps()
				deps.DB = db.App
				deps.Verifier = authntest.Verifier{UID: ownerUID, AuthTime: time.Now()} //nolint:forbidigo // #773: a test's recent sign-in, not a production request path
				mux, _, _ := routes(deps)
				srv := httptest.NewServer(mux)
				defer srv.Close()

				session := authntest.SeedSessionWithSecondFactor(t, db.App, ownerUID, secondFactor)
				resp := doSecondFactorAct(t, srv, session, tc, tc.path(practiceID, clientID, memberID))
				defer resp.Body.Close()

				refusals := countSecondFactorRefusals(t, db, practiceID, ownerID, tc.act)
				if secondFactor {
					if resp.StatusCode == http.StatusForbidden {
						t.Fatalf("status = 403 (%v) with a second factor, want the act to go past the seam", apierrtest.Decode(t, resp))
					}
					if refusals != 0 {
						t.Fatalf("refusals recorded = %d with a second factor, want 0", refusals)
					}
					return
				}
				if resp.StatusCode != http.StatusForbidden {
					t.Fatalf("status = %d, want 403", resp.StatusCode)
				}
				if got := apierrtest.Decode(t, resp).Code; got != apierr.CodeSecondFactorRequired {
					t.Fatalf("code = %q, want %q", got, apierr.CodeSecondFactorRequired)
				}
				if refusals != 1 {
					t.Fatalf("refusals recorded = %d, want 1", refusals)
				}
			})
		}
	}
}

func doSecondFactorAct(t *testing.T, srv *httptest.Server, session string, tc secondFactorActCase, path string) *http.Response {
	t.Helper()
	var body bytes.Buffer
	if tc.body != nil {
		if err := json.NewEncoder(&body).Encode(tc.body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req, err := http.NewRequestWithContext(t.Context(), tc.method, srv.URL+path, &body)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	authntest.AddSessionCookie(req, session)
	req.Header.Set("X-Confirmed", "true")
	req.Header.Set("Idempotency-Key", "second-factor-act-"+tc.act)
	req.Header.Set("Authorization", "Bearer recent-sign-in")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	return resp
}

// countSecondFactorRefusals counts the activity rows that record ownerID
// being refused act at practiceID -- the "who tried and when" of #1532's
// fifth box. The row's own created_at is the when.
func countSecondFactorRefusals(t *testing.T, db *testdb.DB, practiceID, ownerID, act string) int {
	t.Helper()
	var n int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM activity
		 WHERE practice_id = $1 AND subject_kind = 'practice' AND subject_id = $1
		   AND action = $2 AND actor_kind = 'staff' AND actor_staff_id = $3
		   AND diff->>'act' = $4`,
		practiceID, staffauth.ActionSecondFactorRefused, ownerID, act,
	).Scan(&n); err != nil {
		t.Fatalf("count refusals: %v", err)
	}
	return n
}

// TestSecondFactorActs_EveryDeclaredActIsMounted holds the seam's
// declaration and the route table together (#1532's second box): every
// act staffauth.SecondFactorActs names is a route the binary mounts
// behind the seam, and every route mounted behind the seam is one of
// them. A sixth act is one line in that map, and this spec is what
// catches a line naming a route that does not exist.
func TestSecondFactorActs_EveryDeclaredActIsMounted(t *testing.T) {
	_, gRoutes, _ := routes(testDeps())
	mounted := map[string]string{}
	for _, route := range gRoutes {
		if route.SecondFactorAct != "" {
			mounted[route.Method+" "+route.Pattern] = route.SecondFactorAct
		}
	}
	for pattern, act := range staffauth.SecondFactorActs {
		if mounted[pattern] != act {
			t.Errorf("SecondFactorActs names %q as %q, but the route table mounts it as %q", pattern, act, mounted[pattern])
		}
	}
	if len(mounted) != len(staffauth.SecondFactorActs) {
		t.Errorf("route table mounts %d acts behind the seam, SecondFactorActs names %d", len(mounted), len(staffauth.SecondFactorActs))
	}
	if len(staffauth.SecondFactorActs) != len(secondFactorActCases) {
		t.Errorf("SecondFactorActs names %d acts, the spec above drives %d -- add the new act to secondFactorActCases", len(staffauth.SecondFactorActs), len(secondFactorActCases))
	}
}
