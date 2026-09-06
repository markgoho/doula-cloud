package client_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/testdb"
)

// mergedIntoOf reads clientID's merged_into column straight off the
// superuser connection -- the tombstone fact itself, independent of
// anything DetailHandler chooses to report.
func mergedIntoOf(t *testing.T, db *testdb.DB, clientID string) *string {
	t.Helper()
	var mergedInto *string
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT merged_into FROM clients WHERE id = $1`, clientID).Scan(&mergedInto); err != nil {
		t.Fatalf("read merged_into: %v", err)
	}
	return mergedInto
}

// activityActionsFor reads every activity row's action for clientID,
// oldest first -- what proves ADR-0017's amendment "recorded on both
// rows" without needing to unseal a diff.
func activityActionsFor(t *testing.T, db *testdb.DB, clientID string) []string {
	t.Helper()
	rows, err := db.Admin.QueryContext(t.Context(),
		`SELECT action FROM activity WHERE subject_kind = 'client' AND subject_id = $1 ORDER BY created_at`, clientID)
	if err != nil {
		t.Fatalf("query activity: %v", err)
	}
	defer rows.Close()
	var actions []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			t.Fatalf("scan activity: %v", err)
		}
		actions = append(actions, a)
	}
	return actions
}

// setCreatedAt backdates/forwards clientID's created_at, for tests that
// need a deterministic older/younger ordering between two rows without
// sleeping between two real inserts.
func setCreatedAt(t *testing.T, db *testdb.DB, clientID, createdAt string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(), `UPDATE clients SET created_at = $2 WHERE id = $1`, clientID, createdAt); err != nil {
		t.Fatalf("set created_at: %v", err)
	}
}

// TestMergeHandler_AbsorbsUnattachedIntoAttachedMatch proves #814's
// headline case: a phone-referral stub (unattached, given name only)
// later collides with a Client who already has an Engagement. "This is
// her" folds the stub's typed values into the attached record (a
// non-blank absorbed value wins, per the amendment), tombstones the
// stub, records the act on both rows, and the stub disappears from the
// Clients list, from search and from the collision predicate; its
// detail read reports where it went instead of rendering.
func TestMergeHandler_AbsorbsUnattachedIntoAttachedMatch(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-merge-absorb"
	practiceID := seedStaffWithMembership(t, db, identityUID)
	survivorID, _ := seedClientEngagement(t, db, practiceID, "Maya Torres", "old@example.com")
	stubID := seedClient(t, db, practiceID, testMaya, "")
	if _, err := db.Admin.ExecContext(t.Context(), `UPDATE clients SET phone = '555-0142' WHERE id = $1`, stubID); err != nil {
		t.Fatalf("seed stub phone: %v", err)
	}

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedJSON(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/clients/"+stubID+"/merge",
		client.MergeRequest{
			Record:        client.Record{GivenName: testMaya, Phone: "555-0142"},
			OtherClientID: survivorID,
		})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want %d, body=%s", resp.StatusCode, http.StatusOK, body)
	}
	var out client.Record
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.ID != survivorID {
		t.Fatalf("survivor id = %q, want %q -- the attached record must survive", out.ID, survivorID)
	}
	if out.GivenName != testMaya {
		t.Fatalf("givenName = %q, want the absorbed record's own non-blank value to win, same rule mergedEditFields already applies on intake", out.GivenName)
	}
	if out.Phone != "555-0142" {
		t.Fatalf("phone = %q, want the absorbed record's non-blank value to win", out.Phone)
	}
	if out.Email != "old@example.com" {
		t.Fatalf("email = %q, want the survivor's own value kept", out.Email)
	}

	if got := mergedIntoOf(t, db, stubID); got == nil || *got != survivorID {
		t.Fatalf("stub merged_into = %v, want %q", got, survivorID)
	}
	if got := mergedIntoOf(t, db, survivorID); got != nil {
		t.Fatalf("survivor merged_into = %v, want nil -- the survivor is never itself tombstoned", got)
	}

	survivorActions := activityActionsFor(t, db, survivorID)
	if len(survivorActions) == 0 || survivorActions[len(survivorActions)-1] != "merged" {
		t.Fatalf("survivor activity = %v, want a trailing %q row", survivorActions, "merged")
	}
	stubActions := activityActionsFor(t, db, stubID)
	if len(stubActions) == 0 || stubActions[len(stubActions)-1] != "absorbed" {
		t.Fatalf("stub activity = %v, want a trailing %q row", stubActions, "absorbed")
	}

	// Excluded from the Clients list.
	listResp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients?all=true")
	var list client.ListResponse
	if err := json.NewDecoder(listResp.Body).Decode(&list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	_ = listResp.Body.Close()
	for _, item := range list.Items {
		if item.ClientID == stubID {
			t.Fatalf("stub %q appears on the Clients list after being absorbed", stubID)
		}
	}

	// Excluded from search.
	searchResp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients/search?name=Maya")
	var search client.SearchResponse
	if err := json.NewDecoder(searchResp.Body).Decode(&search); err != nil {
		t.Fatalf("decode search: %v", err)
	}
	_ = searchResp.Body.Close()
	for _, m := range search.Matches {
		if m.ID == stubID {
			t.Fatalf("stub %q appears in search after being absorbed", stubID)
		}
	}

	// Detail redirects rather than renders.
	detailResp := authedGet(t, session, srv.URL+"/api/practices/"+practiceID+"/clients/"+stubID)
	var detail client.DetailResponse
	if err := json.NewDecoder(detailResp.Body).Decode(&detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	_ = detailResp.Body.Close()
	if detail.MergedInto == nil || *detail.MergedInto != survivorID {
		t.Fatalf("stub detail mergedInto = %v, want %q", detail.MergedInto, survivorID)
	}
}

// TestMergeHandler_BothUnattachedOlderSurvives proves ADR-0017's
// amendment: "direction never depends on which record is open... where
// both are unattached the older row survives." Run both ways round --
// once editing the older row, once editing the younger -- to prove the
// outcome depends on age, not on which id is on the path.
func TestMergeHandler_BothUnattachedOlderSurvives(t *testing.T) {
	db := testdb.New(t)

	t.Run("editing the younger record", func(t *testing.T) {
		practiceID := seedStaffWithMembership(t, db, "staff-merge-younger-open")
		olderID := seedClient(t, db, practiceID, "Robin Ellis", "")
		setCreatedAt(t, db, olderID, "2026-01-01T00:00:00Z")
		youngerID := seedClient(t, db, practiceID, "Robin", "")
		setCreatedAt(t, db, youngerID, "2026-06-01T00:00:00Z")

		srv, session := newServer(t, db, "staff-merge-younger-open")
		defer srv.Close()

		resp := authedJSON(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/clients/"+youngerID+"/merge",
			client.MergeRequest{Record: client.Record{GivenName: "Robin"}, OtherClientID: olderID})
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
		var out client.Record
		_ = json.NewDecoder(resp.Body).Decode(&out)
		if out.ID != olderID {
			t.Fatalf("survivor = %q, want the older record %q", out.ID, olderID)
		}
		if got := mergedIntoOf(t, db, youngerID); got == nil || *got != olderID {
			t.Fatalf("younger merged_into = %v, want %q", got, olderID)
		}
	})

	t.Run("editing the older record", func(t *testing.T) {
		practiceID := seedStaffWithMembership(t, db, "staff-merge-older-open")
		olderID := seedClient(t, db, practiceID, "Robin Ellis", "")
		setCreatedAt(t, db, olderID, "2026-01-01T00:00:00Z")
		youngerID := seedClient(t, db, practiceID, "Robin", "")
		setCreatedAt(t, db, youngerID, "2026-06-01T00:00:00Z")

		srv, session := newServer(t, db, "staff-merge-older-open")
		defer srv.Close()

		resp := authedJSON(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/clients/"+olderID+"/merge",
			client.MergeRequest{Record: client.Record{GivenName: "Robin Ellis"}, OtherClientID: youngerID})
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
		var out client.Record
		_ = json.NewDecoder(resp.Body).Decode(&out)
		if out.ID != olderID {
			t.Fatalf("survivor = %q, want the older record %q even though it is the one open for editing", out.ID, olderID)
		}
		if got := mergedIntoOf(t, db, youngerID); got == nil || *got != olderID {
			t.Fatalf("younger merged_into = %v, want %q", got, olderID)
		}
	})
}

// TestMergeHandler_RefusesAttachedSource proves "This is her" is offered
// only while the record open for editing is unattached -- an attached
// source is refused even though the other record would happily be
// absorbed the other way.
func TestMergeHandler_RefusesAttachedSource(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-merge-attached-source"
	practiceID := seedStaffWithMembership(t, db, identityUID)
	attachedID, _ := seedClientEngagement(t, db, practiceID, "Cora James", "cora@example.com")
	otherID := seedClient(t, db, practiceID, "Cora", "")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedJSON(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/clients/"+attachedID+"/merge",
		client.MergeRequest{Record: client.Record{GivenName: "Cora James"}, OtherClientID: otherID})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
	if got := mergedIntoOf(t, db, otherID); got != nil {
		t.Fatalf("other merged_into = %v, want nil -- nothing should have been written", got)
	}
}

// TestMergeHandler_RefusesErasedTarget proves ADR-0027's rule survives
// into the merge: an erased Client is never a merge target.
func TestMergeHandler_RefusesErasedTarget(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-merge-erased-target"
	practiceID := seedStaffWithMembership(t, db, identityUID)
	erasedID := seedClient(t, db, practiceID, "Erased Client", "")
	if _, err := db.Admin.ExecContext(t.Context(), `UPDATE clients SET erased_at = now() WHERE id = $1`, erasedID); err != nil {
		t.Fatalf("seed erased_at: %v", err)
	}
	stubID := seedClient(t, db, practiceID, testMaya, "")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedJSON(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/clients/"+stubID+"/merge",
		client.MergeRequest{Record: client.Record{GivenName: testMaya}, OtherClientID: erasedID})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
}

// TestMergeHandler_RefusesSelfMerge proves a Client cannot be merged into
// herself.
func TestMergeHandler_RefusesSelfMerge(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-merge-self"
	practiceID := seedStaffWithMembership(t, db, identityUID)
	soloID := seedClient(t, db, practiceID, "Solo Client", "")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedJSON(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/clients/"+soloID+"/merge",
		client.MergeRequest{Record: client.Record{GivenName: "Solo Client"}, OtherClientID: soloID})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
}

// TestMergeHandler_RefusesErasedSource proves ADR-0027's edit refusal
// extends to the merge endpoint: an erased Client's own record cannot be
// the one absorbed either.
func TestMergeHandler_RefusesErasedSource(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-merge-erased-source"
	practiceID := seedStaffWithMembership(t, db, identityUID)
	erasedID := seedClient(t, db, practiceID, "Erased Client", "")
	if _, err := db.Admin.ExecContext(t.Context(), `UPDATE clients SET erased_at = now() WHERE id = $1`, erasedID); err != nil {
		t.Fatalf("seed erased_at: %v", err)
	}
	otherID := seedClient(t, db, practiceID, "Other", "")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	resp := authedJSON(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/clients/"+erasedID+"/merge",
		client.MergeRequest{Record: client.Record{GivenName: "Erased Client"}, OtherClientID: otherID})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}
}

// TestMergeHandler_InvalidInput covers MergeHandler's own request-shape
// refusals: a malformed path id, an unparseable body, a blank given name,
// and a malformed OtherClientID -- four independent 400s before any
// database write is attempted.
func TestMergeHandler_InvalidInput(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-merge-invalid-input"
	practiceID := seedStaffWithMembership(t, db, identityUID)
	stubID := seedClient(t, db, practiceID, testStub, "")
	otherID := seedClient(t, db, practiceID, "Other", "")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	t.Run("invalid client id", func(t *testing.T) {
		resp := authedJSON(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/clients/not-a-uuid/merge",
			client.MergeRequest{Record: client.Record{GivenName: testStub}, OtherClientID: otherID})
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}
	})

	t.Run("invalid body", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/clients/"+stubID+"/merge", strings.NewReader("{not json"))
		req.Header.Set("Content-Type", "application/json")
		authntest.AddSessionCookie(req, session)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}
	})

	t.Run("blank given name", func(t *testing.T) {
		resp := authedJSON(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/clients/"+stubID+"/merge",
			client.MergeRequest{Record: client.Record{GivenName: "  "}, OtherClientID: otherID})
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}
	})

	t.Run("invalid other client id", func(t *testing.T) {
		resp := authedJSON(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/clients/"+stubID+"/merge",
			client.MergeRequest{Record: client.Record{GivenName: testStub}, OtherClientID: "not-a-uuid"})
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}
	})
}

// TestMergeHandler_NotFound covers the four independent 404s: a
// nonexistent source, a nonexistent target, and -- for a contractor
// Doula, whose CanAccessClient is attachment-narrowed -- an
// otherwise-valid source or target she simply cannot reach.
func TestMergeHandler_NotFound(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedStaffWithMembership(t, db, "staff-merge-not-found")
	stubID := seedClient(t, db, practiceID, testStub, "")
	otherID := seedClient(t, db, practiceID, "Other", "")
	const missingID = "00000000-0000-0000-0000-000000000000"

	srv, session := newServer(t, db, "staff-merge-not-found")
	defer srv.Close()

	t.Run("nonexistent source", func(t *testing.T) {
		resp := authedJSON(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/clients/"+missingID+"/merge",
			client.MergeRequest{Record: client.Record{GivenName: "Nobody"}, OtherClientID: otherID})
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
		}
	})

	t.Run("nonexistent target", func(t *testing.T) {
		resp := authedJSON(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/clients/"+stubID+"/merge",
			client.MergeRequest{Record: client.Record{GivenName: testStub}, OtherClientID: missingID})
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
		}
	})

	const contractorUID = "contractor-merge-not-found"
	seedContractorAtPractice(t, db, practiceID, contractorUID)
	contractorSrv, contractorSession := newServer(t, db, contractorUID)
	defer contractorSrv.Close()

	t.Run("contractor cannot reach the source", func(t *testing.T) {
		resp := authedJSON(t, contractorSession, http.MethodPost, contractorSrv.URL+"/api/practices/"+practiceID+"/clients/"+stubID+"/merge",
			client.MergeRequest{Record: client.Record{GivenName: testStub}, OtherClientID: otherID})
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
		}
	})

	t.Run("contractor cannot reach the target", func(t *testing.T) {
		attachedID, engagementID := seedClientEngagement(t, db, practiceID, "Attached To Contractor", "")
		contractorStaffID := seedContractorAtPractice(t, db, practiceID, "contractor-merge-target-not-found")
		seedGrantedAttachment(t, db, engagementID, contractorStaffID)
		reachingSrv, reachingSession := newServer(t, db, "contractor-merge-target-not-found")
		defer reachingSrv.Close()

		resp := authedJSON(t, reachingSession, http.MethodPost, reachingSrv.URL+"/api/practices/"+practiceID+"/clients/"+attachedID+"/merge",
			client.MergeRequest{Record: client.Record{GivenName: "Attached To Contractor"}, OtherClientID: otherID})
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
		}
	})
}

// TestMergeHandler_ChangedEmailRevokesPendingInviteAndOverlaysFieldValues
// covers the survivor's own email-change branch (mirroring edit.go's
// rule) and mergeFieldValues' overlay loop -- both need the absorbed
// record to carry a real, non-blank value the plain zero-value fixtures
// used elsewhere in this file never exercise.
func TestMergeHandler_ChangedEmailRevokesPendingInviteAndOverlaysFieldValues(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-merge-email-fields"
	practiceID := seedStaffWithMembership(t, db, identityUID)
	survivorID, _ := seedClientEngagement(t, db, practiceID, "Nia Okafor", "old@example.com")
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE clients SET field_values = '{"insuranceProvider":"Old Co"}'::jsonb WHERE id = $1`, survivorID,
	); err != nil {
		t.Fatalf("seed survivor field_values: %v", err)
	}
	outboxID := seedPendingOutboxRow(t, db, survivorID)
	stubID := seedClient(t, db, practiceID, "Nia", "")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	// The absorbed side is what MergeHandler was sent in this request --
	// the stub's freshly typed, not-yet-saved values -- not whatever
	// happens to already be on the stub's own row.
	resp := authedJSON(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/clients/"+stubID+"/merge",
		client.MergeRequest{
			Record:        client.Record{GivenName: "Nia", Email: testNewEmail, FieldValues: json.RawMessage(`{"referralSource":"Hospital"}`)},
			OtherClientID: survivorID,
		})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want %d, body=%s", resp.StatusCode, http.StatusOK, body)
	}
	var out client.Record
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Email != testNewEmail {
		t.Fatalf("email = %q, want the absorbed record's non-blank value to win", out.Email)
	}
	var fieldValues map[string]string
	if err := json.Unmarshal(out.FieldValues, &fieldValues); err != nil {
		t.Fatalf("unmarshal fieldValues: %v", err)
	}
	if fieldValues["insuranceProvider"] != "Old Co" {
		t.Fatalf("fieldValues[insuranceProvider] = %q, want the survivor's own value kept", fieldValues["insuranceProvider"])
	}
	if fieldValues["referralSource"] != "Hospital" {
		t.Fatalf("fieldValues[referralSource] = %q, want the absorbed record's key layered in", fieldValues["referralSource"])
	}

	status, lastErr := outboxStatus(t, db, outboxID)
	if status != deadLetteredStatus || lastErr == "" {
		t.Fatalf("outbox row status = %q lastError = %q, want dead_lettered with a reason -- the email change must revoke it", status, lastErr)
	}
}

