package website_test

import (
	"testing"

	"doula-cloud/api/internal/testdb"
)

// These tests exercise 00045's two practice_id policies directly via
// db.App and set_config -- a plain column comparison, the shape
// practice_memberships (00002) uses -- rather than through the handlers,
// which prove the same thing only as far as staffauth.Middleware is
// trusted to set the session variable.

func seedWebsite(t *testing.T, db *testdb.DB, practiceID, mode, ownURL string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_websites (practice_id, mode, own_url, service_description, cancellation_policy, slug, page_state)
		 VALUES ($1, $2, NULLIF($3, ''), 'Birth support.', 'Two weeks notice.', $4,
		         CASE WHEN $2 = 'hosted' THEN 'pending'::practice_page_state END)`,
		practiceID, mode, ownURL, practiceID,
	); err != nil {
		t.Fatalf("seed practice_websites: %v", err)
	}
}

func seedWebsiteEvent(t *testing.T, db *testdb.DB, practiceID, actorStaffID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_website_events (practice_id, mode, actor_staff_id) VALUES ($1, 'own', $2)`,
		practiceID, actorStaffID,
	); err != nil {
		t.Fatalf("seed practice_website_events: %v", err)
	}
}

// TestRLS_FailsClosedWithNoSessionVarSet proves both tables deny every
// row when app.current_practice_id is unset -- fail closed, not open.
func TestRLS_FailsClosedWithNoSessionVarSet(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Rochester Doulas")
	staffID := seedStaff(t, db, "rls-closed", "Maya Chen")
	seedWebsite(t, db, practiceID, "own", ownSiteURL)
	seedWebsiteEvent(t, db, practiceID, staffID)

	for _, table := range []string{"practice_websites", "practice_website_events"} {
		var count int
		if err := db.App.QueryRowContext(t.Context(),
			`SELECT count(*) FROM `+table, //nolint:gosec // table name is a test literal, not input
		).Scan(&count); err != nil {
			t.Fatalf("query %s with no session variables set: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("%s: expected 0 rows with no session variables set, got %d", table, count)
		}
	}
}

// TestRLS_VisibilityIsScopedToCurrentPractice proves Practice A's
// session sees Practice A's website and never Practice B's -- the answer
// is public once published, but who published it and when is the
// Practice's own record.
func TestRLS_VisibilityIsScopedToCurrentPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA := seedPractice(t, db, "Practice A")
	practiceB := seedPractice(t, db, "Practice B")
	staffA := seedStaff(t, db, "rls-a", "Maya Chen")
	staffB := seedStaff(t, db, "rls-b", "Ana Reyes")
	seedWebsite(t, db, practiceA, "own", "https://a.example.com")
	seedWebsite(t, db, practiceB, "own", "https://b.example.com")
	seedWebsiteEvent(t, db, practiceA, staffA)
	seedWebsiteEvent(t, db, practiceB, staffB)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(t.Context(),
		`SELECT set_config('app.current_practice_id', $1, true)`, practiceA); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var url string
	if err := tx.QueryRowContext(t.Context(), `SELECT own_url FROM practice_websites`).Scan(&url); err != nil {
		t.Fatalf("select practice_websites as Practice A: %v", err)
	}
	if url != "https://a.example.com" {
		t.Fatalf("own_url = %q, want Practice A's", url)
	}

	var events int
	if err := tx.QueryRowContext(t.Context(), `SELECT count(*) FROM practice_website_events`).Scan(&events); err != nil {
		t.Fatalf("count practice_website_events as Practice A: %v", err)
	}
	if events != 1 {
		t.Fatalf("events visible = %d, want only Practice A's 1", events)
	}
}

