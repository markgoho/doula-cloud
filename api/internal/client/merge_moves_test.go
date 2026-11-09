package client_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
	"doula-cloud/api/internal/testdb"
)

// clientIDOfEngagement reads an Engagement's client_id off the superuser
// connection -- the fact the whole of #813 turns on, read independently
// of anything an endpoint chooses to report.
func clientIDOfEngagement(t *testing.T, db *testdb.DB, engagementID string) string {
	t.Helper()
	return testdb.Scalar[string](t, db.Admin, `SELECT client_id FROM engagements WHERE id = $1`, engagementID)
}

func clientIDOfRequest(t *testing.T, db *testdb.DB, requestID string) string {
	t.Helper()
	return testdb.Scalar[string](t, db.Admin, `SELECT client_id FROM engagement_requests WHERE id = $1`, requestID)
}

func clientIDOfPortalUser(t *testing.T, db *testdb.DB, identityUID string) string {
	t.Helper()
	return testdb.Scalar[string](t, db.Admin, `SELECT client_id FROM client_portal_users WHERE identity_uid = $1`, identityUID)
}

// hasDataKey answers whether clientID's data key still exists -- what
// separates "the merge left both sealed histories alone" from "the merge
// quietly shredded one".
func hasDataKey(t *testing.T, db *testdb.DB, clientID string) bool {
	t.Helper()
	return testdb.Scalar[bool](t, db.Admin, `SELECT EXISTS(SELECT 1 FROM client_data_keys WHERE client_id = $1)`, clientID)
}

func seedApprovedRequest(t *testing.T, db *testdb.DB, practiceID, clientID, engagementID, staffID, kind string) string {
	t.Helper()
	return testdb.Scalar[string](t, db.Admin,
		`INSERT INTO engagement_requests (practice_id, client_id, kind, state, requested_by, decided_by, decided_at, engagement_id)
		 VALUES ($1, $2, $3, 'approved', $4, $4, now(), $5) RETURNING id`,
		practiceID, clientID, kind, staffID, engagementID)
}

// markEnteredInError ends an Engagement as "This Engagement should never
// have existed" (00090's ending_reason), which is the Practice's own way
// of saying one pregnancy was typed twice.
func (e *env) markEnteredInError(engagementID string) {
	e.T.Helper()
	e.Exec(`UPDATE engagements SET status = 'completed', ending_reason = 'entered_in_error', birth_outcome = 'unknown' WHERE id = $1`, engagementID)
}

// twoRecordsOfOneWoman is a survivor with an active Engagement and a later
// Client typed for her again, each with an Engagement of its own.
func (e *env) twoRecordsOfOneWoman(name string, absorbedKind string) (survivorID, absorbedID, absorbedEngagementID string) {
	e.T.Helper()
	survivorID = e.attachedClient(name, "first@example.com")
	absorbedID, absorbedEngagementID = testdb.SeedEngagementWithKind(e.T, e.DB, e.Practice.ID, name, "second@example.com", absorbedKind)
	e.setCreatedAt(survivorID, "2024-01-01T00:00:00Z")
	e.setCreatedAt(absorbedID, "2025-01-01T00:00:00Z")
	return survivorID, absorbedID, absorbedEngagementID
}

// erasure erases clientID as the person at the screen.
func (e *env) erasure(clientID string) client.ErasureResponse {
	e.T.Helper()
	return reqtest.Expect[client.ErasureResponse](e.T, http.MethodPost, e.clientURL(clientID, "/erasure"), http.StatusOK, e.SignedIn)
}

