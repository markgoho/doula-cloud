package contracts_test

import (
	"database/sql"
	"net/http"
	"testing"

	"doula-cloud/api/internal/contracts"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// voidURL is the route that voids the Engagement's Contract.
func (e *env) voidURL() string {
	return e.contractURL("/void")
}

func TestPostVoidContractHandler_RefusesAnEngagementItCannotReach(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	e.refusesEngagementsItCannotReach(http.MethodPost, "/void")
}

func TestPostVoidContractHandler_NoContract(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))

	reqtest.ExpectStatus(t, http.MethodPost, e.voidURL(), http.StatusNotFound, e.SignedIn)
}

// TestPostVoidContractHandler_NonSignedRejected proves Void 409s from
// every status but 'signed' -- draft/sent haven't reached an agreement
// yet, and an already-voided Contract can't be voided a second time.
func TestPostVoidContractHandler_NonSignedRejected(t *testing.T) {
	for _, status := range []string{statusDraft, statusSent, statusVoided} {
		t.Run(status, func(t *testing.T) {
			e := newEnv(t, signedInAs(fixture.Owner()))
			e.seedContract(status)

			reqtest.ExpectStatus(t, http.MethodPost, e.voidURL(), http.StatusConflict, e.SignedIn)
		})
	}
}

// TestPostVoidContractHandler_Success proves the signed -> voided
// transition, that the response and a subsequent GET both reflect the
// new status while prose/mergeFields/values stay intact, and that the
// Signed PDF's object reference (signed_pdf_object_path) is left
// untouched by Void -- the ticket's "never deletes or alters" AC for the
// historical record.
func TestPostVoidContractHandler_Success(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	_, objectPath := seedSignedContract(t, e.DB, e.Engagement.ID)

	out := reqtest.Expect[contracts.ContractResponse](t, http.MethodPost, e.voidURL(), http.StatusOK, e.SignedIn)

	if out.Status != statusVoided {
		t.Fatalf("status = %q, want voided", out.Status)
	}
	if out.Prose != mergeFieldProse {
		t.Fatalf("prose = %q, want the original snapshot unchanged", out.Prose)
	}
	if got := e.contract().Status; got != statusVoided {
		t.Fatalf("GET status after Void = %q, want voided (the transition persisted)", got)
	}

	storedObjectPath := testdb.Scalar[sql.NullString](t, e.DB.Admin, `SELECT signed_pdf_object_path FROM contracts WHERE engagement_id = $1`, e.Engagement.ID)
	if !storedObjectPath.Valid || storedObjectPath.String != objectPath {
		t.Fatalf("signed_pdf_object_path = %+v, want unchanged at %q", storedObjectPath, objectPath)
	}
}

// TestPostContractHandler_AllowedAfterVoid proves the recreate-after-void
// flow the ticket's "What to build" section describes -- and #66's user
// story 22 -- actually works: contracts_engagement_id_active_key
// (00020_contracts_recreate_after_void.sql) permits a second row for the
// same Engagement once the first is voided, so POST no longer 409s
// forever the way it would under the original table-wide UNIQUE
// (engagement_id). It also proves the new row is independent of the old
// one: creating and then Sending it must never touch the voided row's
// status or its signed_pdf_object_path, which is exactly what targeting
// fetchContract's returned id (rather than re-deriving "the" row from
// engagement_id) in every UPDATE is for.
func TestPostContractHandler_AllowedAfterVoid(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	testdb.SeedPendingPortalInvite(t, e.DB, e.Engagement.Client.ID)
	e.readyToDraft(mergeFieldProse)
	_, oldObjectPath := seedSignedContract(t, e.DB, e.Engagement.ID)

	reqtest.ExpectStatus(t, http.MethodPost, e.voidURL(), http.StatusOK, e.SignedIn)

	created := reqtest.Expect[contracts.ContractResponse](t, http.MethodPost, e.contractURL(""), http.StatusCreated, e.SignedIn)
	if created.Status != statusDraft {
		t.Fatalf("new contract status = %q, want draft", created.Status)
	}

	// #258: Send now refuses a Contract with any blank merge field.
	// client_name is already prefilled from the Engagement's Client, and
	// mergeFieldProse's other field, price, resolves automatically from
	// amount_cents (#967) -- neither needs a PUT to reach the assertions
	// below, so this just round-trips client_name unchanged.
	reqtest.ExpectStatus(t, http.MethodPut, e.contractURL(""), http.StatusOK, e.SignedIn,
		putting(contracts.MergeFieldValues{clientNameKey: created.Values[clientNameKey]}))

	reqtest.ExpectStatus(t, http.MethodPost, e.send(), http.StatusOK, e.SignedIn)

	rows, err := e.DB.Admin.QueryContext(t.Context(),
		`SELECT status, signed_pdf_object_path FROM contracts WHERE engagement_id = $1 ORDER BY created_at`,
		e.Engagement.ID,
	)
	if err != nil {
		t.Fatalf("query contract rows: %v", err)
	}
	defer rows.Close()

	type row struct {
		status     string
		objectPath sql.NullString
	}
	var got []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.status, &r.objectPath); err != nil {
			t.Fatalf("scan row: %v", err)
		}
		got = append(got, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate rows: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("contract rows for engagement = %d, want 2 (the voided original plus the recreated one)", len(got))
	}
	if got[0].status != statusVoided || !got[0].objectPath.Valid || got[0].objectPath.String != oldObjectPath {
		t.Fatalf("original row = %+v, want voided with its signed_pdf_object_path untouched by the recreate/send", got[0])
	}
	if got[1].status != statusSent || got[1].objectPath.Valid {
		t.Fatalf("recreated row = %+v, want sent with no signed_pdf_object_path", got[1])
	}
}

// TestPostVoidContractHandler_RefusedByRole is #282/#970's own table:
// Void is Owner and Admin only, and refuses every Doula regardless of
// employment type -- the one Contract write AttachingWrite's reach test
// alone could never express, since an employed Doula and an attached
// contractor both reach the Engagement here just as freely as an Owner
// does.
func TestPostVoidContractHandler_RefusedByRole(t *testing.T) {
	cases := []struct {
		name       string
		staff      []fixture.StaffOption
		wantStatus int
	}{
		{"owner", []fixture.StaffOption{fixture.Owner()}, http.StatusOK},
		{caseAdmin, []fixture.StaffOption{fixture.Admin()}, http.StatusOK},
		{"employed doula", []fixture.StaffOption{fixture.Doula()}, http.StatusForbidden},
		{"contractor doula", []fixture.StaffOption{fixture.Doula(), fixture.Contractor()}, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, signedInAs(tc.staff...))
			seedSignedContract(t, e.DB, e.Engagement.ID)

			reqtest.ExpectStatus(t, http.MethodPost, e.voidURL(), tc.wantStatus, e.SignedIn)
		})
	}
}
