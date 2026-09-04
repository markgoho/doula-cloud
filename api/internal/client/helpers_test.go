package client_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// newServer mounts the same routes main.go wires up for this package,
// behind staffauth.Middleware, and seeds a live session for uid --
// returning the token its __session cookie carries, since #151 the
// cookie is the only credential the middleware reads.
func newServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("GET /practices/{practiceId}/clients",
		staffauth.Middleware(db.App)(client.ListHandler()))
	mux.Handle("GET /practices/{practiceId}/clients/search",
		staffauth.Middleware(db.App)(client.SearchHandler()))
	mux.Handle("POST /practices/{practiceId}/clients",
		staffauth.Middleware(db.App)(client.CreateHandler()))
	mux.Handle("GET /practices/{practiceId}/clients/{clientId}",
		staffauth.Middleware(db.App)(client.DetailHandler()))
	mux.Handle("PUT /practices/{practiceId}/clients/{clientId}",
		staffauth.Middleware(db.App)(client.EditHandler()))
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

const (
	doulaRole = "doula"
	// ownerRole is named once for the same reason doulaRole is:
	// golangci-lint's goconst check counts the literal across this
	// package's whole test surface, and #394's erasure tests seed a
	// third Owner.
	ownerRole = "owner"
	// pendingStatus is the one status string this package's tests assert
	// on from two unrelated places -- a portal invite that has not been
	// accepted, and an erasure act that has not run.
	pendingStatus = "pending"
	// deadLetteredStatus is the shared terminal-failure status, asserted
	// on by both the portal-invite outbox tests and #394's erasure ones.
	deadLetteredStatus = "dead_lettered"
)

// seedStaffAtPractice seeds an employee Doula at practiceID.
func seedStaffAtPractice(t *testing.T, db *testdb.DB, practiceID, identityUID string) (staffID string) {
	t.Helper()
	return testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{doulaRole}, "employee")
}

// seedContractorAtPractice mirrors seedStaffAtPractice but for a
// contractor Doula -- ADR-0008's attachment-narrowed column.
func seedContractorAtPractice(t *testing.T, db *testdb.DB, practiceID, identityUID string) (staffID string) {
	t.Helper()
	return testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{doulaRole}, "contractor")
}

// seedOwnerContractorAtPractice seeds a Membership holding both the
// owner role and a contractor employment type -- ADR-0017's "solo
// Practice": someone who runs the Practice and also does the work,
// billed as a contractor.
func seedOwnerContractorAtPractice(t *testing.T, db *testdb.DB, practiceID, identityUID string) (staffID string) {
	t.Helper()
	return testdb.SeedStaffAtPractice(t, db, practiceID, identityUID, []string{ownerRole, doulaRole}, "contractor")
}

// seedStaffWithMembership inserts a new Practice plus a Staff member at
// it, via seedStaffAtPractice.
func seedStaffWithMembership(t *testing.T, db *testdb.DB, identityUID string) (practiceID string) {
	t.Helper()

	practiceID = testdb.SeedPractice(t, db, "Test Practice")
	seedStaffAtPractice(t, db, practiceID, identityUID)
	return practiceID
}

// seedClient inserts a bare Client row (no Engagement) under practiceID,
// using the superuser Admin connection.
func seedClient(t *testing.T, db *testdb.DB, practiceID, givenName, email string) (clientID string) {
	t.Helper()

	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (practice_id, given_name, email) VALUES ($1, $2, NULLIF($3, '')) RETURNING id`,
		practiceID, givenName, email,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	return clientID
}

// seedClientEngagement inserts a Client and an active Engagement linking
// them to practiceID, using the superuser Admin connection.
func seedClientEngagement(t *testing.T, db *testdb.DB, practiceID, givenName, email string) (clientID, engagementID string) {
	t.Helper()

	clientID = seedClient(t, db, practiceID, givenName, email)
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, status, kind) VALUES ($1, $2, 'active', 'birth') RETURNING id`,
		clientID, practiceID,
	).Scan(&engagementID); err != nil {
		t.Fatalf("seed engagement: %v", err)
	}
	return clientID, engagementID
}

