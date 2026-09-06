package client_test

import (
	"net/http"
	"testing"

	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/portalaccount"
	"doula-cloud/api/internal/testdb"
)

// TestEditHandler_EmailChangeLeavesPortalAccessAlone is #619's third
// acceptance criterion, asserted rather than read off the code: ADR-0015
// separates the address the Practice reaches her at from the address she
// signs in with, so a doula fixing a typo in a contact email must not
// revoke anything. It is a test rather than a comment because the
// absence of a behavior is exactly what nobody notices someone adding.
//
// The pending-invite dead-lettering next door
// (TestEditHandler_ChangingEmailRevokesPendingInvite) is not a
// counter-example: that stops an unsent invitation mail going to an
// address the Practice has just said is wrong. It touches no accepted
// Portal Account, which is what this asserts.
func TestEditHandler_EmailChangeLeavesPortalAccessAlone(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-editing-email"
	practiceID := seedStaffWithMembership(t, db, identityUID)
	clientID := seedClient(t, db, practiceID, "Portal Client", "old-contact@example.com")

	portalIdentifier := portalaccount.NewIdentifier()
	testdb.SeedPortalAccount(t, db, portalIdentifier, "her-own@example.com")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO client_portal_users (identity_uid, client_id) VALUES ($1, $2)`, portalIdentifier, clientID,
	); err != nil {
		t.Fatalf("seed client_portal_users: %v", err)
	}

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedJSON(t, session, http.MethodPut, srv.URL+"/practices/"+practiceID+"/clients/"+clientID,
		client.EditRequest{Record: client.Record{GivenName: "Portal Client", Email: "new-contact@example.com"}})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var linkedIdentity, signInAddress string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT pu.identity_uid, pa.sign_in_address
		   FROM client_portal_users pu
		   JOIN portal_accounts pa ON pa.identifier = pu.identity_uid
		  WHERE pu.client_id = $1`,
		clientID,
	).Scan(&linkedIdentity, &signInAddress); err != nil {
		t.Fatalf("read portal account after the edit: %v", err)
	}
	if linkedIdentity != portalIdentifier {
		t.Fatalf("identity_uid = %q, want the Portal Account still linked (%q)", linkedIdentity, portalIdentifier)
	}
	if signInAddress != "her-own@example.com" {
		t.Fatalf("sign_in_address = %q, want it untouched by a contact-email edit", signInAddress)
	}
}
