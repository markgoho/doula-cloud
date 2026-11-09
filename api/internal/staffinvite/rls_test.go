package staffinvite_test

import (
	"testing"

	"doula-cloud/api/internal/testdb"
)

// TestRLS_NotificationWorkerSeesPracticeInvitationsOnlyWhenTrusted
// exercises the practice_invitations_notification_worker policy from
// 00038_staff_invite_outbox.sql directly via db.App and set_config,
// mirroring portalinvite's equivalent pair for client_portal_users/clients
// (00032).
//
// The policy stays closed by default: with no session context at all --
// the shape outbox.ProcessHandler runs under before it sets
// app.notification_worker_trusted -- practice_invitations_practice_visibility
// (00030) admits nothing either, since app.current_practice_id is unset.
// And the door outbox.ProcessHandler relies on actually opens: with the
// flag set, every Invitation is visible, regardless of Practice --
// required since the worker has no single Practice's session to scope by.
func TestRLS_NotificationWorkerSeesPracticeInvitationsOnlyWhenTrusted(t *testing.T) {
	tests := []struct {
		name string
		vars []testdb.SessionVar
		want int
	}{
		{"with no session context", nil, 0},
		{"with the trusted flag set, across Practices", []testdb.SessionVar{testdb.NotificationWorker()}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := testdb.New(t)
			seedPracticeInvitation(t, db, testdb.SeedPractice(t, db, "Worker Visibility A"), "a@example.com")
			seedPracticeInvitation(t, db, testdb.SeedPractice(t, db, "Worker Visibility B"), "b@example.com")

			tx := db.BeginAs(t, tt.vars...)
			if got := testdb.Scalar[int](t, tx, `SELECT count(*) FROM practice_invitations`); got != tt.want {
				t.Fatalf("visible Invitations = %d, want %d", got, tt.want)
			}
		})
	}
}
