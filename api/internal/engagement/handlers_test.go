package engagement_test

import (
	"encoding/json/jsontext"
	"net/http"
	"slices"
	"testing"

	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

func TestDetailHandler_Success(t *testing.T) {
	e := newEnv(t, withEngagement(fixture.NewClient(fixture.ClientName("Detail Client")), fixture.Status(fixture.StatusActive)))

	d := e.detail()

	if d.EngagementID != e.Engagement.ID || d.ClientName != "Detail Client" || d.Status != engagement.StatusActive {
		t.Fatalf("unexpected detail: %+v", d)
	}
}

// TestDetailHandler_ClientDisplayName proves the page's title carries her
// family name, as her own Client record's title does, while clientName
// stays the conversation name (#2004). A preferred name leads the title
// and keeps the family name after it (#2111): the title is
// personname.Full, so "Ama" alone never titles a record whose family name
// is on file.
func TestDetailHandler_ClientDisplayName(t *testing.T) {
	e := newEnv(t, withEngagement(fixture.NewClient(fixture.ClientName("Amara"))))
	e.Exec(`UPDATE clients SET family_name = 'Okafor', preferred_name = 'Ama' WHERE id = $1`, e.Engagement.Client.ID)

	d := e.detail()

	if d.ClientDisplayName != "Ama Okafor" || d.ClientName != "Ama" {
		t.Fatalf("clientDisplayName = %q, clientName = %q, want %q and %q", d.ClientDisplayName, d.ClientName, "Ama Okafor", "Ama")
	}
}

// TestDetailHandler_DueDate proves the Staff side reads the same
// engagements.due_date column the Client portal's page already reads
// (#505), so a Doula can see when the Engagement she's working is due
// without leaving the page (#538).
func TestDetailHandler_DueDate(t *testing.T) {
	e := newEnv(t, withEngagement(fixture.DueDate("2027-06-15")))

	d := e.detail()

	if d.DueDate == nil || *d.DueDate != "2027-06-15" {
		t.Fatalf("dueDate = %v, want %q", d.DueDate, "2027-06-15")
	}
}

// TestDetailHandler_Kind proves the Staff side reads engagements.kind
// (#874) -- staff-only, and absent from portal.Detail entirely
// (api/internal/portal/detail_test.go's own key-absence assertion).
func TestDetailHandler_Kind(t *testing.T) {
	e := newEnv(t, withEngagement(fixture.Kind(fixture.KindPostpartum)))

	d := e.detail()

	if d.Kind != string(engagement.KindPostpartum) {
		t.Fatalf("kind = %q, want postpartum", d.Kind)
	}
}

// TestDetailHandler_NullDueDate covers ADR-0017's "genuinely none" case: a
// postpartum-only Engagement has no due date, and the field must be
// omitted from the JSON entirely -- the page relies on `omitempty` to
// tell "nothing to show" apart from a fetch that broke.
func TestDetailHandler_NullDueDate(t *testing.T) {
	e := newEnv(t)

	raw := reqtest.Expect[map[string]jsontext.Value](t, http.MethodGet, e.engagementURL(""), http.StatusOK, e.SignedIn)

	if _, present := raw["dueDate"]; present {
		t.Fatalf("dueDate key present in response, want omitted: %v", raw["dueDate"])
	}
}

func TestDetailHandler_NotFoundAtWrongPractice(t *testing.T) {
	e := newEnv(t)
	elsewhere := fixture.Engagement(t, e.DB, fixture.Practice(t, e.DB))

	reqtest.ExpectStatus(t, http.MethodGet, e.Srv.URL+"/api/practices/"+e.Practice.ID+"/engagements/"+elsewhere.ID, http.StatusNotFound, e.SignedIn)
}

func TestDetailHandler_InvalidEngagementID(t *testing.T) {
	e := newEnv(t)

	reqtest.ExpectStatus(t, http.MethodGet, e.Srv.URL+"/api/practices/"+e.Practice.ID+"/engagements/not-a-uuid", http.StatusBadRequest, e.SignedIn)
}

// TestDetailHandler_ContractorWithoutAttachmentForbidden proves the
// per-Engagement half of ADR-0008's attachment rule: same "not found"
// response an out-of-practice Engagement gets, so a contractor can't
// distinguish "doesn't exist" from "not attached" (#230).
func TestDetailHandler_ContractorWithoutAttachmentForbidden(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Contractor()))

	reqtest.ExpectStatus(t, http.MethodGet, e.engagementURL(""), http.StatusNotFound, e.SignedIn)
}

