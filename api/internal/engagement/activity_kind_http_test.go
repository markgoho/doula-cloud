package engagement_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// #2755: ?kind= narrows the ledger to care, money or doulas, and a
// contractor's money gate still applies under it.
func TestListActivityHandler_KindNarrowsTheLedger(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()), withEngagement(fixture.Status(fixture.StatusActive)))
	e.seedActivity(activity.ActionVisitLogged, activity.StaffActor(e.Staff.ID))
	e.seedActivity(activity.ActionContractSigned, activity.StaffActor(e.Staff.ID))
	e.seedActivity(activity.ActionContractPriced, activity.StaffActor(e.Staff.ID))
	e.seedActivity(activity.ActionInvoiceRaised, activity.StaffActor(e.Staff.ID))
	e.seedActivity(activity.ActionDoulaAttached, activity.StaffActor(e.Staff.ID))

	read := func(who fixture.StaffRecord, kind string) map[string]bool {
		seen := map[string]bool{}
		resp := reqtest.Expect[engagement.ActivityListResponse](t, http.MethodGet, e.engagementURL("/activity?kind="+kind), http.StatusOK, e.As(who))
		for _, item := range resp.Items {
			seen[item.Action] = true
		}
		return seen
	}

	if got := read(e.Staff, "care"); len(got) != 1 || !got["visit_logged"] {
		t.Errorf("care = %v, want only the Visit", got)
	}
	if got := read(e.Staff, "money"); len(got) != 3 || !got["contract_signed"] || !got["contract_priced"] || !got["invoice_raised"] {
		t.Errorf("money = %v, want the Contract and Invoice entries", got)
	}
	if got := read(e.Staff, "doulas"); len(got) != 1 || !got["doula_attached"] {
		t.Errorf("doulas = %v, want only the attachment", got)
	}

	lena := e.Colleague(fixture.Contractor())
	testdb.SeedGrantedAttachment(t, e.DB, e.Engagement.ID, lena.ID)
	if got := read(lena, "money"); len(got) != 1 || !got["contract_signed"] {
		t.Errorf("a contractor's money = %v, want the Contract entry alone", got)
	}
}

func TestListActivityHandler_RefusesAnUnknownKind(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))

	got := reqtest.Expect[apierr.APIError](t, http.MethodGet, e.engagementURL("/activity?kind=everything"), http.StatusBadRequest, e.SignedIn)

	if got.Details["kind"] == "" {
		t.Errorf("details = %v, want a sentence under kind", got.Details)
	}
}
