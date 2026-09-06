package client_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/testdb"
)

// TestEditHandler_CollisionPredicate_TwoSarahsPostalCodeSavesFreely
// proves #814's AC: two Clients sharing a given name at one Practice can
// each take a postal-code correction with no prompt of any kind -- the
// bug FindMatches' substring recall used to cause.
func TestEditHandler_CollisionPredicate_TwoSarahsPostalCodeSavesFreely(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedStaffWithMembership(t, db, "staff-two-sarahs")
	seedClientFull(t, db, practiceID, "Sarah", "Nguyen", "", "")
	editingID := seedClientFull(t, db, practiceID, "Sarah", "Osei", "", "")

	srv, session := newServer(t, db, "staff-two-sarahs")
	defer srv.Close()

	resp := authedJSON(t, session, http.MethodPut, srv.URL+"/api/practices/"+practiceID+"/clients/"+editingID,
		client.EditRequest{Record: client.Record{GivenName: "Sarah", FamilyName: "Osei", AddressPostalCode: "14604"}})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d (no prompt for two Sarahs)", resp.StatusCode, http.StatusOK)
	}
}

// TestEditHandler_CollisionPredicate_AnnDoesNotCollideWithSubstringCousins
// proves the second half of the same AC: a Client named Ann saves
// against Joanna, Hannah and Deanna on file, with no prompt -- substring
// recall used to fire on all three.
func TestEditHandler_CollisionPredicate_AnnDoesNotCollideWithSubstringCousins(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedStaffWithMembership(t, db, "staff-ann")
	seedClientFull(t, db, practiceID, "Joanna", "Reyes", "", "")
	seedClientFull(t, db, practiceID, "Hannah", "Reyes", "", "")
	seedClientFull(t, db, practiceID, "Deanna", "Reyes", "", "")
	editingID := seedClientFull(t, db, practiceID, "Ann", "Reyes", "", "")

	srv, session := newServer(t, db, "staff-ann")
	defer srv.Close()

	resp := authedJSON(t, session, http.MethodPut, srv.URL+"/api/practices/"+practiceID+"/clients/"+editingID,
		client.EditRequest{Record: client.Record{GivenName: "Ann", FamilyName: "Reyes", AddressLocality: "Rochester"}})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d (Ann must not collide with Joanna/Hannah/Deanna)", resp.StatusCode, http.StatusOK)
	}
}

// TestEditHandler_CollisionPredicate_SharedDateOfBirthNoNameWordSavesFreely
// proves ADR-0017's amendment "a bare date-of-birth collision between two
// unrelated names is a coincidence and passes silently" -- twins with
// unrelated names sharing a date of birth, unlike a case with a shared
// name word, block nothing.
func TestEditHandler_CollisionPredicate_SharedDateOfBirthNoNameWordSavesFreely(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedStaffWithMembership(t, db, "staff-shared-dob")
	seedClientFull(t, db, practiceID, "Priya", "Chandra", "", "1990-05-01")
	editingID := seedClientFull(t, db, practiceID, "Wren", testFletcher, "", "1990-05-01")

	srv, session := newServer(t, db, "staff-shared-dob")
	defer srv.Close()

	resp := authedJSON(t, session, http.MethodPut, srv.URL+"/api/practices/"+practiceID+"/clients/"+editingID,
		client.EditRequest{Record: client.Record{GivenName: "Wren", FamilyName: testFletcher, DateOfBirth: "1990-05-01", Phone: "555-0199"}})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d (shared DOB with no shared name word must not prompt)", resp.StatusCode, http.StatusOK)
	}
}

// TestEditHandler_CollisionPredicate_SharedDateOfBirthWithNameWordAsks
// proves the other side of that same rule: sharing a date of birth AND a
// whole name word is a possible duplicate -- gate two, not silence.
func TestEditHandler_CollisionPredicate_SharedDateOfBirthWithNameWordAsks(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedStaffWithMembership(t, db, "staff-dob-shared-word")
	seedClientFull(t, db, practiceID, "Priya", "Chandra", "", "1990-05-01")
	editingID := seedClientFull(t, db, practiceID, "Priya", testFletcher, "", "")

	srv, session := newServer(t, db, "staff-dob-shared-word")
	defer srv.Close()

	resp := authedJSON(t, session, http.MethodPut, srv.URL+"/api/practices/"+practiceID+"/clients/"+editingID,
		client.EditRequest{Record: client.Record{GivenName: "Priya", FamilyName: testFletcher, DateOfBirth: "1990-05-01"}})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d (shared DOB + shared name word is a possible duplicate)", resp.StatusCode, http.StatusConflict)
	}
	var out client.EditConflictResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Substitution {
		t.Fatalf("substitution = true, want gate two (possible duplicate), not gate one")
	}
}

