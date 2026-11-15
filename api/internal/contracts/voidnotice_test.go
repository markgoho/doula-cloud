package contracts_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

const kindVoidRequested = "void_requested"

// notice is one pending staff_notice_outbox row as the tests below read it:
// kind, recipient, subject and the person who caused it.
func notice(kind, recipient, subject, triggeredBy string) string {
	return strings.Join([]string{kind, recipient, subject, triggeredBy}, " ")
}

// expectQueued requires the pending Staff notices to be exactly want.
func expectQueued(t *testing.T, e *env, want ...string) {
	t.Helper()
	var joined string
	if err := e.DB.Admin.QueryRowContext(t.Context(),
		`SELECT COALESCE(string_agg(kind::text || ' ' || recipient_staff_id || ' ' || subject_id || ' ' || triggered_by_staff_id,
		                            ',' ORDER BY kind::text, recipient_staff_id::text COLLATE "C"), '')
		   FROM staff_notice_outbox WHERE status = 'pending'`).Scan(&joined); err != nil {
		t.Fatalf("read staff_notice_outbox: %v", err)
	}
	slices.Sort(want)
	if got := joined; got != strings.Join(want, ",") {
		t.Fatalf("queued = %q, want %q", got, strings.Join(want, ","))
	}
}

// #2953: a Doula asking for a void tells each Owner and Admin, in the
// request's own transaction, and nobody else.
func TestPostVoidRequestHandler_TellsEachOwnerAndAdmin(t *testing.T) {
	e := newEnv(t)
	e.seedContract(statusSigned)
	owner := e.Colleague(fixture.Owner())
	admin := e.Colleague(fixture.Admin())
	e.Colleague(fixture.Doula())

	out := e.requestVoid("the client's plan changed", http.StatusCreated)

	id := out.VoidRequests[0].ID
	expectQueued(t, e,
		notice(kindVoidRequested, owner.ID, id, e.Staff.ID),
		notice(kindVoidRequested, admin.ID, id, e.Staff.ID))
}

func TestPostVoidRequestHandler_DoesNotTellTheOwnerWhoAsked(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	e.seedContract(statusSigned)
	other := e.Colleague(fixture.Owner())

	out := e.requestVoid("asking myself", http.StatusCreated)

	expectQueued(t, e, notice(kindVoidRequested, other.ID, out.VoidRequests[0].ID, e.Staff.ID))
}

func TestPostVoidRequestHandler_AContractorOnAGrantedAttachmentTellsThem(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Contractor()))
	e.seedContract(statusSigned)
	testdb.SeedGrantedAttachment(e.T, e.DB, e.Engagement.ID, e.Staff.ID)
	owner := e.Colleague(fixture.Owner())

	out := e.requestVoid("asking as a contractor", http.StatusCreated)

	expectQueued(t, e, notice(kindVoidRequested, owner.ID, out.VoidRequests[0].ID, e.Staff.ID))
}

func TestPostVoidRequestHandler_ARefusedRequestTellsNobody(t *testing.T) {
	e := newEnv(t)
	e.seedContract(statusSigned)
	e.Colleague(fixture.Owner())

	reqtest.ExpectStatus(t, http.MethodPost, e.voidRequestURL(), http.StatusBadRequest, e.SignedIn, reasoned(""))

	expectQueued(t, e)
}

// #2953: a decline tells the Doula who asked, not the one who declined,
// and not another Doula whose request is still open.
func TestPostVoidRequestDeclineHandler_TellsTheDoulaWhoAsked(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Admin()))
	e.seedContract(statusSigned)
	asker := e.Colleague(fixture.Doula())
	other := e.Colleague(fixture.Doula())
	requestID := e.seedVoidRequestBy(asker, "asking")
	e.seedVoidRequestBy(other, "also asking")

	e.decline(requestID, "not yet")

	expectQueued(t, e, notice("void_declined", asker.ID, requestID, e.Staff.ID))
}

func TestPostVoidRequestDeclineHandler_DoesNotTellTheOneWhoDeclinedHerOwn(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Admin()))
	e.seedContract(statusSigned)
	requestID := e.seedVoidRequestBy(e.Staff, "asking")

	e.decline(requestID, "changed my mind")

	expectQueued(t, e)
}

func TestPostVoidRequestDeclineHandler_ADecisionAlreadyMadeTellsNobodyAgain(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Admin()))
	e.seedContract(statusSigned)
	asker := e.Colleague(fixture.Doula())
	requestID := e.seedVoidRequestBy(asker, "asking")

	e.decline(requestID, "not yet")
	reqtest.ExpectStatus(t, http.MethodPost, e.declineURL(requestID), http.StatusNotFound, e.SignedIn, reasoned("still not yet"))

	expectQueued(t, e, notice("void_declined", asker.ID, requestID, e.Staff.ID))
}

// #2953: voiding the Contract tells every Doula whose request it closed.
func TestPostVoidContractHandler_TellsEachDoulaWhoAsked(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	e.seedContract(statusSigned)
	first := e.Colleague(fixture.Doula())
	second := e.Colleague(fixture.Doula(), fixture.Contractor())
	firstID := e.seedVoidRequestBy(first, "asking")
	secondID := e.seedVoidRequestBy(second, "asking too")

	reqtest.Expect[contracts.ContractResponse](t, http.MethodPost, e.voidURL(), http.StatusOK, e.SignedIn)

	expectQueued(t, e,
		notice("void_granted", first.ID, firstID, e.Staff.ID),
		notice("void_granted", second.ID, secondID, e.Staff.ID))
}

func TestPostVoidContractHandler_DoesNotTellTheOwnerWhoAskedAndVoided(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	e.seedContract(statusSigned)
	e.seedVoidRequestBy(e.Staff, "asking")

	reqtest.Expect[contracts.ContractResponse](t, http.MethodPost, e.voidURL(), http.StatusOK, e.SignedIn)

	expectQueued(t, e)
}