// TestMergeHandler_RefusesChainedAndRepeatedMerge proves no chain (a
// merge target that is itself already absorbed) and the retry-safety the
// route's idempotency exemption promises: merging the same source twice
// 409s the second time rather than absorbing it again.
func TestMergeHandler_RefusesChainedAndRepeatedMerge(t *testing.T) {
	db := testdb.New(t)
	const identityUID = "staff-merge-chain"
	practiceID := seedStaffWithMembership(t, db, identityUID)
	survivorID, _ := seedClientEngagement(t, db, practiceID, "Root Survivor", "root@example.com")
	firstStubID := seedClient(t, db, practiceID, "First Stub", "")
	secondStubID := seedClient(t, db, practiceID, "Second Stub", "")

	srv, session := newServer(t, db, identityUID)
	defer srv.Close()

	first := authedJSON(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/clients/"+firstStubID+"/merge",
		client.MergeRequest{Record: client.Record{GivenName: "First Stub"}, OtherClientID: survivorID})
	defer first.Body.Close()
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first merge status = %d, want %d", first.StatusCode, http.StatusOK)
	}

	// Chained: firstStubID is now a tombstone, refused as a target.
	chained := authedJSON(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/clients/"+secondStubID+"/merge",
		client.MergeRequest{Record: client.Record{GivenName: "Second Stub"}, OtherClientID: firstStubID})
	defer chained.Body.Close()
	if chained.StatusCode != http.StatusConflict {
		t.Fatalf("chained merge status = %d, want %d", chained.StatusCode, http.StatusConflict)
	}

	// Repeated: firstStubID (now a tombstone) cannot be merged again.
	repeated := authedJSON(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/clients/"+firstStubID+"/merge",
		client.MergeRequest{Record: client.Record{GivenName: "First Stub"}, OtherClientID: survivorID})
	defer repeated.Body.Close()
	if repeated.StatusCode != http.StatusConflict {
		t.Fatalf("repeated merge status = %d, want %d", repeated.StatusCode, http.StatusConflict)
	}
}
