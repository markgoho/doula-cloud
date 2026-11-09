package mailsuppress_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/mail"
	"doula-cloud/api/internal/mailsuppress"
	"doula-cloud/api/internal/reqtest"
)

func TestListHandler_ShowsThisPracticesSuppressedAddresses(t *testing.T) {
	e := newEnv(t, &mail.FakeSender{})
	clientAt(t, e.DB, e.Practice, "bounced@example.test")
	clientAt(t, e.DB, e.Practice, "spammed@example.test")
	suppress(t, e.DB, "bounced@example.test", mailsuppress.CauseBounce)
	suppress(t, e.DB, "spammed@example.test", mailsuppress.CauseComplaint)

	got := reqtest.Expect[struct {
		Suppressions []struct {
			Address   string `json:"address"`
			Cause     string `json:"cause"`
			Clearable bool   `json:"clearable"`
		} `json:"suppressions"`
	}](t, http.MethodGet, e.listURL(), http.StatusOK, e.SignedIn)

	if len(got.Suppressions) != 2 {
		t.Fatalf("listed %d suppressions, want 2", len(got.Suppressions))
	}
	// Clearable is the screen's affordance and ADR-0029's rule in one
	// field: a complaint row is shown, so Staff know why the mail stops,
	// but it offers nothing to press.
	for _, s := range got.Suppressions {
		if s.Cause == mailsuppress.CauseBounce && !s.Clearable {
			t.Fatal("a bounce is not offered as clearable")
		}
		if s.Cause == mailsuppress.CauseComplaint && s.Clearable {
			t.Fatal("a complaint is offered as clearable")
		}
	}
}

func TestClearHandler_ClearsABounce(t *testing.T) {
	e := newEnv(t, &mail.FakeSender{}, fixture.Admin())
	clientAt(t, e.DB, e.Practice, testAddress)
	suppress(t, e.DB, testAddress, mailsuppress.CauseBounce)

	e.clear(testAddress, http.StatusNoContent)

	if len(e.clearer.Deleted()) != 1 {
		t.Fatal("the clear never reached Mailgun's own bounce list")
	}
	requireActive(t, e.DB, testAddress, false)
}

// TestClearHandler_Refusals table-drives what the endpoint refuses, and
// that each refusal leaves the suppression standing and never reaches
// Mailgun's own list.
//   - ADR-0029's rule has to hold at the endpoint, not only in the screen:
//     the shared domain's reputation is what a second complaint costs.
//   - email_suppressions carries no practice_id and no RLS policy, so the
//     handler is the only thing standing between one Practice and
//     another's addresses; it answers 404, not 403, so a Practice does not
//     learn that another Practice's Client is suppressed.
//   - A Doula is neither Owner nor Admin: ADR-0008 keeps the roster's own
//     addresses in the same hands as the roster.
//   - Mailgun is the side that can refuse, and the local row must survive
//     it unchanged.
func TestClearHandler_Refusals(t *testing.T) {
	cases := []struct {
		name       string
		staff      fixture.StaffOption
		cause      string
		otherOwns  bool
		deleteErr  error
		want       int
		wantActive bool
	}{
		{"a complaint", fixture.Owner(), mailsuppress.CauseComplaint, false, nil, http.StatusConflict, true},
		{"another Practice's address", fixture.Owner(), mailsuppress.CauseBounce, true, nil, http.StatusNotFound, true},
		{"an address nobody suppressed", fixture.Owner(), "", false, nil, http.StatusNotFound, false},
		{"a Doula", fixture.Doula(), mailsuppress.CauseBounce, false, nil, http.StatusForbidden, true},
		{"a Mailgun failure", fixture.Owner(), mailsuppress.CauseBounce, false, errBoom, http.StatusBadGateway, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, &mail.FakeSender{DeleteErr: tc.deleteErr}, tc.staff)
			owner := e.Practice
			if tc.otherOwns {
				owner = fixture.Practice(t, e.DB, fixture.PracticeName("Theirs"))
			}
			clientAt(t, e.DB, owner, testAddress)
			if tc.cause != "" {
				suppress(t, e.DB, testAddress, tc.cause)
			}

			e.clear(testAddress, tc.want)

			if len(e.clearer.Deleted()) != 0 {
				t.Fatal("a refused clear still reached Mailgun")
			}
			requireActive(t, e.DB, testAddress, tc.wantActive)
		})
	}
}

func TestClearHandler_RejectsAMalformedBody(t *testing.T) {
	e := newEnv(t, &mail.FakeSender{})

	for _, tc := range []struct {
		name string
		body reqtest.Option
	}{
		{"not JSON", reqtest.RawJSON([]byte("{"))},
		{"no address", clearBody("")},
		{"blank address", clearBody("   ")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reqtest.ExpectStatus(t, http.MethodPost, e.listURL()+"/clear", http.StatusBadRequest, e.SignedIn, tc.body)
		})
	}
}