// TestMergeHandler_MovesEngagementsRequestsAndPortalLinks is the whole
// point of #813: two records that both carry history become one, and
// everything that follows the woman moves to the survivor. It is also
// the test that proves 00112's narrowed engagements_freeze_outcome
// trigger admits the merge's central write -- before that migration the
// database refused it outright, whatever GLOSSARY.md said.
func TestMergeHandler_MovesEngagementsRequestsAndPortalLinks(t *testing.T) {
	e := newEnv(t)
	survivorID, absorbedID, absorbedEngagementID := e.twoRecordsOfOneWoman("Rosa Lind", "postpartum")
	requestID := seedApprovedRequest(t, e.DB, e.Practice.ID, absorbedID, absorbedEngagementID, e.Staff.ID, "postpartum")
	testdb.SeedPortalUser(t, e.DB, "portal-rosa-second", absorbedID)

	out := e.mergeOK(absorbedID, client.MergeRequest{GivenName: "Rosa Lind", Email: "second@example.com", OtherClientID: survivorID})

	if out.MergedFrom != absorbedID {
		t.Fatalf("mergedFrom = %q, want %q", out.MergedFrom, absorbedID)
	}
	if out.Moved.Engagements != 1 || out.Moved.EngagementRequests != 1 || out.Moved.PortalAccounts != 1 {
		t.Fatalf("moved = %+v, want one of each -- a silent zero is exactly what the row counts exist to catch", out.Moved)
	}
	if got := clientIDOfEngagement(t, e.DB, absorbedEngagementID); got != survivorID {
		t.Fatalf("engagement client_id = %q, want the survivor %q", got, survivorID)
	}
	if got := clientIDOfRequest(t, e.DB, requestID); got != survivorID {
		t.Fatalf("request client_id = %q, want the survivor %q -- an approved Request follows its Engagement", got, survivorID)
	}
	if got := clientIDOfPortalUser(t, e.DB, "portal-rosa-second"); got != survivorID {
		t.Fatalf("portal link client_id = %q, want the survivor %q", got, survivorID)
	}

	// The plaintext audit, which has to outlive a shredding of both keys.
	if mergedAt := testdb.Scalar[*time.Time](t, e.DB.Admin, `SELECT merged_at FROM clients WHERE id = $1`, absorbedID); mergedAt == nil {
		t.Fatalf("merged_at = nil, want a timestamp -- who merged this and when must survive an erasure")
	}
	if mergedBy := testdb.Scalar[*string](t, e.DB.Admin, `SELECT merged_by_staff_id FROM clients WHERE id = $1`, absorbedID); mergedBy == nil || *mergedBy != e.Staff.ID {
		t.Fatalf("merged_by_staff_id = %v, want %q", mergedBy, e.Staff.ID)
	}

	// Both keys survive: the two sealed histories may never fold into one.
	if !hasDataKey(t, e.DB, absorbedID) || !hasDataKey(t, e.DB, survivorID) {
		t.Fatalf("a data key was destroyed by the merge -- ADR-0027 keeps both until an erasure shreds them")
	}
}

// TestEngagementFreezeStillRefusesAnUntombstonedMove proves the freeze
// was narrowed rather than dropped. The refusal that #813 had to get
// past lives in the database (00093's engagements_freeze_outcome), and
// the only thing that opens it is a durable tombstone -- not a session
// flag, and not a caller's say-so.
func TestEngagementFreezeStillRefusesAnUntombstonedMove(t *testing.T) {
	e := newEnv(t)
	_, engagementID := testdb.SeedNamedEngagement(t, e.DB, e.Practice.ID, "Held Fast", "held@example.com")
	otherID := e.newClient("Somebody Else")

	_, err := e.DB.Admin.ExecContext(t.Context(), `UPDATE engagements SET client_id = $2 WHERE id = $1`, engagementID, otherID)
	if err == nil {
		t.Fatalf("moving an Engagement with no tombstone succeeded, want the trigger's refusal")
	}
	if got := clientIDOfEngagement(t, e.DB, engagementID); got == otherID {
		t.Fatalf("engagement moved to %q anyway", otherID)
	}
}

// TestDetailHandler_MergedClientShowsBothHistoriesAsTwo proves the
// constraint #813's decision comment puts hardest: the screen may not
// imply one history. ADR-0027 seals each Client's diffs under her own
// key and ADR-0022's activity table is append-only, so the absorbed
// woman's entries can never be re-sealed under the survivor's -- they
// are read under their own key and labeled with the record they were
// written against.
func TestDetailHandler_MergedClientShowsBothHistoriesAsTwo(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	survivorID, absorbedID, absorbedEngagementID := e.twoRecordsOfOneWoman("Nell Frost", "postpartum")
	seedApprovedRequest(t, e.DB, e.Practice.ID, absorbedID, absorbedEngagementID, e.Staff.ID, "postpartum")

	e.merge(absorbedID, client.MergeRequest{GivenName: "Nell Frost", OtherClientID: survivorID}, http.StatusOK)
	out := reqtest.Expect[client.DetailResponse](t, http.MethodGet, e.clientURL(survivorID, ""), http.StatusOK, e.SignedIn)

	if len(out.MergedFrom) != 1 || out.MergedFrom[0].ClientID != absorbedID {
		t.Fatalf("mergedFrom = %+v, want the one absorbed record %q", out.MergedFrom, absorbedID)
	}
	if out.MergedFrom[0].MergedByName == nil {
		t.Fatalf("mergedFrom[0].mergedByName = nil -- who merged this must be answerable, and in plaintext")
	}

	var labeled, unlabeled int
	for _, h := range out.History {
		if h.ClientEvent == nil {
			continue
		}
		if h.FromMergedRecord != nil {
			if *h.FromMergedRecord != absorbedID {
				t.Fatalf("fromMergedRecord = %q, want %q", *h.FromMergedRecord, absorbedID)
			}
			labeled++
		} else {
			unlabeled++
		}
	}
	if labeled == 0 || unlabeled == 0 {
		t.Fatalf("history has %d labeled and %d unlabeled entries, want both -- two trails shown as two", labeled, unlabeled)
	}
}

