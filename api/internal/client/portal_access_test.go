package client_test

import (
	"encoding/json"
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

	resp := authedJSON(t, session, http.MethodPut, srv.URL+"/api/practices/"+practiceID+"/clients/"+clientID,
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

// TestDetailHandler_ShowsHerOwnSignInAddressChange is the read side of
// #619's activity record, and the one Client-authored row the
// client-subject history has: her Practice must be able to answer "how
// did this come to be?" and see her name against it, not a Staff
// member's and not "Doula Cloud".
//
// The diff is written unsealed (ADR-0027 seals what changed, and this
// row deliberately records no address at all -- ADR-0015 makes her login
// hers, not the Practice's contact detail), so this also proves openDiff
// passes a plaintext diff straight through rather than trying to unseal
// it.
func TestDetailHandler_ShowsHerOwnSignInAddressChange(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-reading-her-change"
	practiceID := seedStaffWithMembership(t, db, identityUID)
	clientID := seedClient(t, db, practiceID, "Margaretha", "contact@example.com")

	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO activity (practice_id, subject_kind, subject_id, action, diff, actor_kind, actor_client_id)
		 VALUES ($1, 'client', $2, 'portal_sign_in_address_changed', '{}', 'client', $2)`,
		practiceID, clientID,
	); err != nil {
		t.Fatalf("seed activity row: %v", err)
	}

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients/"+clientID)
	defer resp.Body.Close()
	var out client.DetailResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode detail: %v", err)
	}

	for _, entry := range out.History {
		if entry.ClientEvent == nil || entry.ClientEvent.EventType != "portal_sign_in_address_changed" {
			continue
		}
		if entry.ClientEvent.ActorKind != "client" {
			t.Fatalf("actorKind = %q, want client", entry.ClientEvent.ActorKind)
		}
		if entry.ClientEvent.ActorName == nil || *entry.ClientEvent.ActorName != "Margaretha" {
			t.Fatalf("actorName = %v, want her own name", entry.ClientEvent.ActorName)
		}
		if string(entry.ClientEvent.Diff) != "{}" {
			t.Fatalf("diff = %s, want an empty diff carrying neither address", entry.ClientEvent.Diff)
		}
		return
	}
	t.Fatalf("no sign-in-address change in history %+v", out.History)
}
