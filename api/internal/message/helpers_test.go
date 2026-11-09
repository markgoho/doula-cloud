package message_test

import (
	"database/sql"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/message"
	"doula-cloud/api/internal/objectstore"
	"doula-cloud/api/internal/push"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

const doulaRole = "doula"
const ownerRole = "owner"
const adminRole = "admin"

// env is what a Staff-side test in this package starts from: a Practice
// with this package's routes mounted on a server, backed by a fresh
// in-memory ObjectStore and Pusher (no real GCS bucket or push service is
// reachable from api/ tests, per docs/testing/api.md), and one Staff member
// at the screen, an employee Doula unless signedInAs says otherwise.
type env struct {
	*fixture.Scene
}

type envOption = fixture.SceneOption

var signedInAs = fixture.SignedInAs

func newEnv(t *testing.T, opts ...envOption) *env {
	t.Helper()
	mount := func(g *staffauth.GatedRouter, ir *idempotency.Router, app *sql.DB) {
		message.Mount(g, ir, app, objectstore.NewMemoryStore(), push.NewFakePusher())
	}
	return &env{fixture.NewScene(t, mount, opts...)}
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

// seedMutedPushPreference inserts a notification_preferences row muting
// push for identityUID/engagementID, using the superuser Admin connection
// -- #303's own filter inside push_subscriptions_for_message_recipient
// (00067_notification_preferences.sql).
func seedMutedPushPreference(t *testing.T, db *testdb.DB, identityUID, engagementID string) {
	t.Helper()

	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO notification_preferences (identity_uid, engagement_id, channel, muted)
		 VALUES ($1, $2, 'push', true)`,
		identityUID, engagementID,
	); err != nil {
		t.Fatalf("seed muted notification_preferences: %v", err)
	}
}
