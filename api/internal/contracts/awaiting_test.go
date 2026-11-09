package contracts_test

import (
	"net/http"
	"strings"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/apierrtest"
	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// seedNamedEngagement seeds a Client carrying both a legal given name and
// the preferred name she is actually called, plus her Engagement --
// seedEngagement's Client has no preferred name, and the roll-up is
// supposed to print the one a person answers to.
func seedNamedEngagement(t *testing.T, db *testdb.DB, practiceID, givenName, preferredName string) (engagementID, clientID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (practice_id, given_name, preferred_name, email)
		      VALUES ($1, $2, $3, 'client@example.com') RETURNING id`,
		practiceID, givenName, preferredName,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, kind) VALUES ($1, $2, 'birth') RETURNING id`,
		clientID, practiceID,
	).Scan(&engagementID); err != nil {
		t.Fatalf("seed engagement: %v", err)
	}
	return engagementID, clientID
}

func (e *env) awaitingURL() string {
	return e.Srv.URL + "/api/practices/" + e.Practice.ID + "/contracts/awaiting-signature"
}

// awaiting fetches one page of the roll-up, with query appended to its
// route, and decodes it, failing the test unless the status is 200.
func (e *env) awaiting(query string) contracts.AwaitingResponse {
	e.T.Helper()
	return reqtest.Expect[contracts.AwaitingResponse](e.T, http.MethodGet, e.awaitingURL()+query, http.StatusOK, e.SignedIn)
}

// seedOutstanding seeds n Engagements each with a sent Contract, attached
// to the Staff member at the screen when attach is set.
func (e *env) seedOutstanding(n int, attach bool) {
	e.T.Helper()
	for range n {
		engagement := e.AnotherEngagement()
		if attach {
			testdb.SeedGrantedAttachment(e.T, e.DB, engagement.ID, e.Staff.ID)
		}
		seedContract(e.T, e.DB, engagement.ID, statusSent, mergeFieldProse)
	}
}

// itemIDs is the set of Engagements the roll-up lists.
func itemIDs(out contracts.AwaitingResponse) map[string]bool {
	ids := map[string]bool{}
	for _, item := range out.Items {
		ids[item.EngagementID] = true
	}
	return ids
}

// TestAwaitingSignatureHandler_EmptyPracticeAnswersWithAnEmptyList proves
// a Practice owing no signatures gets an empty list rather than JSON
// null -- asserted on the raw bytes, because a []AwaitingItem field
// decodes null and [] into the same nil slice and only one of the two is
// a list the screen can render.
func TestAwaitingSignatureHandler_EmptyPracticeAnswersWithAnEmptyList(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))

	body := reqtest.ExpectBody(t, http.MethodGet, e.awaitingURL(), http.StatusOK, e.SignedIn)

	if !strings.Contains(string(body), `"items":[]`) {
		t.Fatalf("body = %s, want an empty items array", body)
	}
	if strings.Contains(string(body), "nextCursor") {
		t.Fatalf("body = %s, want no cursor on a list with nothing after it", body)
	}
}

// TestAwaitingSignatureHandler_ListsOnlyOutstandingContractsOldestFirst
// is #426's whole point: every Contract still waiting on somebody, in one
// read, oldest first -- and nothing else. A signed Contract is done and a
// voided one is a superseded record; neither is work anybody can chase.
func TestAwaitingSignatureHandler_ListsOnlyOutstandingContractsOldestFirst(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))

	draftEngagement, draftClient := seedNamedEngagement(t, e.DB, e.Practice.ID, "Jamesina", jamieName)
	// #2114: the list names each Client in full, her preferred name
	// followed by the family name.
	testdb.NameClient(t, e.DB, draftClient, jamieName, "Okafor")
	seedContract(t, e.DB, draftEngagement, statusDraft, mergeFieldProse)
	sentEngagement, _ := seedNamedEngagement(t, e.DB, e.Practice.ID, "Renata", "")
	seedContract(t, e.DB, sentEngagement, statusSent, mergeFieldProse)
	seedContract(t, e.DB, e.AnotherEngagement().ID, statusSigned, mergeFieldProse)
	seedContract(t, e.DB, e.AnotherEngagement().ID, statusVoided, mergeFieldProse)

	out := e.awaiting("")

	if len(out.Items) != 2 || out.HasMore || out.NextCursor != nil {
		t.Fatalf("page = %d items, hasMore=%v, cursor=%v; want the two outstanding Contracts on one page",
			len(out.Items), out.HasMore, out.NextCursor)
	}
	first, second := out.Items[0], out.Items[1]
	if first.EngagementID != draftEngagement || second.EngagementID != sentEngagement {
		t.Fatalf("order = %s, %s; want the longest wait first", first.EngagementID, second.EngagementID)
	}
	if first.Status != statusDraft || second.Status != statusSent {
		t.Fatalf("statuses = %q, %q; want the Practice's own unsent work told apart from the Client's",
			first.Status, second.Status)
	}
	if first.ClientID != draftClient || first.ClientName != jamieName+" Okafor" {
		t.Fatalf("client = %s/%q, want the Client named as she is called, in full", first.ClientID, first.ClientName)
	}
	if second.ClientName != "Renata" {
		t.Fatalf("client name = %q, want the legal given name where no preferred one was given", second.ClientName)
	}
	if first.ContractID == "" || first.CreatedAt.IsZero() {
		t.Fatalf("row = %+v, want the Contract identified and dated", first)
	}
}