// TestEditHandler_CollisionPredicate_ExactEmailAsksRegardlessOfName proves
// email counts as a collision on its own, per ADR-0017's amendment: two
// Clients with no name in common at all still ask when their email
// matches exactly.
func TestEditHandler_CollisionPredicate_ExactEmailAsksRegardlessOfName(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedStaffWithMembership(t, db, "staff-exact-email")
	seedClientFull(t, db, practiceID, "Priya", "Chandra", "shared@example.com", "")
	editingID := seedClientFull(t, db, practiceID, "Wren", testFletcher, "", "")

	srv, session := newServer(t, db, "staff-exact-email")
	defer srv.Close()

	resp := authedJSON(t, session, http.MethodPut, srv.URL+"/api/practices/"+practiceID+"/clients/"+editingID,
		client.EditRequest{Record: client.Record{GivenName: "Wren", FamilyName: testFletcher, Email: "shared@example.com"}})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d (exact email collides regardless of name)", resp.StatusCode, http.StatusConflict)
	}
}

// TestEditHandler_CollisionPredicate_CorrectingToExistingFirstNameSaves
// proves ADR-0017's amendment worked example: "Sara" corrected to "Sarah"
// still saves where a Sarah Chen is already on file -- the corrected name
// does not equal Sarah Chen's, so gate one never fires.
func TestEditHandler_CollisionPredicate_CorrectingToExistingFirstNameSaves(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedStaffWithMembership(t, db, "staff-sara-to-sarah")
	seedClientFull(t, db, practiceID, "Sarah", "Chen", "", "")
	editingID := seedClientFull(t, db, practiceID, "Sara", "Beck", "", "")

	srv, session := newServer(t, db, "staff-sara-to-sarah")
	defer srv.Close()

	resp := authedJSON(t, session, http.MethodPut, srv.URL+"/api/practices/"+practiceID+"/clients/"+editingID,
		client.EditRequest{Record: client.Record{GivenName: "Sarah", FamilyName: "Beck"}})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d (Sara -> Sarah Beck must not collide with Sarah Chen)", resp.StatusCode, http.StatusOK)
	}
}

// TestEditHandler_CollisionPredicate_SubstitutionBlocksWithSubstitutionFlag
// proves gate one itself: a name column changed and the result exactly
// matches another Client's given and family name -- refused, and the
// response names it a substitution so the frontend never offers a merge
// on it.
func TestEditHandler_CollisionPredicate_SubstitutionBlocksWithSubstitutionFlag(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedStaffWithMembership(t, db, "staff-substitution")
	existingID := seedClientFull(t, db, practiceID, testNadia, testHaddad, "", "")
	editingID := seedClientFull(t, db, practiceID, "Sarah", "Beck", "", "")

	srv, session := newServer(t, db, "staff-substitution")
	defer srv.Close()

	resp := authedJSON(t, session, http.MethodPut, srv.URL+"/api/practices/"+practiceID+"/clients/"+editingID,
		client.EditRequest{Record: client.Record{GivenName: testNadia, FamilyName: testHaddad}})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
	var out client.EditConflictResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !out.Substitution {
		t.Fatalf("substitution = false, want true (exact given+family match)")
	}
	if out.MergeOffered {
		t.Fatalf("mergeOffered = true, want false -- gate one never offers a merge")
	}
	if len(out.Matches) != 1 || out.Matches[0].ID != existingID {
		t.Fatalf("matches = %+v, want exactly Nadia Haddad", out.Matches)
	}
}

// TestEditHandler_RefusesEditingAMergedRecord proves a tombstoned row is
// not editable -- clients_update's own USING clause (00080) would
// otherwise let the UPDATE silently match zero rows.
func TestEditHandler_RefusesEditingAMergedRecord(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-edit-merged"
	practiceID := seedStaffWithMembership(t, db, identityUID)
	survivorID := seedClient(t, db, practiceID, "Survivor", "")
	absorbedID := seedClient(t, db, practiceID, "Absorbed", "")
	if _, err := db.Admin.ExecContext(t.Context(), `UPDATE clients SET merged_into = $2 WHERE id = $1`, absorbedID, survivorID); err != nil {
		t.Fatalf("seed merged_into: %v", err)
	}

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedJSON(t, session, http.MethodPut, srv.URL+"/api/practices/"+practiceID+"/clients/"+absorbedID,
		client.EditRequest{Record: client.Record{GivenName: "Absorbed", AddressLocality: "Rochester"}})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
}
