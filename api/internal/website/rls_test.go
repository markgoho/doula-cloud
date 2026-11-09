package website_test

import (
	"testing"

	"doula-cloud/api/internal/fixture"
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
	practice := fixture.Practice(t, db)
	seedWebsite(t, db, practice.ID, "own", ownSiteURL)
	seedWebsiteEvent(t, db, practice.ID, fixture.Staff(t, db, practice).ID)

	testdb.RequireFailsClosed(t, db, "practice_websites")
	testdb.RequireFailsClosed(t, db, "practice_website_events")
}

// TestRLS_VisibilityIsScopedToCurrentPractice proves Practice A's
// session sees Practice A's website and never Practice B's -- the answer
// is public once published, but who published it and when is the
// Practice's own record.
func TestRLS_VisibilityIsScopedToCurrentPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA := fixture.Practice(t, db)
	practiceB := fixture.Practice(t, db)
	seedWebsite(t, db, practiceA.ID, "own", "https://a.example.com")
	seedWebsite(t, db, practiceB.ID, "own", "https://b.example.com")
	seedWebsiteEvent(t, db, practiceA.ID, fixture.Staff(t, db, practiceA).ID)
	seedWebsiteEvent(t, db, practiceB.ID, fixture.Staff(t, db, practiceB).ID)

	tx := db.BeginAs(t, testdb.Practice(practiceA.ID))

	url := testdb.Scalar[string](t, tx, `SELECT own_url FROM practice_websites`)
	if url != "https://a.example.com" {
		t.Fatalf("own_url = %q, want Practice A's", url)
	}

	events := testdb.Scalar[int](t, tx, `SELECT count(*) FROM practice_website_events`)
	if events != 1 {
		t.Fatalf("events visible = %d, want only Practice A's 1", events)
	}
}

// TestRLS_RefusesAWriteForAnotherPractice proves WITH CHECK is spelled
// out rather than left to default: a session acting as Practice A cannot
// insert a row naming Practice B.
func TestRLS_RefusesAWriteForAnotherPractice(t *testing.T) {
	db := testdb.New(t)
	practiceA := fixture.Practice(t, db)
	practiceB := fixture.Practice(t, db)
	tx := db.BeginAs(t, testdb.Practice(practiceA.ID))

	if _, err := tx.ExecContext(t.Context(),
		`INSERT INTO practice_websites (practice_id, mode, own_url) VALUES ($1, 'own', 'https://b.example.com')`,
		practiceB.ID,
	); err == nil {
		t.Fatal("insert for another Practice succeeded, want it refused")
	}
}

// TestRLS_EventsCannotBeRewritten proves the audit trail is append-only
// at the grant level: app_runtime holds SELECT and INSERT and nothing
// else, so an UPDATE fails however the policies read.
func TestRLS_EventsCannotBeRewritten(t *testing.T) {
	db := testdb.New(t)
	practice := fixture.Practice(t, db)
	seedWebsiteEvent(t, db, practice.ID, fixture.Staff(t, db, practice).ID)

	tx := db.BeginAs(t, testdb.Practice(practice.ID))

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
	practiceID := testdb.SeedPractice(t, db, "Rochester Doulas")

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
	first := testdb.SeedPractice(t, db, "Rochester Doulas")
	second := testdb.SeedPractice(t, db, "Genesee Birth Collective")
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

	count := testdb.Scalar[int](t, conn, `SELECT count(*) FROM practice_websites WHERE mode = 'hosted'`)
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