// TestAwaitingSignatureHandler_IgnoresAnotherPracticesContracts proves
// the roll-up is scoped to the caller's Practice: an outstanding Contract
// somewhere else is not this Practice's work to chase.
func TestAwaitingSignatureHandler_IgnoresAnotherPracticesContracts(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	otherPracticeID := testdb.SeedPractice(t, e.DB, "Other Practice")
	_, otherEngagement := testdb.SeedEngagement(t, e.DB, otherPracticeID)
	seedContract(t, e.DB, otherEngagement, statusSent, mergeFieldProse)
	e.seedContract(statusSent)

	out := e.awaiting("")

	if len(out.Items) != 1 || out.Items[0].EngagementID != e.Engagement.ID {
		t.Fatalf("items = %+v, want only this Practice's outstanding Contract", out.Items)
	}
}

// TestAwaitingSignatureHandler_AdmitsAnAdminAndAnEmployedDoula proves who
// reaches the roll-up. The person who actually chases signatures at a
// fourteen-doula agency is an Admin, not the Owner, so she is the second
// of the two seats. ADR-0008's Engagements/Visits/Messages row, not the
// money row, governs this roll-up (#973): an employed Doula reaches every
// Engagement at the Practice, so she sees every outstanding Contract, the
// same as an Owner or Admin.
func TestAwaitingSignatureHandler_AdmitsAnAdminAndAnEmployedDoula(t *testing.T) {
	cases := []struct {
		name  string
		staff fixture.StaffOption
	}{
		{"an Admin", fixture.Admin()},
		{"an employed Doula", fixture.Doula()},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(c.staff))
			e.seedContract(statusSent)

			out := e.awaiting("")

			if len(out.Items) != 1 || out.Items[0].EngagementID != e.Engagement.ID {
				t.Fatalf("items = %+v, want the outstanding Contract", out.Items)
			}
		})
	}
}

// TestAwaitingSignatureHandler_ContractorSeesOnlyHerAttachedEngagements
// is #973's own security boundary: a contractor Doula reaches only an
// Engagement she holds an open, granted attachment on, so an outstanding
// Contract on an Engagement she is not attached to never reaches her. A
// contractor who holds no granted attachment at all gets an empty list,
// not an error and not the Practice's whole roll-up -- the narrow case
// #973's acceptance criteria calls out by name.
func TestAwaitingSignatureHandler_ContractorSeesOnlyHerAttachedEngagements(t *testing.T) {
	t.Run("attached to one of two", func(t *testing.T) {
		e := newEnv(t, signedInAs(fixture.Contractor()), attachedToEngagement())
		e.seedContract(statusSent)
		seedContract(t, e.DB, e.AnotherEngagement().ID, statusSent, mergeFieldProse)

		out := e.awaiting("")

		if len(out.Items) != 1 || out.Items[0].EngagementID != e.Engagement.ID {
			t.Fatalf("items = %+v, want exactly the one attached Engagement (no leak of the unattached one)", out.Items)
		}
	})

	t.Run("attached to none", func(t *testing.T) {
		e := newEnv(t, signedInAs(fixture.Contractor()))
		e.seedContract(statusSent)

		if out := e.awaiting(""); len(out.Items) != 0 {
			t.Fatalf("items = %+v, want none for a contractor with no granted attachment", out.Items)
		}
	})
}

