package client_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/testdb"
)

// seedPortalFeedback inserts one piece of Feedback sent by portalUID.
// clientID is "" for a piece keyed only to the Portal Account -- sent
// from a screen with no single Engagement in it -- and otherwise rides
// with practiceID, as feedback_sender's own CHECK (00118) demands.
// issueNumber is 0 for a piece whose issue has not been opened yet.
func seedPortalFeedback(t *testing.T, db *testdb.DB, portalUID, practiceID, clientID string, issueNumber int) (feedbackID string) {
	t.Helper()
	feedbackID = uuid.NewString()
	var practice, clientRef, issue any
	if clientID != "" {
		practice, clientRef = practiceID, clientID
	}
	if issueNumber != 0 {
		issue = issueNumber
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO feedback (id, kind, text, page_url, route_id, app_build, screen_width, browser,
		     portal_account, practice_id, client_id, issue_number, sent_at)
		 VALUES ($1, 'not_working', 'the text she typed', '/portal', '/portal', 'abc1234', 390, 'Safari 18',
		     $2, $3, $4, $5, now())`,
		feedbackID, portalUID, practice, clientRef, issue,
	); err != nil {
		t.Fatalf("seed feedback: %v", err)
	}
	return feedbackID
}

// seedStaffFeedback inserts one piece sent by a Staff member from under
// practiceID -- the piece no Client's Erasure may touch.
func seedStaffFeedback(t *testing.T, db *testdb.DB, staffID, practiceID string, issueNumber int) (feedbackID string) {
	t.Helper()
	feedbackID = uuid.NewString()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO feedback (id, kind, text, page_url, route_id, app_build, screen_width, browser,
		     staff_id, practice_id, roles, issue_number, sent_at)
		 VALUES ($1, 'idea_or_request', '', '/practices/x', '/practices/[practiceId]', 'abc1234', 390, 'Safari 18',
		     $2, $3, '{owner}', $4, now())`,
		feedbackID, staffID, practiceID, issueNumber,
	); err != nil {
		t.Fatalf("seed staff feedback: %v", err)
	}
	return feedbackID
}

func feedbackExists(t *testing.T, db *testdb.DB, feedbackID string) bool {
	t.Helper()
	var exists bool
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT EXISTS (SELECT 1 FROM feedback WHERE id = $1)`, feedbackID,
	).Scan(&exists); err != nil {
		t.Fatalf("read feedback: %v", err)
	}
	return exists
}

// pendingCloseJobs returns the issue number of every pending "close as
// erased" job, in order.
func pendingCloseJobs(t *testing.T, db *testdb.DB) []int {
	t.Helper()
	rows, err := db.Admin.QueryContext(t.Context(),
		`SELECT issue_number FROM feedback_issue_outbox
		  WHERE act = 'close_erased' AND status = 'pending' ORDER BY issue_number`)
	if err != nil {
		t.Fatalf("read close jobs: %v", err)
	}
	defer rows.Close()

	numbers := []int{}
	for rows.Next() {
		var n int
		if err := rows.Scan(&n); err != nil {
			t.Fatalf("scan close job: %v", err)
		}
		numbers = append(numbers, n)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate close jobs: %v", err)
	}
	return numbers
}

func eraseOK(t *testing.T, db *testdb.DB, uid, practiceID, clientID string) {
	t.Helper()
	srv, session := newServer(t, db, uid)
	defer srv.Close()
	resp := postErasure(t, session, srv, practiceID, clientID)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("erase status = %d, want %d: %s", resp.StatusCode, http.StatusOK, readBody(t, resp))
	}
}

// TestEraseHandler_DestroysHerFeedbackAndQueuesACloseForEachIssue is
// #1525's Erasure AC: every piece she sent is deleted outright, each one
// that has an issue gets a "close as erased" job carrying that issue's
// number, and a Staff member's piece at the same Practice is not hers
// and stays. She is the only Client behind her login, so the pieces
// keyed only to the login go too, with the Portal Account row.
func TestEraseHandler_DestroysHerFeedbackAndQueuesACloseForEachIssue(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-erase-feedback"
	const portalUID = "portal-erase-feedback"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	clientID := seedFullClient(t, db, practiceID, staffID)
	testdb.SeedPortalAccount(t, db, portalUID, portalUID+"@example.com")
	testdb.AttachPortalUser(t, db, portalUID, clientID)

	withIssue := seedPortalFeedback(t, db, portalUID, practiceID, clientID, 11)
	notYetOpened := seedPortalFeedback(t, db, portalUID, practiceID, clientID, 0)
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO feedback_issue_outbox (feedback_id) VALUES ($1)`, notYetOpened,
	); err != nil {
		t.Fatalf("seed open job: %v", err)
	}
	loginOnly := seedPortalFeedback(t, db, portalUID, "", "", 12)
	staffPiece := seedStaffFeedback(t, db, staffID, practiceID, 13)
	// #1526: the founder read one of her pieces. The record of that read
	// goes with the piece (00122's ON DELETE CASCADE).
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO feedback_reads (feedback_id, staff_id, read_at) VALUES ($1, $2, now())`, withIssue, staffID,
	); err != nil {
		t.Fatalf("seed read row: %v", err)
	}

	eraseOK(t, db, uid, practiceID, clientID)

	var readRows int
	if err := db.Admin.QueryRowContext(t.Context(), `SELECT count(*) FROM feedback_reads`).Scan(&readRows); err != nil {
		t.Fatalf("count read rows: %v", err)
	}
	if readRows != 0 {
		t.Fatalf("read rows = %d, want the read of her piece destroyed with it", readRows)
	}

	for name, id := range map[string]string{"with an issue": withIssue, "not yet opened": notYetOpened, "keyed only to the login": loginOnly} {
		if feedbackExists(t, db, id) {
			t.Errorf("her piece %s is still there, want it destroyed", name)
		}
	}
	if !feedbackExists(t, db, staffPiece) {
		t.Error("a Staff member's piece is gone -- a Client's Erasure must leave it")
	}
	if got := pendingCloseJobs(t, db); !slices.Equal(got, []int{11, 12}) {
		t.Fatalf("close jobs = %v, want [11 12] -- one per erased piece that had an issue, none for the Staff piece", got)
	}

	// The open job outlives its piece: the feedback issue worker marks it
	// done on its next run, having opened nothing (the feedback package's
	// own TestIssueWorker_OpenJobWhosePieceIsGoneOpensNoIssue).
	var openJobs int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM feedback_issue_outbox WHERE act = 'open' AND feedback_id = $1 AND status = 'pending'`, notYetOpened,
	).Scan(&openJobs); err != nil {
		t.Fatalf("count open jobs: %v", err)
	}
	if openJobs != 1 {
		t.Fatalf("pending open jobs for the unopened piece = %d, want 1", openJobs)
	}

	// The Practice's own record of the act says nothing about Feedback: a
	// piece goes to DoulaCloud only and never to her Practice.
	if scope := string(erasureScopeRaw(t, db, clientID)); strings.Contains(strings.ToLower(scope), "feedback") {
		t.Fatalf("erasure scope = %s, want no mention of Feedback", scope)
	}
}

