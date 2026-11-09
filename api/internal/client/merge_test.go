package client_test

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"net/http"
	"testing"

	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

const (
	missingID   = "00000000-0000-0000-0000-000000000000"
	notAUUID    = "not-a-uuid"
	olderCreate = "2026-01-01T00:00:00Z"
	youngCreate = "2026-06-01T00:00:00Z"
)

// mergedIntoOf reads clientID's merged_into column straight off the
// superuser connection -- the tombstone fact itself, independent of
// anything DetailHandler chooses to report.
func mergedIntoOf(t *testing.T, db *testdb.DB, clientID string) *string {
	t.Helper()
	return testdb.Scalar[*string](t, db.Admin, `SELECT merged_into FROM clients WHERE id = $1`, clientID)
}

// activityActionsFor reads every activity row's action for clientID,
// oldest first -- what proves ADR-0017's amendment "recorded on both
// rows" without needing to unseal a diff.
func activityActionsFor(t *testing.T, db *testdb.DB, clientID string) []string {
	t.Helper()
	return testdb.Column[string](t, db.Admin,
		`SELECT action FROM activity WHERE subject_kind = 'client' AND subject_id = $1 ORDER BY created_at`, clientID)
}

// setCreatedAt backdates/forwards clientID's created_at, for tests that
// need a deterministic older/younger ordering between two rows without
// sleeping between two real inserts.
func (e *env) setCreatedAt(clientID, createdAt string) {
	e.T.Helper()
	e.Exec(`UPDATE clients SET created_at = $2 WHERE id = $1`, clientID, createdAt)
}

// requireMergedInto fails unless clientID was absorbed into survivorID.
func requireMergedInto(t *testing.T, db *testdb.DB, clientID, survivorID string) {
	t.Helper()
	if got := mergedIntoOf(t, db, clientID); got == nil || *got != survivorID {
		t.Fatalf("merged_into of %s = %v, want %q", clientID, got, survivorID)
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
	e := newEnv(t)
	survivorID := e.attachedClient("Maya Torres", "old@example.com")
	stubID := e.newClient(testMaya)
	e.Exec(`UPDATE clients SET phone = '555-0142' WHERE id = $1`, stubID)

	out := e.mergeOK(stubID, client.MergeRequest{GivenName: testMaya, Phone: "555-0142", OtherClientID: survivorID})
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

	requireMergedInto(t, e.DB, stubID, survivorID)
	if got := mergedIntoOf(t, e.DB, survivorID); got != nil {
		t.Fatalf("survivor merged_into = %v, want nil -- the survivor is never itself tombstoned", got)
	}

	survivorActions := activityActionsFor(t, e.DB, survivorID)
	if len(survivorActions) == 0 || survivorActions[len(survivorActions)-1] != "merged" {
		t.Fatalf("survivor activity = %v, want a trailing %q row", survivorActions, "merged")
	}
	stubActions := activityActionsFor(t, e.DB, stubID)
	if len(stubActions) == 0 || stubActions[len(stubActions)-1] != "absorbed" {
		t.Fatalf("stub activity = %v, want a trailing %q row", stubActions, "absorbed")
	}

	list := reqtest.Expect[client.ListResponse](t, http.MethodGet, e.PracticeURL("/clients?all=true"), http.StatusOK, e.SignedIn)
	for _, item := range list.Items {
		if item.ClientID == stubID {
			t.Fatalf("stub %q appears on the Clients list after being absorbed", stubID)
		}
	}

	search := reqtest.Expect[client.SearchResponse](t, http.MethodPost, e.PracticeURL("/clients/search"), http.StatusOK, e.SignedIn,
		reqtest.JSON(client.SearchRequest{Name: "Maya"}))
	for _, m := range search.Matches {
		if m.ID == stubID {
			t.Fatalf("stub %q appears in search after being absorbed", stubID)
		}
	}

	detail := reqtest.Expect[client.DetailResponse](t, http.MethodGet, e.clientURL(stubID, ""), http.StatusOK, e.SignedIn)
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
	for _, tc := range []struct {
		name          string
		openIsYounger bool
		givenName     string
	}{
		{"editing the younger record", true, "Robin"},
		{"editing the older record", false, "Robin Ellis"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			olderID := e.newClient("Robin Ellis")
			e.setCreatedAt(olderID, olderCreate)
			youngerID := e.newClient("Robin")
			e.setCreatedAt(youngerID, youngCreate)
			open, other := olderID, youngerID
			if tc.openIsYounger {
				open, other = youngerID, olderID
			}

			out := e.mergeOK(open, client.MergeRequest{GivenName: tc.givenName, OtherClientID: other})

			if out.ID != olderID {
				t.Fatalf("survivor = %q, want the older record %q", out.ID, olderID)
			}
			requireMergedInto(t, e.DB, youngerID, olderID)
		})
	}
}