// TestDetailHandler_ContractorWithGrantedAttachmentSucceeds proves the
// other half: an open, granted attachment reaches.
func TestDetailHandler_ContractorWithGrantedAttachmentSucceeds(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Contractor()))
	testdb.SeedGrantedAttachment(t, e.DB, e.Engagement.ID, e.Staff.ID)

	d := e.detail()

	if len(d.StatusMoves) != 0 {
		t.Fatalf("statusMoves = %v, want none -- ADR-0015 refuses a contractor Doula every move outright", d.StatusMoves)
	}
}

// TestDetailHandler_StatusMoves proves Detail.StatusMoves (#253) mirrors
// legalMoves for every combination TestTransitionHandler_LegalMovesByRole
// doesn't already reach through a successful write -- a bare Staff
// member with no role, and an employee Doula reading a completed
// Engagement, whose reopen move is Owner/Admin only.
func TestDetailHandler_StatusMoves(t *testing.T) {
	cases := []struct {
		name   string
		role   roleCase
		status string
		want   []string
	}{
		{"bare staff, no role, at intake", noRoleCase, engagement.StatusIntake, nil},
		{"owner at intake sees active and completed", ownerCase, engagement.StatusIntake, []string{engagement.StatusActive, engagement.StatusCompleted}},
		{"owner at completed sees reopen", ownerCase, engagement.StatusCompleted, []string{engagement.StatusActive}},
		{"employee doula at completed sees no reopen", employeeDoulaCase, engagement.StatusCompleted, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(tc.role.opts...), withEngagement(fixture.Status(tc.status)))

			got := e.detail().StatusMoves

			if !slices.Equal(got, tc.want) && len(got)+len(tc.want) > 0 {
				t.Fatalf("statusMoves = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestDetailHandler_ClientPortalState proves Detail's #255 fields --
// ClientPortalInviteStatus/ClientEmailSuppressed/ClientHasEmail -- read
// client.FetchPortalInviteState exactly, the same derivation the Clients
// list's own PortalInviteStatus/EmailSuppressed use, reinstated on the
// Engagement hub as standing information.
func TestDetailHandler_ClientPortalState(t *testing.T) {
	e := newEnv(t)
	active := fixture.Status(fixture.StatusActive)
	engagementFor := func(c fixture.ClientRecord) string {
		return fixture.Engagement(t, e.DB, e.Practice, fixture.ForClient(c), active).ID
	}
	detailOf := func(engagementID string) engagement.Detail {
		return reqtest.Expect[engagement.Detail](t, http.MethodGet,
			e.Srv.URL+"/api/practices/"+e.Practice.ID+"/engagements/"+engagementID, http.StatusOK, e.SignedIn)
	}

	neverInvited := engagementFor(fixture.Client(t, e.DB, e.Practice))
	pending := fixture.Client(t, e.DB, e.Practice)
	testdb.SeedPendingPortalInvite(t, e.DB, pending.ID)
	accepted := fixture.Client(t, e.DB, e.Practice, fixture.WithPortalLogin())
	noEmail := fixture.Client(t, e.DB, e.Practice, fixture.NoEmail())

	if d := detailOf(neverInvited); d.ClientPortalInviteStatus != nil || !d.ClientHasEmail {
		t.Fatalf("never-invited detail = %+v, want nil status and hasEmail true", d)
	}
	if d := detailOf(engagementFor(pending)); d.ClientPortalInviteStatus == nil || *d.ClientPortalInviteStatus != "pending" {
		t.Fatalf("pending detail status = %v, want \"pending\"", d.ClientPortalInviteStatus)
	}
	if d := detailOf(engagementFor(accepted)); d.ClientPortalInviteStatus == nil || *d.ClientPortalInviteStatus != "accepted" {
		t.Fatalf("accepted detail status = %v, want \"accepted\"", d.ClientPortalInviteStatus)
	}
	if d := detailOf(engagementFor(noEmail)); d.ClientHasEmail {
		t.Fatal("no-email detail hasEmail = true, want false")
	}
}

// TestDetailHandler_ClientsCanPay proves ClientsCanPay (#270) reads
// practices.stripe_connect_card_payments_status directly rather than
// GetConnectStatusHandler's own live Stripe retrieve: false by default
// (the column's own default is 'unsupported'), and true only once the
// column reads 'active', the same webhook-synced state PostInvoiceHandler
// gates on.
func TestDetailHandler_ClientsCanPay(t *testing.T) {
	e := newEnv(t)

	if e.detail().ClientsCanPay {
		t.Fatal("clientsCanPay = true before Stripe Connect is active, want false")
	}

	testdb.SeedClientsCanPay(t, e.DB, e.Practice.ID)

	if !e.detail().ClientsCanPay {
		t.Fatal("clientsCanPay = false once card_payments is active, want true")
	}
}
