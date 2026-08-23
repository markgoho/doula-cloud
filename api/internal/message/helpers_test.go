package message_test

import (
	"testing"

	"doula-cloud/api/internal/testdb"
)

// seedPractice inserts a Practice using the superuser Admin connection.
func seedPractice(t *testing.T, db *testdb.DB, name string) string {
	t.Helper()
	var id string
	if err := db.Admin.QueryRowContext(t.Context(), `INSERT INTO practices (name) VALUES ($1) RETURNING id`, name).Scan(&id); err != nil {
		t.Fatalf("seed practice %q: %v", name, err)
	}
	return id
}

// seedStaffAtPractice inserts a Staff row bound to identityUID and a
// practice_memberships row linking them to an existing practiceID, using
// the superuser Admin connection so fixture setup isn't gated by the
// policies under test.
func seedStaffAtPractice(t *testing.T, db *testdb.DB, practiceID, identityUID string) (staffID string) {
	t.Helper()

	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff (identity_uid, name, email) VALUES ($1, 'Test Staff', 'staff@example.com') RETURNING id`,
		identityUID,
	).Scan(&staffID); err != nil {
		t.Fatalf("seed staff: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_memberships (practice_id, staff_id, roles, employment_type) VALUES ($1, $2, '{doula}', 'employee')`,
		practiceID, staffID,
	); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	return staffID
}

// seedStaffAtPracticeNamed mirrors seedStaffAtPractice but takes an
// explicit name, for tests that assert a specific Staff member's name
// shows up as a Message's sender -- seedStaffAtPractice always hardcodes
// "Test Staff", which would make such an assertion vacuous once two Staff
// are seeded in the same test.
func seedStaffAtPracticeNamed(t *testing.T, db *testdb.DB, practiceID, identityUID, name string) (staffID string) {
	t.Helper()

	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff (identity_uid, name, email) VALUES ($1, $2, 'staff@example.com') RETURNING id`,
		identityUID, name,
	).Scan(&staffID); err != nil {
		t.Fatalf("seed staff: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_memberships (practice_id, staff_id, roles, employment_type) VALUES ($1, $2, '{doula}', 'employee')`,
		practiceID, staffID,
	); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
	return staffID
}

// seedContractorAtPractice mirrors seedStaffAtPractice but for a
// contractor Doula -- ADR-0008's attachment-narrowed column.
func seedContractorAtPractice(t *testing.T, db *testdb.DB, practiceID, identityUID string) (staffID string) {
	t.Helper()

	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff (identity_uid, name, email) VALUES ($1, 'Test Staff', 'staff@example.com') RETURNING id`,
		identityUID,
	).Scan(&staffID); err != nil {
		t.Fatalf("seed staff: %v", err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_memberships (practice_id, staff_id, roles, employment_type) VALUES ($1, $2, '{doula}', 'contractor')`,
		practiceID, staffID,
	); err != nil {
		t.Fatalf("seed contractor membership: %v", err)
	}
	return staffID
}

// seedGrantedAttachment inserts an open, granted-origin
// engagement_attachments row directly -- no handler in this codebase
// writes one yet (#317 builds that).
func seedGrantedAttachment(t *testing.T, db *testdb.DB, engagementID, staffID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO engagement_attachments (engagement_id, staff_id, origin, attached_by) VALUES ($1, $2, 'granted', $2)`,
		engagementID, staffID,
	); err != nil {
		t.Fatalf("seed granted attachment: %v", err)
	}
}

// seedClientEngagement inserts a Client and an Engagement linking them to
// practiceID, using the superuser Admin connection.
func seedClientEngagement(t *testing.T, db *testdb.DB, practiceID, name, email string) (clientID, engagementID string) {
	t.Helper()

	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (name, email) VALUES ($1, $2) RETURNING id`,
		name, email,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, status) VALUES ($1, $2, 'intake') RETURNING id`,
		clientID, practiceID,
	).Scan(&engagementID); err != nil {
		t.Fatalf("seed engagement: %v", err)
	}
	return clientID, engagementID
}

// seedPortalUser links identityUID to clientID via client_portal_users,
// using the superuser Admin connection.
func seedPortalUser(t *testing.T, db *testdb.DB, identityUID, clientID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO client_portal_users (identity_uid, client_id) VALUES ($1, $2)`,
		identityUID, clientID,
	); err != nil {
		t.Fatalf("seed client_portal_users: %v", err)
	}
}

// seedMessage inserts a Message on engagementID from senderType/senderID,
// using the superuser Admin connection so fixture setup isn't gated by
// the RLS policies under test.
func seedMessage(t *testing.T, db *testdb.DB, engagementID, senderType, senderID, body string) {
	t.Helper()

	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO messages (engagement_id, sender_type, sender_id, body) VALUES ($1, $2, $3, $4)`,
		engagementID, senderType, senderID, body,
	); err != nil {
		t.Fatalf("seed message: %v", err)
	}
}

// seedMessageWithAttachment inserts a Message row carrying attachment
// metadata directly (bypassing CreateHandler/ObjectStore.Put entirely),
// for tests that need a DB row pointing at an object path without a real
// upload -- e.g. exercising the download endpoint's ObjectStore.Get
// failure branch, where the store never needs to have actually stored
// anything.
func seedMessageWithAttachment(t *testing.T, db *testdb.DB, engagementID, senderType, senderID, objectPath, contentType, filename string, byteSize int64) (messageID string) {
	t.Helper()

	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO messages (engagement_id, sender_type, sender_id, attachment_object_path, attachment_content_type, attachment_byte_size, attachment_filename)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		engagementID, senderType, senderID, objectPath, contentType, byteSize, filename,
	).Scan(&messageID); err != nil {
		t.Fatalf("seed message with attachment: %v", err)
	}
	return messageID
}

// seedPushSubscription inserts a push_subscriptions row for
// ownerType/ownerID, using the superuser Admin connection.
func seedPushSubscription(t *testing.T, db *testdb.DB, ownerType, ownerID, endpoint string) (id string) {
	t.Helper()

	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO push_subscriptions (owner_type, owner_id, endpoint, p256dh_key, auth_key)
		 VALUES ($1, $2, $3, 'p256dh-key', 'auth-key') RETURNING id`,
		ownerType, ownerID, endpoint,
	).Scan(&id); err != nil {
		t.Fatalf("seed push_subscription: %v", err)
	}
	return id
}
