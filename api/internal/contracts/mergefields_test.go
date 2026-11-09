package contracts_test

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"net/http"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// noMergeFieldProse is Contract prose that holds no {{merge_field}} at
// all -- the template a Practice may write that #1776 found broke the
// Engagement page, because mergeFields marshaled as null.
const noMergeFieldProse = "This agreement has no fields to fill in."

// noFields seeds the Engagement's Contract at status with prose that
// holds no merge field.
func noFields(status string) func(*env) {
	return func(e *env) { seedContract(e.t, e.db, e.Engagement.ID, status, noMergeFieldProse) }
}

// TestContractResponses_NoMergeFieldsMarshalAsEmptyList proves #1776's
// fix on every response that carries mergeFields: a Contract whose prose
// holds no merge field answers with an empty list, never null. Each row
// checks the literal JSON `"mergeFields": []`, not only a decoded empty
// slice, because null decodes to an empty slice too and null is what the
// screens cannot read.
func TestContractResponses_NoMergeFieldsMarshalAsEmptyList(t *testing.T) {
	staffURL := func(path string) func(*env) string {
		return func(e *env) string { return e.contractURL(path) }
	}
	portalURL := func(path string) func(*env) string {
		return func(e *env) string { return e.portalURL(path) }
	}
	reason := func(reason string) func(*env) []reqtest.Option {
		return func(*env) []reqtest.Option {
			return []reqtest.Option{reqtest.JSON(map[string]string{"reason": reason})}
		}
	}
	cases := []struct {
		name    string
		opts    []envOption
		seed    func(*env)
		method  string
		url     func(*env) string
		request func(*env) []reqtest.Option
		want    int
	}{
		{
			name: "create", seed: func(e *env) { e.readyToDraft(noMergeFieldProse) },
			method: http.MethodPost, url: staffURL(""), want: http.StatusCreated,
		},
		{
			name: "staff read", opts: []envOption{signedInAs(fixture.Owner())}, seed: noFields(statusDraft),
			method: http.MethodGet, url: staffURL(""), want: http.StatusOK,
		},
		{
			name: "edit", opts: []envOption{signedInAs(fixture.Owner())}, seed: noFields(statusDraft),
			method: http.MethodPut, url: staffURL(""), want: http.StatusOK,
			request: func(*env) []reqtest.Option { return []reqtest.Option{putting(nil)} },
		},
		{
			name: "send",
			seed: func(e *env) {
				noFields(statusDraft)(e)
				testdb.SeedPendingPortalInvite(e.t, e.db, e.Engagement.Client.ID)
			},
			method: http.MethodPost, url: staffURL("/send"), want: http.StatusOK,
		},
		{
			name: "void", opts: []envOption{signedInAs(fixture.Owner())}, seed: noFields(statusSigned),
			method: http.MethodPost, url: staffURL("/void"), want: http.StatusOK,
		},
		{
			name: "void request", seed: noFields(statusSigned),
			method: http.MethodPost, url: staffURL("/void-request"), request: reason("the plan changed"), want: http.StatusCreated,
		},
		{
			name: "void request decline", opts: []envOption{signedInAs(fixture.Admin())},
			seed: func(e *env) {
				noFields(statusSigned)(e)
				seedVoidRequest(e.t, e.db, e.Engagement.ID, e.Staff.ID, "asking")
			},
			method: http.MethodPost, request: reason("not yet"), want: http.StatusOK,
			url: func(e *env) string {
				requestID := testdb.Scalar[string](e.t, e.db.Admin, `SELECT id FROM contract_void_requests WHERE engagement_id = $1`, e.Engagement.ID)
				return e.contractURL("/void-request/" + requestID + "/decline")
			},
		},
		{
			name: "client read", opts: []envOption{withPortalLogin()}, seed: noFields(statusSent),
			method: http.MethodGet, url: portalURL(""), want: http.StatusOK,
		},
		{
			name: "client sign", opts: []envOption{withPortalLogin()}, seed: noFields(statusSent),
			method: http.MethodPost, url: portalURL("/sign"), want: http.StatusOK,
			request: func(*env) []reqtest.Option { return []reqtest.Option{signing("Jordan Client", true)} },
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t, c.opts...)
			c.seed(e)
			session := e.signedIn
			if e.asClient != nil {
				session = e.asClient
			}
			opts := []reqtest.Option{session}
			if c.request != nil {
				opts = append(opts, c.request(e)...)
			}

			body := reqtest.ExpectBody(t, c.method, c.url(e), c.want, opts...)

			var raw map[string]jsontext.Value
			if err := json.Unmarshal(body, &raw); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if got := string(raw["mergeFields"]); got != "[]" {
				t.Fatalf("mergeFields = %s, want [] for prose with no merge fields (#1776)", got)
			}
		})
	}
}