// TestEraseHandler_LoginOnlyFeedbackGoesWithTheLastPracticeOut is
// #1525's Portal Account AC, in #830's two-Practice shape. The first
// Practice to erase her takes the pieces that name its own Client and
// nothing else; the piece keyed only to the login, and the other
// Practice's, stay while the login does. The last Practice out takes the
// login, and the login-only piece goes with it, with its close job.
func TestEraseHandler_LoginOnlyFeedbackGoesWithTheLastPracticeOut(t *testing.T) {
	db := testdb.New(t)
	const uidA = "owner-erase-feedback-shared-a"
	const uidB = "owner-erase-feedback-shared-b"
	const portalUID = "portal-erase-feedback-shared"
	practiceA, staffA := testdb.SeedStaffAtNewPractice(t, db, uidA, []string{ownerRole}, "employee")
	practiceB, staffB := testdb.SeedStaffAtNewPractice(t, db, uidB, []string{ownerRole}, "employee")
	clientA := seedFullClient(t, db, practiceA, staffA)
	clientB := seedFullClient(t, db, practiceB, staffB)
	testdb.SeedPortalAccount(t, db, portalUID, portalUID+"@example.com")
	testdb.AttachPortalUser(t, db, portalUID, clientA)
	testdb.AttachPortalUser(t, db, portalUID, clientB)

	pieceA := seedPortalFeedback(t, db, portalUID, practiceA, clientA, 21)
	pieceB := seedPortalFeedback(t, db, portalUID, practiceB, clientB, 22)
	loginOnly := seedPortalFeedback(t, db, portalUID, "", "", 23)

	eraseOK(t, db, uidA, practiceA, clientA)

	if feedbackExists(t, db, pieceA) {
		t.Fatal("the erasing Practice's piece is still there, want it destroyed")
	}
	if !feedbackExists(t, db, pieceB) {
		t.Fatal("another Practice's piece is gone -- one Practice's Erasure must not reach it")
	}
	if !feedbackExists(t, db, loginOnly) {
		t.Fatal("the login-only piece is gone while the login is still reached, want it kept until the Portal Account goes")
	}
	if got := pendingCloseJobs(t, db); !slices.Equal(got, []int{21}) {
		t.Fatalf("close jobs after the first Erasure = %v, want [21]", got)
	}

	eraseOK(t, db, uidB, practiceB, clientB)

	if feedbackExists(t, db, pieceB) || feedbackExists(t, db, loginOnly) {
		t.Fatal("a piece outlived the last Practice's Erasure, want every one destroyed with the login")
	}
	if got := pendingCloseJobs(t, db); !slices.Equal(got, []int{21, 22, 23}) {
		t.Fatalf("close jobs after the last Erasure = %v, want [21 22 23]", got)
	}
}