// TestAwaitingSignatureHandler_OwnerOrAdminContractorSeesTheWholePractice
// is #1635: the list narrows only for the contractor who holds neither
// the Owner role nor the Admin role (staffauth.Reader.IsAmbientContractor,
// the population ADR-0008 confines). Employment type is independent of
// roles (GLOSSARY.md), so an Owner or an Admin whose Membership reads
// "contractor" is real, and she reaches the whole Practice. She reads
// each outstanding Contract, on an Engagement she holds a granted
// Attachment on and on one she does not. The same defect on the Clients
// list is #742.
func TestAwaitingSignatureHandler_OwnerOrAdminContractorSeesTheWholePractice(t *testing.T) {
	cases := []struct {
		name     string
		staff    []fixture.StaffOption
		attached bool
	}{
		{"an Owner who is a contractor, with no Attachment", []fixture.StaffOption{fixture.Owner()}, false},
		{"an Admin who is a contractor, with no Attachment", []fixture.StaffOption{fixture.Admin()}, false},
		{"an Owner who is a contractor Doula, attached to one Engagement", []fixture.StaffOption{fixture.Owner(), fixture.Doula()}, true},
		{"an Admin who is a contractor Doula, attached to one Engagement", []fixture.StaffOption{fixture.Admin(), fixture.Doula()}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(append(tc.staff, fixture.Contractor())...))
			e.seedContract(statusSent)
			if tc.attached {
				testdb.SeedGrantedAttachment(t, e.DB, e.Engagement.ID, e.Staff.ID)
			}
			second := e.AnotherEngagement()
			seedContract(t, e.DB, second.ID, statusDraft, mergeFieldProse)

			out := e.awaiting("")

			got := itemIDs(out)
			if len(out.Items) != 2 || !got[e.Engagement.ID] || !got[second.ID] {
				t.Fatalf("items = %+v, want both outstanding Contracts at the Practice -- an Owner or an Admin is not narrowed to her own Attachments", out.Items)
			}
		})
	}
}

// TestAwaitingSignatureHandler_RejectsAMalformedCursor proves a cursor
// nobody this endpoint issued is refused rather than silently treated as
// page one, in docs/api-design.md section 7's structured shape.
func TestAwaitingSignatureHandler_RejectsAMalformedCursor(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))

	for _, cursor := range []string{"not!valid!base64!", "YmFkdGltZXxzb21lLWlk"} {
		out := apierrtest.Expect(t, http.MethodGet, e.awaitingURL()+"?cursor="+cursor, http.StatusBadRequest, e.SignedIn)

		if out.Code != apierr.CodeInvalidArgument || out.Message != contracts.MsgInvalidCursor {
			t.Fatalf("cursor %q: error = %+v, want the structured refusal", cursor, out)
		}
	}
}

// TestAwaitingSignatureHandler_WalksTheCursor proves the envelope
// docs/api-design.md section 4 asks for: a full first page carries a
// cursor, and that cursor resumes at the row after the last one rather
// than repeating it. The contractor row exercises listAttachedAwaiting's
// own cursor branch (the `after != nil` path).
func TestAwaitingSignatureHandler_WalksTheCursor(t *testing.T) {
	cases := []struct {
		name   string
		staff  fixture.StaffOption
		attach bool
	}{
		{"owner", fixture.Owner(), false},
		{"contractor", fixture.Contractor(), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(c.staff))
			const total = 31 // awaitingPageSize (30) + 1, to force a second page
			e.seedOutstanding(total, c.attach)

			first := e.awaiting("")
			if len(first.Items) != 30 || !first.HasMore || first.NextCursor == nil {
				t.Fatalf("first page = %d items, hasMore=%v, cursor=%v; want 30/true/non-nil",
					len(first.Items), first.HasMore, first.NextCursor)
			}

			second := e.awaiting("?cursor=" + *first.NextCursor)
			if len(second.Items) != 1 || second.HasMore || second.NextCursor != nil {
				t.Fatalf("second page = %d items, hasMore=%v, cursor=%v; want 1/false/nil",
					len(second.Items), second.HasMore, second.NextCursor)
			}
			if second.Items[0].ContractID == first.Items[29].ContractID {
				t.Fatal("the second page repeated the first page's last row")
			}
		})
	}
}