// TestMergeHandler_AttachedSourceSurvivesAndAbsorbsTheOther proves the
// reversal #813 records (ADR-0040): an attached record open for editing
// is no longer refused. It survives, and the other record is absorbed
// into it -- attachment decides direction, not whether the question may
// be asked at all.
func TestMergeHandler_AttachedSourceSurvivesAndAbsorbsTheOther(t *testing.T) {
	e := newEnv(t)
	attachedID := e.attachedClient("Cora James", "cora@example.com")
	otherID := e.newClient("Cora")

	e.merge(attachedID, client.MergeRequest{GivenName: "Cora James", OtherClientID: otherID}, http.StatusOK)

	requireMergedInto(t, e.DB, otherID, attachedID)
	if got := mergedIntoOf(t, e.DB, attachedID); got != nil {
		t.Fatalf("attached merged_into = %v, want nil", got)
	}
}

// TestMergeHandler_Refusals covers what MergeHandler turns away before it
// writes anything: an erased Client on either side (ADR-0027), a Client
// merged into herself, the four request-shape failures, and -- for a
// contractor Doula, whose CanAccessClient is attachment-narrowed -- a record
// that does not exist or that she cannot reach.
func TestMergeHandler_Refusals(t *testing.T) {
	erase := func(e *env, name string) string {
		id := e.newClient(name)
		e.Exec(`UPDATE clients SET erased_at = now() WHERE id = $1`, id)
		return id
	}
	for _, tc := range []struct {
		name       string
		contractor bool
		want       int
		// setup returns the Client open for editing and the request sent.
		setup func(e *env) (sourceID string, body reqtest.Option)
	}{
		{"an erased target", false, http.StatusConflict, func(e *env) (string, reqtest.Option) {
			erasedID := erase(e, "Erased Client")
			return e.newClient(testMaya), reqtest.JSON(client.MergeRequest{GivenName: testMaya, OtherClientID: erasedID})
		}},
		{"an erased source", false, http.StatusConflict, func(e *env) (string, reqtest.Option) {
			erasedID := erase(e, "Erased Client")
			return erasedID, reqtest.JSON(client.MergeRequest{GivenName: "Erased Client", OtherClientID: e.newClient("Other")})
		}},
		{"a Client merged into herself", false, http.StatusConflict, func(e *env) (string, reqtest.Option) {
			soloID := e.newClient("Solo Client")
			return soloID, reqtest.JSON(client.MergeRequest{GivenName: "Solo Client", OtherClientID: soloID})
		}},
		{"an invalid client id", false, http.StatusBadRequest, func(e *env) (string, reqtest.Option) {
			return notAUUID, reqtest.JSON(client.MergeRequest{GivenName: testStub, OtherClientID: e.newClient("Other")})
		}},
		{"an invalid body", false, http.StatusBadRequest, func(e *env) (string, reqtest.Option) {
			return e.newClient(testStub), reqtest.RawJSON([]byte("{not json"))
		}},
		{"a blank given name", false, http.StatusBadRequest, func(e *env) (string, reqtest.Option) {
			return e.newClient(testStub), reqtest.JSON(client.MergeRequest{GivenName: "  ", OtherClientID: e.newClient("Other")})
		}},
		{"an invalid other client id", false, http.StatusBadRequest, func(e *env) (string, reqtest.Option) {
			return e.newClient(testStub), reqtest.JSON(client.MergeRequest{GivenName: testStub, OtherClientID: notAUUID})
		}},
		{"a nonexistent source", false, http.StatusNotFound, func(e *env) (string, reqtest.Option) {
			return missingID, reqtest.JSON(client.MergeRequest{GivenName: "Nobody", OtherClientID: e.newClient("Other")})
		}},
		{"a nonexistent target", false, http.StatusNotFound, func(e *env) (string, reqtest.Option) {
			return e.newClient(testStub), reqtest.JSON(client.MergeRequest{GivenName: testStub, OtherClientID: missingID})
		}},
		{"a source the contractor cannot reach", true, http.StatusNotFound, func(e *env) (string, reqtest.Option) {
			return e.newClient(testStub), reqtest.JSON(client.MergeRequest{GivenName: testStub, OtherClientID: e.newClient("Other")})
		}},
		{"a target the contractor cannot reach", true, http.StatusNotFound, func(e *env) (string, reqtest.Option) {
			attachedID, engagementID := testdb.SeedEngagementInStatus(e.T, e.DB, e.Practice.ID, "Attached To Contractor", "", "active")
			testdb.SeedGrantedAttachment(e.T, e.DB, engagementID, e.Staff.ID)
			return attachedID, reqtest.JSON(client.MergeRequest{GivenName: "Attached To Contractor", OtherClientID: e.newClient("Other")})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var opts []envOption
			if tc.contractor {
				opts = append(opts, signedInAs(fixture.Contractor()))
			}
			e := newEnv(t, opts...)
			sourceID, body := tc.setup(e)

			reqtest.ExpectStatus(t, http.MethodPost, e.clientURL(sourceID, "/merge"), tc.want, e.SignedIn, body)
		})
	}
}