// TestEraseHandler_ReachesTheAbsorbedRecord proves the other half of
// that same constraint. The merge deliberately leaves the absorbed
// record's data key standing, so it is the survivor's erasure that has
// to shred it -- and the tombstone still holds her name until something
// redacts it, which no ordinary write can, because clients_update
// refuses a merged row outright.
func TestEraseHandler_ReachesTheAbsorbedRecord(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	survivorID := e.attachedClient("Vera Ash", "vera@example.com")
	absorbedID := seedFullClient(t, e.DB, e.Practice.ID, e.Staff.ID)
	// A Stripe Customer allocated against the absorbed record before the
	// merge. It cannot move -- client_stripe_customers carries no UPDATE
	// grant (00076) -- so the survivor's erasure is the only thing that
	// will ever delete it, and its 90-day redaction floor has to reach
	// the date the erasure reports.
	seedMappedCustomer(t, e.DB, e.Practice.ID, absorbedID, "acct_absorbed", "cus_absorbed", 0)

	e.merge(absorbedID, client.MergeRequest{GivenName: "Vera Ash", OtherClientID: survivorID}, http.StatusOK)
	erasure := e.erasure(survivorID)

	if erasure.StripeCustomersQueued != 1 {
		t.Fatalf("stripeCustomersQueued = %d, want 1 -- the absorbed record's Customer is only ever reached from here", erasure.StripeCustomersQueued)
	}
	if erasure.StripeRedactionEligibleAt == nil {
		t.Fatalf("stripeRedactionEligibleAt = nil, want the absorbed record's own 90-day floor")
	}

	var givenName string
	var email *string
	var erasedAt *time.Time
	var mergedInto *string
	var mergedAt *time.Time
	if err := e.DB.Admin.QueryRowContext(t.Context(),
		`SELECT given_name, email, erased_at, merged_into, merged_at FROM clients WHERE id = $1`, absorbedID,
	).Scan(&givenName, &email, &erasedAt, &mergedInto, &mergedAt); err != nil {
		t.Fatalf("read absorbed record: %v", err)
	}
	if givenName != client.ErasedGivenName || email != nil {
		t.Fatalf("absorbed record = %q / %v, want it redacted -- an erasure that misses the tombstone leaves her name standing forever", givenName, email)
	}
	if erasedAt == nil {
		t.Fatalf("absorbed erased_at = nil, want a timestamp")
	}
	if mergedInto == nil || *mergedInto != survivorID || mergedAt == nil {
		t.Fatalf("merge audit was destroyed by the erasure -- it is plaintext precisely so it outlives one")
	}
	if hasDataKey(t, e.DB, absorbedID) {
		t.Fatalf("absorbed record's data key survived the erasure -- her second sealed trail is still readable")
	}
	if hasDataKey(t, e.DB, survivorID) {
		t.Fatalf("survivor's data key survived her own erasure")
	}

	// The screen's own answer afterwards. Both trails are unreadable, and
	// the plaintext merge audit is what still explains why there are two.
	out := reqtest.Expect[client.DetailResponse](t, http.MethodGet, e.clientURL(survivorID, ""), http.StatusOK, e.SignedIn)
	if len(out.MergedFrom) != 1 {
		t.Fatalf("mergedFrom = %+v, want the absorbed record still named after the erasure", out.MergedFrom)
	}
	if out.MergedFrom[0].ErasedAt == nil {
		t.Fatalf("mergedFrom[0].erasedAt = nil -- the screen must not imply the absorbed record is still live")
	}
	if out.MergedFrom[0].MergedByName == nil {
		t.Fatalf("mergedFrom[0].mergedByName = nil -- who merged this must outlive the shredding of both keys")
	}
}

