package sitebuild_test

import (
	"net/http"
	"testing"
	"time"

	"doula-cloud/api/internal/sitebuild"
	"doula-cloud/api/internal/testdb"
)

func verifyHandler(db *testdb.DB, p sitebuild.Prober) http.Handler {
	return sitebuild.VerifyHandler(db.App, sitebuild.Verifier{Prober: p, Now: time.Now}, workerSecret)
}

func TestVerify_RefusesWithoutTheSecret(t *testing.T) {
	db := testdb.New(t)
	prober := &fakeProber{fallback: sitebuild.PageProbe{State: sitebuild.StateLive}}

	if rec := post(t, verifyHandler(db, prober), "wrong-secret"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", rec.Code)
	}
	if len(prober.asked) != 0 {
		t.Fatalf("probed %v on an unauthorized call", prober.asked)
	}
}

func TestVerify_RecordsAPageThatResolves(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedHostedPage(t, db, "Rochester Doulas", "rochester-doulas")
	prober := &fakeProber{fallback: sitebuild.PageProbe{State: sitebuild.StateLive}}

	if rec := post(t, verifyHandler(db, prober), workerSecret); rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}

	state, detail, checked := pageState(t, db, practiceID)
	if state != sitebuild.StateLive {
		t.Fatalf("page_state = %q, want %q", state, sitebuild.StateLive)
	}
	if detail != "" {
		t.Fatalf("page_check_detail = %q, want empty for a page that loaded", detail)
	}
	if !checked {
		t.Fatal("page_checked_at is null; the probe did not record when it ran")
	}
}

func TestVerify_RecordsAPageThatDoesNot(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedHostedPage(t, db, "Rochester Doulas", "rochester-doulas")
	prober := &fakeProber{fallback: sitebuild.PageProbe{
		State:  sitebuild.StateFailed,
		Detail: "the site answered 404 for this page",
	}}

	post(t, verifyHandler(db, prober), workerSecret)

	state, detail, _ := pageState(t, db, practiceID)
	if state != sitebuild.StateFailed {
		t.Fatalf("page_state = %q, want %q", state, sitebuild.StateFailed)
	}
	if detail == "" {
		t.Fatal("page_check_detail is empty; she is told it failed but not why")
	}
}

// One Practice's broken page must not take another's down with it, and
// each result has to land on its own row.
func TestVerify_ScoresEachPageOnItsOwn(t *testing.T) {
	db := testdb.New(t)
	good := seedHostedPage(t, db, "Rochester Doulas", "rochester-doulas")
	bad := seedHostedPage(t, db, "Finger Lakes Birth", "finger-lakes-birth")
	prober := &fakeProber{
		results: map[string]sitebuild.PageProbe{
			"rochester-doulas":   {State: sitebuild.StateLive},
			"finger-lakes-birth": {State: sitebuild.StateFailed, Detail: "the site did not answer"},
		},
	}

	post(t, verifyHandler(db, prober), workerSecret)

	if state, _, _ := pageState(t, db, good); state != sitebuild.StateLive {
		t.Fatalf("live page reads %q", state)
	}
	if state, _, _ := pageState(t, db, bad); state != sitebuild.StateFailed {
		t.Fatalf("broken page reads %q", state)
	}
	if len(prober.asked) != 2 {
		t.Fatalf("probed %v, want both pages", prober.asked)
	}
}

// A page that was live and has since broken must be caught. This is
// what the sweep buys beyond the post-deploy callback.
func TestVerify_TurnsALivePageBackToFailed(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedHostedPage(t, db, "Rochester Doulas", "rochester-doulas")
	post(t, verifyHandler(db, &fakeProber{fallback: sitebuild.PageProbe{State: sitebuild.StateLive}}), workerSecret)

	post(t, verifyHandler(db, &fakeProber{fallback: sitebuild.PageProbe{
		State: sitebuild.StateFailed, Detail: "the site answered 404 for this page",
	}}), workerSecret)

	if state, _, _ := pageState(t, db, practiceID); state != sitebuild.StateFailed {
		t.Fatalf("page_state = %q, want the regression caught", state)
	}
}

// A Practice on her own website has no page of ours, and the sweep must
// not invent a state for one.
func TestVerify_IgnoresAPracticeOnHerOwnWebsite(t *testing.T) {
	db := testdb.New(t)
	var practiceID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO practices (name) VALUES ('Own Site Doula') RETURNING id`,
	).Scan(&practiceID); err != nil {
		t.Fatalf("seed practice: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_websites (practice_id, mode, own_url)
		 VALUES ($1, 'own', 'https://example.com')`, practiceID,
	); err != nil {
		t.Fatalf("seed own-site declaration: %v", err)
	}
	prober := &fakeProber{fallback: sitebuild.PageProbe{State: sitebuild.StateLive}}

	post(t, verifyHandler(db, prober), workerSecret)

	if len(prober.asked) != 0 {
		t.Fatalf("probed %v, want nothing for a Practice with no page here", prober.asked)
	}
	state, _, _ := pageState(t, db, practiceID)
	if state != "" {
		t.Fatalf("page_state = %q, want no state at all", state)
	}
}

// Nothing published is not a failure.
func TestVerify_NoPagesAtAll(t *testing.T) {
	db := testdb.New(t)

	if rec := post(t, verifyHandler(db, &fakeProber{}), workerSecret); rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
}