// TestMergeHandler_ChangedEmailRevokesPendingInviteAndOverlaysFieldValues
// covers the survivor's own email-change branch (mirroring edit.go's
// rule) and mergeFieldValues' overlay loop -- both need the absorbed
// record to carry a real, non-blank value the plain zero-value fixtures
// used elsewhere in this file never exercise.
func TestMergeHandler_ChangedEmailRevokesPendingInviteAndOverlaysFieldValues(t *testing.T) {
	e := newEnv(t)
	survivorID := e.attachedClient("Nia Okafor", "old@example.com")
	e.Exec(`UPDATE clients SET field_values = '{"insuranceProvider":"Old Co"}'::jsonb WHERE id = $1`, survivorID)
	outboxID := seedPendingOutboxRow(t, e.DB, survivorID)
	stubID := e.newClient("Nia")

	// The absorbed side is what MergeHandler was sent in this request --
	// the stub's freshly typed, not-yet-saved values -- not whatever
	// happens to already be on the stub's own row.
	out := e.mergeOK(stubID, client.MergeRequest{
		GivenName: "Nia", Email: testNewEmail, FieldValues: jsontext.Value(`{"referralSource":"Hospital"}`),
		OtherClientID: survivorID,
	})
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

	status, lastErr := outboxStatus(t, e.DB, outboxID)
	if status != deadLetteredStatus || lastErr == "" {
		t.Fatalf("outbox row status = %q lastError = %q, want dead_lettered with a reason -- the email change must revoke it", status, lastErr)
	}
}

// TestMergeHandler_RefusesChainedAndRepeatedMerge proves no chain (a
// merge target that is itself already absorbed) and the retry-safety the
// route's idempotency exemption promises: merging the same source twice
// 409s the second time rather than absorbing it again.
func TestMergeHandler_RefusesChainedAndRepeatedMerge(t *testing.T) {
	e := newEnv(t)
	survivorID := e.attachedClient("Root Survivor", "root@example.com")
	firstStubID := e.newClient("First Stub")
	secondStubID := e.newClient("Second Stub")

	e.merge(firstStubID, client.MergeRequest{GivenName: "First Stub", OtherClientID: survivorID}, http.StatusOK)
	// Chained: firstStubID is now a tombstone, refused as a target.
	e.merge(secondStubID, client.MergeRequest{GivenName: "Second Stub", OtherClientID: firstStubID}, http.StatusConflict)
	// Repeated: firstStubID (now a tombstone) cannot be merged again.
	e.merge(firstStubID, client.MergeRequest{GivenName: "First Stub", OtherClientID: survivorID}, http.StatusConflict)
}