// TestEraseHandler_DestroysFeedbackThatNamesAnAbsorbedRecord -- a merge
// moves no Feedback, so a piece sent while an Engagement belonged to the
// record later absorbed still names that record. The survivor's Erasure
// is the only act that reaches a tombstone (#813), so it has to take the
// piece too. The login here is attached to nobody, so nothing but the
// absorbed record's own pass can be what removed the piece.
func TestEraseHandler_DestroysFeedbackThatNamesAnAbsorbedRecord(t *testing.T) {
	db := testdb.New(t)
	const uid = "owner-erase-feedback-absorbed"
	const portalUID = "portal-erase-feedback-absorbed"
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, uid, []string{ownerRole}, "employee")
	survivorID, _ := testdb.SeedEngagementInStatus(t, db, practiceID, "Vera Ash", "vera@example.com", "active")
	absorbedID := seedFullClient(t, db, practiceID, staffID)
	testdb.SeedPortalAccount(t, db, portalUID, portalUID+"@example.com")
	piece := seedPortalFeedback(t, db, portalUID, practiceID, absorbedID, 31)

	srv, session := newServer(t, db, uid)
	defer srv.Close()
	merge := authedJSON(t, session, http.MethodPost, srv.URL+"/api/practices/"+practiceID+"/clients/"+absorbedID+"/merge",
		client.MergeRequest{GivenName: "Vera Ash", OtherClientID: survivorID})
	defer merge.Body.Close()
	if merge.StatusCode != http.StatusOK {
		t.Fatalf("merge status = %d, want %d: %s", merge.StatusCode, http.StatusOK, readBody(t, merge))
	}

	eraseOK(t, db, uid, practiceID, survivorID)

	if feedbackExists(t, db, piece) {
		t.Fatal("the piece naming the absorbed record is still there, want it destroyed")
	}
	if got := pendingCloseJobs(t, db); !slices.Equal(got, []int{31}) {
		t.Fatalf("close jobs = %v, want [31]", got)
	}
}

// callAsPractice runs one statement as app_runtime under practiceID's
// own session variable -- a Staff transaction's shape -- and reports
// whether the database refused it.
func callAsPractice(t *testing.T, db *testdb.DB, practiceID, statement string, arg any) (refused bool) {
	t.Helper()
	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(t.Context(),
		`SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set_config: %v", err)
	}
	_, err = tx.ExecContext(t.Context(), statement, arg)
	return err != nil
}

// TestEraseClientFeedback_RefusesAnyoneButAnErasedClientOfThisPractice
// is the boundary 00121's first door enforces itself: it is SECURITY
// DEFINER and sees every row, so it must refuse a Client who is not
// erased, and a Client -- erased or not -- of a Practice other than the
// one in the transaction. Without that, any Staff transaction could
// destroy any Client's Feedback by naming her.
func TestEraseClientFeedback_RefusesAnyoneButAnErasedClientOfThisPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA, clientA, clientB, portalUID := seedTwoPracticeLogin(t, db, "feedback-door")
	pieceA := seedPortalFeedback(t, db, portalUID, practiceA, clientA, 41)
	markErased(t, db, clientB)

	const call = `SELECT erase_client_feedback($1)`
	if !callAsPractice(t, db, practiceA, call, clientA) {
		t.Fatal("erase_client_feedback admitted a Client who is not erased")
	}
	if !callAsPractice(t, db, practiceA, call, clientB) {
		t.Fatal("erase_client_feedback admitted another Practice's Client")
	}
	if !feedbackExists(t, db, pieceA) {
		t.Fatal("a refused call destroyed a piece")
	}
}

// TestErasePortalAccountFeedback_RefusesWhileALiveClientReachesTheLogin
// is the second door's own boundary, the same predicate
// portal_accounts_erasure_delete (00108) carries: one Practice has
// erased its Client, another's still reaches the login, so the pieces
// keyed to the login are not this Practice's to destroy.
func TestErasePortalAccountFeedback_RefusesWhileALiveClientReachesTheLogin(t *testing.T) {
	db := testdb.New(t)
	practiceA, clientA, _, portalUID := seedTwoPracticeLogin(t, db, "feedback-login-door")
	loginOnly := seedPortalFeedback(t, db, portalUID, "", "", 51)
	markErased(t, db, clientA)

	if !callAsPractice(t, db, practiceA, `SELECT erase_portal_account_feedback($1)`, portalUID) {
		t.Fatal("erase_portal_account_feedback admitted a login another Practice's un-erased Client still reaches")
	}
	if !feedbackExists(t, db, loginOnly) {
		t.Fatal("a refused call destroyed a piece")
	}
}