// TestRLS_RefusesAWriteForAnotherPractice proves WITH CHECK is spelled
// out rather than left to default: a session acting as Practice A cannot
// insert a row naming Practice B.
func TestRLS_RefusesAWriteForAnotherPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA := seedPractice(t, db, "Practice A")
	practiceB := seedPractice(t, db, "Practice B")
	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(t.Context(),
		`SELECT set_config('app.current_practice_id', $1, true)`, practiceA); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	if _, err := tx.ExecContext(t.Context(),
		`INSERT INTO practice_websites (practice_id, mode, own_url) VALUES ($1, 'own', 'https://b.example.com')`,
		practiceB,
	); err == nil {
		t.Fatal("insert for another Practice succeeded, want it refused")
	}
}

// TestRLS_EventsCannotBeRewritten proves the audit trail is append-only
// at the grant level: app_runtime holds SELECT and INSERT and nothing
// else, so an UPDATE fails however the policies read.
func TestRLS_EventsCannotBeRewritten(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Rochester Doulas")
	staffID := seedStaff(t, db, "rls-append-only", "Maya Chen")
	seedWebsiteEvent(t, db, practiceID, staffID)

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(t.Context(),
		`SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	if _, err := tx.ExecContext(t.Context(),
		`UPDATE practice_website_events SET mode = 'hosted'`); err == nil {
		t.Fatal("update on practice_website_events succeeded, want it refused")
	}
}

// TestSchema_RefusesAModeWithoutItsFacts proves 00045's CHECK
// constraints hold the line the handler also holds: a hosted page
// without the two facts, and an own declaration without a URL, are both
// impossible rather than merely unlikely.
func TestSchema_RefusesAModeWithoutItsFacts(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Rochester Doulas")

	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_websites (practice_id, mode) VALUES ($1, 'own')`, practiceID,
	); err == nil {
		t.Fatal("inserted mode 'own' with no URL, want it refused")
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_websites (practice_id, mode, service_description) VALUES ($1, 'hosted', 'Birth support.')`,
		practiceID,
	); err == nil {
		t.Fatal("inserted mode 'hosted' with no cancellation policy, want it refused")
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_websites (practice_id, mode, service_description, cancellation_policy)
		 VALUES ($1, 'hosted', repeat('a', 501), 'Two weeks.')`, practiceID,
	); err == nil {
		t.Fatal("inserted a service description past the budget, want it refused")
	}
}

// TestRLS_SiteBuilderReadsEveryPublishedPageAndWritesNothing proves
// 00046's role does the one job it exists for.
//
// Every other policy in this schema scopes a read to one Practice, which
// is right for the BFF and useless for a build that has to render every
// published page in one pass. The permissive policies added for
// site_builder sit beside the per-Practice ones rather than replacing
// them, and the grant is SELECT and nothing else -- so the credential
// that lives in a public website's build job can read what is about to
// be published and change nothing at all.
func TestRLS_SiteBuilderReadsEveryPublishedPageAndWritesNothing(t *testing.T) {
	db := testdb.New(t)
	first := seedPractice(t, db, "Rochester Doulas")
	second := seedPractice(t, db, "Genesee Birth Collective")
	seedWebsite(t, db, first, "hosted", "")
	seedWebsite(t, db, second, "hosted", "")

	// A connection of its own, so SET ROLE cannot leak into another
	// test's use of the pool.
	conn, err := db.Admin.Conn(t.Context())
	if err != nil {
		t.Fatalf("open connection: %v", err)
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.ExecContext(t.Context(), `SET ROLE site_builder`); err != nil {
		t.Fatalf("set role site_builder: %v", err)
	}

	var count int
	if err := conn.QueryRowContext(t.Context(),
		`SELECT count(*) FROM practice_websites WHERE mode = 'hosted'`,
	).Scan(&count); err != nil {
		t.Fatalf("read published pages as site_builder: %v", err)
	}
	if count != 2 {
		t.Fatalf("site_builder sees %d published pages, want 2 -- a build that "+
			"sees fewer than exist deletes the pages it cannot see", count)
	}

	if _, err := conn.ExecContext(t.Context(),
		`UPDATE practice_websites SET mode = 'own' WHERE practice_id = $1`, first,
	); err == nil {
		t.Fatal("site_builder wrote to practice_websites; the grant is SELECT only")
	}
}
