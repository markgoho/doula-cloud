package contracts_test

import (
	"net/http"
	"slices"
	"testing"

	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

const kindVoidRequested = "void_requested"

type queuedNotice struct{ recipient, kind, subject, triggeredBy string }

func queuedNotices(t *testing.T, e *env) []queuedNotice {
	t.Helper()
	rows, err := e.DB.Admin.QueryContext(t.Context(),
		`SELECT recipient_staff_id, kind::text, subject_id, triggered_by_staff_id
		   FROM staff_notice_outbox WHERE status = 'pending' ORDER BY kind, recipient_staff_id`)
	if err != nil {
		t.Fatalf("read staff_notice_outbox: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var out []queuedNotice
	for rows.Next() {
		var n queuedNotice
		if err := rows.Scan(&n.recipient, &n.kind, &n.subject, &n.triggeredBy); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read rows: %v", err)
	}
	return out
}

func sortedNotices(n ...queuedNotice) []queuedNotice {
	slices.SortFunc(n, func(a, b queuedNotice) int {
		if a.kind != b.kind {
			return compareStrings(a.kind, b.kind)
		}
		return compareStrings(a.recipient, b.recipient)
	})
	return n
}

func compareStrings(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
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

	requestID := out.VoidRequests[0].ID
	want := sortedNotices(
		queuedNotice{owner.ID, kindVoidRequested, requestID, e.Staff.ID},
		queuedNotice{admin.ID, kindVoidRequested, requestID, e.Staff.ID},
	)
	if got := queuedNotices(t, e); !slices.Equal(got, want) {
		t.Fatalf("queued = %+v, want %+v", got, want)
	}
}

func TestPostVoidRequestHandler_DoesNotTellTheOwnerWhoAsked(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	e.seedContract(statusSigned)
	other := e.Colleague(fixture.Owner())

	out := e.requestVoid("asking myself", http.StatusCreated)

	want := []queuedNotice{{other.ID, kindVoidRequested, out.VoidRequests[0].ID, e.Staff.ID}}
	if got := queuedNotices(t, e); !slices.Equal(got, want) {
		t.Fatalf("queued = %+v, want %+v", got, want)
	}
}

func TestPostVoidRequestHandler_AContractorOnAGrantedAttachmentTellsThem(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Contractor()))
	e.seedContract(statusSigned)
	testdb.SeedGrantedAttachment(e.T, e.DB, e.Engagement.ID, e.Staff.ID)
	owner := e.Colleague(fixture.Owner())

	out := e.requestVoid("asking as a contractor", http.StatusCreated)

	want := []queuedNotice{{owner.ID, kindVoidRequested, out.VoidRequests[0].ID, e.Staff.ID}}
	if got := queuedNotices(t, e); !slices.Equal(got, want) {
		t.Fatalf("queued = %+v, want %+v", got, want)
	}
}

func TestPostVoidRequestHandler_ARefusedRequestTellsNobody(t *testing.T) {
	e := newEnv(t)
	e.seedContract(statusSigned)
	e.Colleague(fixture.Owner())

	reqtest.ExpectStatus(t, http.MethodPost, e.voidRequestURL(), http.StatusBadRequest, e.SignedIn, reasoned(""))

	if got := queuedNotices(t, e); len(got) != 0 {
		t.Fatalf("queued = %+v, want none", got)
	}
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

	want := []queuedNotice{{asker.ID, "void_declined", requestID, e.Staff.ID}}
	if got := queuedNotices(t, e); !slices.Equal(got, want) {
		t.Fatalf("queued = %+v, want %+v", got, want)
	}
}

func TestPostVoidRequestDeclineHandler_DoesNotTellTheOneWhoDeclinedHerOwn(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Admin()))
	e.seedContract(statusSigned)
	requestID := e.seedVoidRequestBy(e.Staff, "asking")

	e.decline(requestID, "changed my mind")

	if got := queuedNotices(t, e); len(got) != 0 {
		t.Fatalf("queued = %+v, want none", got)
	}
}

func TestPostVoidRequestDeclineHandler_ADecisionAlreadyMadeTellsNobodyAgain(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Admin()))
	e.seedContract(statusSigned)
	asker := e.Colleague(fixture.Doula())
	requestID := e.seedVoidRequestBy(asker, "asking")

	e.decline(requestID, "not yet")
	reqtest.ExpectStatus(t, http.MethodPost, e.declineURL(requestID), http.StatusNotFound, e.SignedIn, reasoned("still not yet"))

	if got := queuedNotices(t, e); len(got) != 1 {
		t.Fatalf("queued = %+v, want the one", got)
	}
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

	want := sortedNotices(
		queuedNotice{first.ID, "void_granted", firstID, e.Staff.ID},
		queuedNotice{second.ID, "void_granted", secondID, e.Staff.ID},
	)
	if got := queuedNotices(t, e); !slices.Equal(got, want) {
		t.Fatalf("queued = %+v, want %+v", got, want)
	}
}

func TestPostVoidContractHandler_DoesNotTellTheOwnerWhoAskedAndVoided(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	e.seedContract(statusSigned)
	e.seedVoidRequestBy(e.Staff, "asking")

	reqtest.Expect[contracts.ContractResponse](t, http.MethodPost, e.voidURL(), http.StatusOK, e.SignedIn)

	if got := queuedNotices(t, e); len(got) != 0 {
		t.Fatalf("queued = %+v, want none", got)
	}
}