// TestMergeHandler_RevokesTheAbsorbedRecordsPendingInvitation proves the
// one attachment the merge does not move. An invitation is re-sendable,
// moving it would collide with client_portal_users_one_pending_per_client
// (00026) whenever the survivor holds her own, and the address it was
// sent to may not even be the address the fold kept.
func TestMergeHandler_RevokesTheAbsorbedRecordsPendingInvitation(t *testing.T) {
	e := newEnv(t)
	survivorID := e.attachedClient("Della Roe", "della@example.com")
	absorbedID, _ := testdb.SeedNamedEngagement(t, e.DB, e.Practice.ID, "Della Roe", "della2@example.com")
	testdb.SeedPendingPortalInvite(t, e.DB, absorbedID)
	e.setCreatedAt(survivorID, "2024-01-01T00:00:00Z")
	e.setCreatedAt(absorbedID, "2025-01-01T00:00:00Z")

	out := e.mergeOK(absorbedID, client.MergeRequest{GivenName: "Della Roe", OtherClientID: survivorID})

	if out.Moved.RevokedInvitations != 1 || out.Moved.PortalAccounts != 0 {
		t.Fatalf("moved = %+v, want one revoked invitation and no moved account", out.Moved)
	}
	if token := testdb.Scalar[*string](t, e.DB.Admin, `SELECT invite_token FROM client_portal_users WHERE client_id = $1`, absorbedID); token != nil {
		t.Fatalf("invite_token = %v, want nil -- an invitation to a record that no longer exists must not stay redeemable", *token)
	}
	if status := testdb.Scalar[string](t, e.DB.Admin,
		`SELECT o.status FROM portal_invite_outbox o
		   JOIN client_portal_users pu ON pu.id = o.client_portal_user_id
		  WHERE pu.client_id = $1`, absorbedID); status == pendingStatus {
		t.Fatalf("outbox status = pending, want the send stopped -- the email would name a record that is now a tombstone")
	}
}

// TestDefinerFunctionsRefuseWithoutTheirPrecondition drives the two
// SECURITY DEFINER functions directly, as app_runtime, rather than
// through the endpoint. That is the point: they are the two doors that
// can write what no Staff-facing policy admits -- moving an accepted
// portal link, and writing to a tombstone at all -- so whether they
// refuse must be asserted at the SQL surface. Anything holding
// app_runtime can call them, and "MergeHandler's ordering makes this
// unreachable" is a claim about one caller, not about the function.
func TestDefinerFunctionsRefuseWithoutTheirPrecondition(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	survivorID := e.attachedClient("Kept Record", "kept@example.com")
	otherID, _ := testdb.SeedNamedEngagement(t, e.DB, e.Practice.ID, "Other Record", "other@example.com")
	testdb.SeedPortalUser(t, e.DB, "portal-definer-refusals", otherID)

	// One transaction per refusal: a RAISE aborts the transaction it fires
	// in, so a second call on the same one reports only that the block is
	// already dead rather than what the function itself would have said.

	// No tombstone: the two records are simply two records.
	var moved int
	tx := e.DB.BeginAs(t, testdb.Practice(e.Practice.ID))
	err := tx.QueryRowContext(t.Context(), `SELECT merge_client_portal_links($1, $2)`, otherID, survivorID).Scan(&moved)
	_ = tx.Rollback()
	if err == nil {
		t.Fatalf("merge_client_portal_links moved %d links with no tombstone, want a refusal", moved)
	}
	if !strings.Contains(err.Error(), "already merged into the destination") {
		t.Fatalf("error = %v, want the function's own refusal", err)
	}
	if got := clientIDOfPortalUser(t, e.DB, "portal-definer-refusals"); got != otherID {
		t.Fatalf("portal link moved to %q anyway", got)
	}

	// Nor may a tombstone be redacted while the record it was merged into
	// is still live -- that is what keeps this from being a delete button
	// on somebody else's Client.
	tx = e.DB.BeginAs(t, testdb.Practice(e.Practice.ID))
	_, err = tx.ExecContext(t.Context(), `SELECT redact_absorbed_client($1)`, otherID)
	_ = tx.Rollback()
	if err == nil {
		t.Fatalf("redact_absorbed_client redacted a live record, want a refusal")
	}
	if !strings.Contains(err.Error(), "after the record it was merged into has been erased") {
		t.Fatalf("error = %v, want the function's own refusal", err)
	}
}