// seedPendingRequest inserts a pending engagement_requests row for
// clientID -- ADR-0017's Engagement Request record, in its default
// 'pending' state.
func seedPendingRequest(t *testing.T, db *testdb.DB, practiceID, clientID, staffID, kind string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO engagement_requests (practice_id, client_id, kind, requested_by) VALUES ($1, $2, $3, $4)`,
		practiceID, clientID, kind, staffID,
	); err != nil {
		t.Fatalf("seed pending request: %v", err)
	}
}

// seedRefusedRequest inserts a refused engagement_requests row for
// clientID -- ADR-0017's "Refusal, and the Client it leaves behind": a
// recorded state with a required reason, no Engagement, and nothing else
// built for her.
func seedRefusedRequest(t *testing.T, db *testdb.DB, practiceID, clientID, staffID, kind string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO engagement_requests (practice_id, client_id, kind, requested_by, state, decided_by, decided_at, reason)
		 VALUES ($1, $2, $3, $4, 'refused', $4, now(), 'not now')`,
		practiceID, clientID, kind, staffID,
	); err != nil {
		t.Fatalf("seed refused request: %v", err)
	}
}

// seedFieldTemplate inserts a client_field_templates row for practiceID
// directly, bypassing clientfieldtemplate.PutHandler -- for tests
// exercising resolveFields' live-read against the template in isolation
// from that package's own write path.
func seedFieldTemplate(t *testing.T, db *testdb.DB, practiceID, fieldsJSON string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO client_field_templates (practice_id, fields) VALUES ($1, $2)
		 ON CONFLICT (practice_id) DO UPDATE SET fields = EXCLUDED.fields`,
		practiceID, fieldsJSON,
	); err != nil {
		t.Fatalf("seed field template: %v", err)
	}
}

// seedGrantedAttachment inserts an open, granted-origin
// engagement_attachments row directly.
func seedGrantedAttachment(t *testing.T, db *testdb.DB, engagementID, staffID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO engagement_attachments (engagement_id, staff_id, origin, attached_by) VALUES ($1, $2, 'granted', $2)`,
		engagementID, staffID,
	); err != nil {
		t.Fatalf("seed granted attachment: %v", err)
	}
}

// seedPendingOutboxRow inserts a pending client_portal_users +
// portal_invite_outbox pair for clientID, mirroring what
// portalinvite.invite() leaves behind right after a Staff member sends an
// invite -- used to prove EditHandler revokes it on an email change.
func seedPendingOutboxRow(t *testing.T, db *testdb.DB, clientID string) (outboxID string) {
	t.Helper()

	var portalUserID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO client_portal_users (client_id, invite_token) VALUES ($1, gen_random_uuid()) RETURNING id`,
		clientID,
	).Scan(&portalUserID); err != nil {
		t.Fatalf("seed portal user: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO portal_invite_outbox (client_portal_user_id) VALUES ($1) RETURNING id`,
		portalUserID,
	).Scan(&outboxID); err != nil {
		t.Fatalf("seed outbox row: %v", err)
	}
	return outboxID
}

// seedAcceptedPortalUser inserts an accepted (identity_uid set)
// client_portal_users row for clientID -- the state accept.go leaves
// behind, which ListHandler's portalInviteStatus reads as "accepted"
// regardless of what portal_invite_outbox says.
func seedAcceptedPortalUser(t *testing.T, db *testdb.DB, clientID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO client_portal_users (client_id, invite_token, identity_uid) VALUES ($1, gen_random_uuid(), $2)`,
		clientID, "identity-"+clientID,
	); err != nil {
		t.Fatalf("seed accepted portal user: %v", err)
	}
}

// outboxStatus reads status/last_error for outboxID.
func outboxStatus(t *testing.T, db *testdb.DB, outboxID string) (status, lastError string) {
	t.Helper()
	var le any
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status, last_error FROM portal_invite_outbox WHERE id = $1`, outboxID,
	).Scan(&status, &le); err != nil {
		t.Fatalf("read outbox row: %v", err)
	}
	if le != nil {
		lastError, _ = le.(string)
	}
	return status, lastError
}
