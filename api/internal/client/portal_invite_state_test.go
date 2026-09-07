package client_test

import (
	"testing"

	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/testdb"
)

// fetchPortalInviteState runs client.FetchPortalInviteState in its own
// transaction, the same shape runWorker (erasure_outbox_test.go) uses for
// a package function that takes a *sql.Tx directly rather than through an
// HTTP handler.
func fetchPortalInviteState(t *testing.T, db *testdb.DB, clientID string) client.PortalInviteState {
	t.Helper()
	tx, err := db.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	state, err := client.FetchPortalInviteState(t.Context(), tx, clientID)
	if err != nil {
		t.Fatalf("fetch portal invite state: %v", err)
	}
	return state
}

// TestFetchPortalInviteState_NeverInvited proves the "no client_portal_users
// row" case reads Status nil, mirroring ListItem.PortalInviteStatus's own
// "Never invited" absence (#255) -- and that a Client with an email on
// file reports HasEmail true.
func TestFetchPortalInviteState_NeverInvited(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Never Invited Practice")
	clientID := testdb.SeedNamedClient(t, db, practiceID, "Never Invited", "never@example.com")

	state := fetchPortalInviteState(t, db, clientID)

	if state.Status != nil {
		t.Fatalf("status = %v, want nil", state.Status)
	}
	if state.EmailSuppressed {
		t.Fatal("emailSuppressed = true, want false")
	}
	if !state.HasEmail {
		t.Fatal("hasEmail = false, want true")
	}
}

// TestFetchPortalInviteState_NoEmail proves a Client with no address on
// file reports HasEmail false -- the fact the Engagement page's own
// "cannot be invited at all" copy (#255) is built on.
func TestFetchPortalInviteState_NoEmail(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "No Email Practice")
	clientID := testdb.SeedNamedClient(t, db, practiceID, "No Email", "")

	state := fetchPortalInviteState(t, db, clientID)

	if state.Status != nil {
		t.Fatalf("status = %v, want nil", state.Status)
	}
	if state.HasEmail {
		t.Fatal("hasEmail = true, want false")
	}
}

// TestFetchPortalInviteState_Pending proves a pending (outbox row, no
// identity_uid) client_portal_users row reads Status "pending" -- exactly
// the word ListItem.PortalInviteStatus reports for the same row shape.
func TestFetchPortalInviteState_Pending(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Pending Practice")
	clientID := testdb.SeedNamedClient(t, db, practiceID, "Pending Client", "pending@example.com")
	seedPendingOutboxRow(t, db, clientID)

	state := fetchPortalInviteState(t, db, clientID)

	if state.Status == nil || *state.Status != pendingStatus {
		t.Fatalf("status = %v, want %q", state.Status, pendingStatus)
	}
	if !state.HasEmail {
		t.Fatal("hasEmail = false, want true")
	}
}

// TestFetchPortalInviteState_Accepted proves an accepted (identity_uid
// set) client_portal_users row reads Status "accepted" regardless of
// outbox state, the same precedence ListItem.PortalInviteStatus applies.
func TestFetchPortalInviteState_Accepted(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Accepted Practice")
	clientID := testdb.SeedNamedClient(t, db, practiceID, "Accepted Client", "accepted-fetch@example.com")
	seedAcceptedPortalUser(t, db, clientID)

	state := fetchPortalInviteState(t, db, clientID)

	if state.Status == nil || *state.Status != "accepted" {
		t.Fatalf("status = %v, want \"accepted\"", state.Status)
	}
}

// TestFetchPortalInviteState_EmailSuppressed proves EmailSuppressed
// mirrors ListItem's own #785 column: true only while the address's
// email_suppressions row has no cleared_at, compared lower-cased.
func TestFetchPortalInviteState_EmailSuppressed(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Suppressed Practice")
	clientID := testdb.SeedNamedClient(t, db, practiceID, "Suppressed Client", "Blocked@Example.com")
	seedPendingOutboxRow(t, db, clientID)
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO email_suppressions (address, cause) VALUES ($1, 'bounce')`,
		"blocked@example.com",
	); err != nil {
		t.Fatalf("seed suppression: %v", err)
	}

	state := fetchPortalInviteState(t, db, clientID)

	if !state.EmailSuppressed {
		t.Fatal("emailSuppressed = false, want true")
	}
}