// TestRedactAbsorbedClientWritesTheSameNameErasureDoes ties the name the
// definer function hardcodes to the Go constant every other erasure path
// writes. The function owns the value rather than taking it from a
// caller -- it is the one door onto a tombstone, so a chosen name would
// be a door onto a chosen write -- and this is what keeps the two from
// drifting into an erased record that reads differently depending on
// which path erased it.
func TestRedactAbsorbedClientWritesTheSameNameErasureDoes(t *testing.T) {
	e := newEnv(t, signedInAs(fixture.Owner()))
	survivorID := e.attachedClient("Kept Record", "kept@example.com")
	absorbedID := seedFullClient(t, e.DB, e.Practice.ID, e.Staff.ID)

	e.merge(absorbedID, client.MergeRequest{GivenName: "Kept Record", OtherClientID: survivorID}, http.StatusOK)
	e.erasure(survivorID)

	for _, id := range []string{absorbedID, survivorID} {
		if name := testdb.Scalar[string](t, e.DB.Admin, `SELECT given_name FROM clients WHERE id = $1`, id); name != client.ErasedGivenName {
			t.Fatalf("name of %s = %q, want %q -- the definer function and Go must write the same word", id, name, client.ErasedGivenName)
		}
	}
}

// TestMergeHandler_RefusesTwoPendingRequestsOfOneKind proves the merge
// asks before it writes. engagement_requests_one_pending (00042) is a
// unique index on (client_id, kind) where state = 'pending', so moving
// one pending Request onto a record that already holds one of that kind
// would raise a constraint violation the endpoint could only report as
// a 500.
func TestMergeHandler_RefusesTwoPendingRequestsOfOneKind(t *testing.T) {
	e := newEnv(t)
	survivorID := e.attachedClient("Wren Ash", "wren@example.com")
	otherID := testdb.SeedNamedClient(t, e.DB, e.Practice.ID, "Wren Ash", "wren2@example.com")
	seedPendingRequest(t, e.DB, e.Practice.ID, survivorID, e.Staff.ID, "birth")
	seedPendingRequest(t, e.DB, e.Practice.ID, otherID, e.Staff.ID, "birth")

	e.merge(otherID, client.MergeRequest{GivenName: "Wren Ash", OtherClientID: survivorID}, http.StatusConflict)

	if got := mergedIntoOf(t, e.DB, otherID); got != nil {
		t.Fatalf("merged_into = %v, want nil -- the refusal must be asked before anything is written", got)
	}
}

// TestMergeHandler_EnteredInErrorDoesNotCountAsAttached proves the
// second half of #813's work. "This Engagement should never have
// existed" (00090) is the Practice saying one pregnancy was typed
// twice, so the record holding only that is the one absorbed rather
// than the one that wins -- and the approved Request that produced the
// Engagement is discounted alongside it, since an Engagement exists
// only because a Request was approved and discounting the Engagement
// alone would have changed nothing.
func TestMergeHandler_EnteredInErrorDoesNotCountAsAttached(t *testing.T) {
	e := newEnv(t)
	realID := e.attachedClient("Iris Vale", "iris@example.com")
	typoID, typoEngagementID := testdb.SeedNamedEngagement(t, e.DB, e.Practice.ID, "Iris Vale", "iris2@example.com")
	seedApprovedRequest(t, e.DB, e.Practice.ID, typoID, typoEngagementID, e.Staff.ID, "birth")
	e.markEnteredInError(typoEngagementID)
	// The typo record is deliberately the OLDER of the two, so the
	// older-survives tiebreak would keep it if the discount did not
	// apply. Attachment is the only thing that can decide this one.
	e.setCreatedAt(realID, "2025-06-01T00:00:00Z")
	e.setCreatedAt(typoID, "2024-06-01T00:00:00Z")

	e.merge(typoID, client.MergeRequest{GivenName: "Iris Vale", OtherClientID: realID}, http.StatusOK)

	requireMergedInto(t, e.DB, typoID, realID)
}
