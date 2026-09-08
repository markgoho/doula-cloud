package testdb

import (
	"encoding/json"
	"strings"
	"testing"

	"doula-cloud/api/internal/activity"
)

// SeedPractice inserts a bare Practice row using the superuser Admin
// connection.
func SeedPractice(t *testing.T, db *DB, name string) (practiceID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO practices (name) VALUES ($1) RETURNING id`, name,
	).Scan(&practiceID); err != nil {
		// coverage:ignore reason: fixture insert failure, not exercised by the happy-path test
		t.Fatalf("testdb: seed practice %q: %v", name, err)
	}
	return practiceID
}

// SeedClientsCanPay flips practiceID's stripe_connect_card_payments_status
// straight to 'active' -- the shared fixture for payments.ClientsCanPay's
// three readers (the Engagement read, the Practice-wide Invoice totals,
// PostInvoiceHandler's own gate), which each need this exact state and
// nothing else about a Connect account (no account id, no Stripe fixture
// call). Bypasses the Connect webhook that ordinarily writes this column,
// the same way SeedPractice bypasses onboarding.
func SeedClientsCanPay(t *testing.T, db *DB, practiceID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`UPDATE practices SET stripe_connect_card_payments_status = 'active' WHERE id = $1`, practiceID,
	); err != nil {
		// coverage:ignore reason: fixture update failure, not exercised by the happy-path test
		t.Fatalf("testdb: seed clients-can-pay %q: %v", practiceID, err)
	}
}

// SeedStaff inserts a bare Staff row, with no practice_memberships row,
// using the superuser Admin connection. Named "Test Staff "+identityUID
// and emailed identityUID+"@example.com", the same derivation
// SeedStaffAtPractice uses, so a caller that later attaches a membership
// itself (a test proving RLS visibility before and after membership is
// granted, say) sees the same Staff identity either way.
func SeedStaff(t *testing.T, db *DB, identityUID string) (staffID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff (identity_uid, name, email, work_state) VALUES ($1, $2, $3, 'NY') RETURNING id`,
		identityUID, "Test Staff "+identityUID, identityUID+"@example.com",
	).Scan(&staffID); err != nil {
		// coverage:ignore reason: fixture insert failure, not exercised by the happy-path test
		t.Fatalf("testdb: seed staff: %v", err)
	}
	return staffID
}

// SeedStaffAtNewPractice creates a fresh Practice and a Staff member on
// it in one call, for the "give me an authenticated caller at her own
// Practice" fixture shape a dozen package tests repeated as their own
// seedOwner/seedMember.
func SeedStaffAtNewPractice(t *testing.T, db *DB, identityUID string, roles []string, employmentType string) (practiceID, staffID string) {
	t.Helper()
	practiceID = SeedPractice(t, db, "Test Practice")
	staffID = SeedStaffAtPractice(t, db, practiceID, identityUID, roles, employmentType)
	return practiceID, staffID
}

// SeedContractorAtPractice seeds a Staff row at practiceID with the
// contractor Doula shape (`doula` role, contractor employment) a
// half-dozen package tests each declared their own copy of.
func SeedContractorAtPractice(t *testing.T, db *DB, practiceID, identityUID string) (staffID string) {
	t.Helper()
	return SeedStaffAtPractice(t, db, practiceID, identityUID, []string{"doula"}, "contractor")
}

// SeedStaffAtPractice inserts a Staff row bound to identityUID and a
// practice_memberships row granting roles at practiceID as
// employmentType, using the superuser Admin connection (which bypasses
// RLS) so fixture setup isn't gated by the policies under test. The Staff
// row is named "Test Staff "+identityUID and emailed
// identityUID+"@example.com" -- both derived from the one value every
// caller already picks uniquely -- so two Staff seeded in the same test
// never collide.
func SeedStaffAtPractice(t *testing.T, db *DB, practiceID, identityUID string, roles []string, employmentType string) (staffID string) {
	t.Helper()
	return SeedNamedStaffAtPractice(t, db, practiceID, identityUID, "Test Staff "+identityUID, roles, employmentType)
}

// SeedNamedStaffAtPractice is SeedStaffAtPractice with an explicit
// display name, for a test that asserts a specific name comes back (e.g.
// a Message's sender).
func SeedNamedStaffAtPractice(t *testing.T, db *DB, practiceID, identityUID, name string, roles []string, employmentType string) (staffID string) {
	t.Helper()

	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO staff (identity_uid, name, email, work_state) VALUES ($1, $2, $3, 'NY') RETURNING id`,
		identityUID, name, identityUID+"@example.com",
	).Scan(&staffID); err != nil {
		// coverage:ignore reason: fixture insert failure, not exercised by the happy-path test
		t.Fatalf("testdb: seed staff: %v", err)
	}

	literal := "{" + strings.Join(roles, ",") + "}"
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_memberships (practice_id, staff_id, roles, employment_type) VALUES ($1, $2, $3::practice_role[], $4)`,
		practiceID, staffID, literal, employmentType,
	); err != nil {
		// coverage:ignore reason: fixture insert failure, not exercised by the happy-path test
		t.Fatalf("testdb: seed membership: %v", err)
	}
	return staffID
}

// SeedPortalAccount inserts a portal_accounts row using the superuser
// Admin connection. client_portal_users.identity_uid (#616) carries a
// foreign key to portal_accounts.identifier, so any fixture that seeds a
// client_portal_users row with identity_uid set needs a matching row
// here first, or the insert fails.
func SeedPortalAccount(t *testing.T, db *DB, identifier, signInAddress string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO portal_accounts (identifier, sign_in_address) VALUES ($1, $2)`,
		identifier, signInAddress,
	); err != nil {
		// coverage:ignore reason: fixture insert failure, not exercised by the happy-path test
		t.Fatalf("testdb: seed portal account %q: %v", identifier, err)
	}
}

// SeedEngagement inserts a bare Client and an Engagement linking them to
// practiceID, using the superuser Admin connection -- the minimum an
// Engagement-scoped access check (staffauth.Reader.CanAccessEngagement,
// activitygate's own Rules) needs to decide against. Collapses the
// near-identical seedAccessEngagement/seedEngagement copies #706 found
// duplicated in staffauth_test and activitygate_test.
func SeedEngagement(t *testing.T, db *DB, practiceID string) (clientID, engagementID string) {
	t.Helper()
	return SeedNamedEngagement(t, db, practiceID, "Test Client", "test-client@example.com")
}

// SeedNamedClient inserts a Client row with the given name and email,
// using the superuser Admin connection. An empty email is stored as NULL
// (NULLIF), not as an empty string -- ADR-0017's "no email on file"
// refusal (payments' invoicing path) and client's own erasure/merge
// tests both key off the column actually being NULL.
func SeedNamedClient(t *testing.T, db *DB, practiceID, givenName, email string) (clientID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (practice_id, given_name, email) VALUES ($1, $2, NULLIF($3, '')) RETURNING id`,
		practiceID, givenName, email,
	).Scan(&clientID); err != nil {
		// coverage:ignore reason: fixture insert failure, not exercised by the happy-path test
		t.Fatalf("testdb: seed client %q: %v", givenName, err)
	}
	return clientID
}

// SeedNamedEngagement inserts a Client with the given name and email and
// an Engagement linking them to practiceID, at the schema's default
// status ("intake"). Collapses the near-identical seedClientEngagement
// copies #848 found duplicated across a dozen package tests.
func SeedNamedEngagement(t *testing.T, db *DB, practiceID, name, email string) (clientID, engagementID string) {
	t.Helper()
	clientID = SeedNamedClient(t, db, practiceID, name, email)
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, kind) VALUES ($1, $2, 'birth') RETURNING id`,
		clientID, practiceID,
	).Scan(&engagementID); err != nil {
		// coverage:ignore reason: fixture insert failure, not exercised by the happy-path test
		t.Fatalf("testdb: seed engagement: %v", err)
	}
	return clientID, engagementID
}

// SeedEngagementWithKind is SeedNamedEngagement with an explicit
// Engagement kind, for a test that needs a postpartum-only Engagement
// rather than the hardcoded 'birth' every other seed helper here inserts
// (#311's suppression rule is the first thing to need one).
func SeedEngagementWithKind(t *testing.T, db *DB, practiceID, name, email, kind string) (clientID, engagementID string) {
	t.Helper()
	clientID = SeedNamedClient(t, db, practiceID, name, email)
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, kind) VALUES ($1, $2, $3) RETURNING id`,
		clientID, practiceID, kind,
	).Scan(&engagementID); err != nil {
		// coverage:ignore reason: fixture insert failure, not exercised by the happy-path test
		t.Fatalf("testdb: seed engagement: %v", err)
	}
	return clientID, engagementID
}

// EndingReasonForStatus is 'care_complete' for status == "completed" and
// nil for anything else -- #940's engagements_completed_is_explained
// CHECK demands a non-null ending_reason on every completed row, and
// every fixture across every package that seeds an Engagement directly
// (rather than through SeedEngagementInStatus below) needs the identical
// conditional to stay legal. One function so the "if completed, supply
// any legal reason" decision lives in one place rather than copied at
// each INSERT.
func EndingReasonForStatus(status string) any {
	if status == "completed" {
		return "care_complete"
	}
	return nil
}

// BirthOutcomeForStatus is EndingReasonForStatus's twin for the second
// half of the same CHECK (#940): a 'completed' row carries a birth
// outcome too. 'unknown' is the fixture's answer because it is the one
// value engagements_outcome_is_dated (00093) lets stand with no
// pregnancy_ended_on, so a fixture that only needs a completed
// Engagement does not have to invent a date to get one -- which is
// exactly the cost ADR-0015 accepts for a real Practice as well.
func BirthOutcomeForStatus(status string) any {
	if status == "completed" {
		return "unknown"
	}
	return nil
}

// SeedEngagementInStatus is SeedNamedEngagement with an explicit
// Engagement status, for a test that needs the Engagement in a specific
// state rather than the default "intake". A status of "completed" also
// sets ending_reason and birth_outcome (EndingReasonForStatus,
// BirthOutcomeForStatus) -- see their own doc comments.
func SeedEngagementInStatus(t *testing.T, db *DB, practiceID, name, email, status string) (clientID, engagementID string) {
	t.Helper()
	clientID = SeedNamedClient(t, db, practiceID, name, email)
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO engagements (client_id, practice_id, status, kind, ending_reason, birth_outcome)
		 VALUES ($1, $2, $3, 'birth', $4, $5) RETURNING id`,
		clientID, practiceID, status, EndingReasonForStatus(status), BirthOutcomeForStatus(status),
	).Scan(&engagementID); err != nil {
		// coverage:ignore reason: fixture insert failure, not exercised by the happy-path test
		t.Fatalf("testdb: seed engagement: %v", err)
	}
	return clientID, engagementID
}

// SeedAttachment inserts an engagement_attachments row directly, using the
// superuser Admin connection -- origin is "accrued" or "granted"
// (ADR-0008), and ended sets ended_at to now() rather than leaving the
// attachment open. attached_by is always staffID: no caller here needs to
// distinguish an accrual from a grant by a different actor. Collapses the
// identical seedAttachment copies #706 found duplicated in staffauth_test
// and activitygate_test.
func SeedAttachment(t *testing.T, db *DB, engagementID, staffID, origin string, ended bool) {
	t.Helper()
	endedAt := "NULL"
	if ended {
		endedAt = "now()"
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO engagement_attachments (engagement_id, staff_id, origin, attached_by, ended_at)
		 VALUES ($1, $2, $3::attachment_origin, $2, `+endedAt+`)`,
		engagementID, staffID, origin,
	); err != nil {
		// coverage:ignore reason: fixture insert failure, not exercised by the happy-path test
		t.Fatalf("testdb: seed attachment (origin=%s, ended=%v): %v", origin, ended, err)
	}
}

// SeedGrantedAttachment is SeedAttachment with origin "granted" and
// ended false -- the open, granted attachment shape a half-dozen package
// tests each declared their own copy of.
func SeedGrantedAttachment(t *testing.T, db *DB, engagementID, staffID string) {
	t.Helper()
	SeedAttachment(t, db, engagementID, staffID, "granted", false)
}

// SeedActivity writes one activity.Record row directly against a
// transaction of its own, for a test that needs an audit-trail entry to
// already exist rather than exercising the write that would produce it.
// Collapses the near-identical seedActivity copies #848 found duplicated
// in portal, activityfeed and engagement's own tests.
func SeedActivity(t *testing.T, db *DB, practiceID, subjectKind, subjectID, action string, actor activity.Actor) {
	t.Helper()
	SeedActivityWithDiff(t, db, practiceID, subjectKind, subjectID, action, actor, nil)
}

// SeedActivityWithDiff is SeedActivity for a row whose diff is the point
// of the test -- a visit_reassigned entry naming both ends of the move,
// for instance. A nil diff takes activity.Record's own default, which is
// what SeedActivity passes.
func SeedActivityWithDiff(t *testing.T, db *DB, practiceID, subjectKind, subjectID, action string, actor activity.Actor, diff json.RawMessage) {
	t.Helper()
	tx, err := db.Admin.BeginTx(t.Context(), nil)
	if err != nil {
		// coverage:ignore reason: fixture transaction failure, not exercised by the happy-path test
		t.Fatalf("testdb: seed activity begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := activity.Record(t.Context(), tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: subjectKind,
		SubjectID:   subjectID,
		Action:      action,
		Diff:        diff,
		Actor:       actor,
	}); err != nil {
		// coverage:ignore reason: fixture insert failure, not exercised by the happy-path test
		t.Fatalf("testdb: seed activity: %v", err)
	}
	if err := tx.Commit(); err != nil {
		// coverage:ignore reason: fixture transaction failure, not exercised by the happy-path test
		t.Fatalf("testdb: seed activity commit: %v", err)
	}
}

// AttachPortalUser links an already-seeded Portal Account (identifier) to
// clientID via a new client_portal_users row, using the superuser Admin
// connection. For the second (and later) client_portal_users row a
// multi-Practice Portal Account holds (#309, ADR-0015) -- the first row
// is SeedPortalAccount's own caller's job, since that call also mints the
// Portal Account itself and a second SeedPortalAccount call for the same
// identifier would collide on portal_accounts' own primary key.
func AttachPortalUser(t *testing.T, db *DB, identifier, clientID string) {
	t.Helper()
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO client_portal_users (identity_uid, client_id) VALUES ($1, $2)`,
		identifier, clientID,
	); err != nil {
		// coverage:ignore reason: fixture insert failure, not exercised by the happy-path test
		t.Fatalf("testdb: attach portal user %q to client %q: %v", identifier, clientID, err)
	}
}

// SeedPortalUser mints a fresh Portal Account for identityUID and
// attaches it to clientID -- the "a Client has an accepted portal user"
// shape a half-dozen package tests each declared their own copy of.
func SeedPortalUser(t *testing.T, db *DB, identityUID, clientID string) {
	t.Helper()
	SeedPortalAccount(t, db, identityUID, identityUID+"@example.com")
	AttachPortalUser(t, db, identityUID, clientID)
}

// SeedPendingPortalInvite inserts a client_portal_users row for clientID
// with no identity_uid, plus its portal_invite_outbox row (default status
// 'pending') -- mirroring what portalinvite.invite() leaves behind right
// after a Staff member sends an invite (#255): "invited, not yet
// accepted". The outbox row matters, not just the client_portal_users
// one -- PortalInviteStatus reads a portal user row with no outbox row
// at all the same as "never invited" (see its own doc comment), so a
// fixture missing it would silently fail any Send-precondition test that
// wants a genuinely pending invite. Returns the client_portal_users row's
// id.
func SeedPendingPortalInvite(t *testing.T, db *DB, clientID string) (clientPortalUserID string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO client_portal_users (client_id, invite_token) VALUES ($1, gen_random_uuid()) RETURNING id`,
		clientID,
	).Scan(&clientPortalUserID); err != nil {
		// coverage:ignore reason: fixture insert failure, not exercised by the happy-path test
		t.Fatalf("testdb: seed pending portal invite for client %q: %v", clientID, err)
	}
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO portal_invite_outbox (client_portal_user_id) VALUES ($1)`,
		clientPortalUserID,
	); err != nil {
		// coverage:ignore reason: fixture insert failure, not exercised by the happy-path test
		t.Fatalf("testdb: seed portal invite outbox for client %q: %v", clientID, err)
	}
	return clientPortalUserID
}

// SeedPushSubscription inserts a push_subscriptions row for
// ownerType/ownerID, using the superuser Admin connection. p256dh_key and
// auth_key are fixed placeholder values -- no caller asserts on them,
// only on the row's existence and its endpoint.
func SeedPushSubscription(t *testing.T, db *DB, ownerType, ownerID, endpoint string) (id string) {
	t.Helper()
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO push_subscriptions (owner_type, owner_id, endpoint, p256dh_key, auth_key)
		 VALUES ($1, $2, $3, 'p256dh-key', 'auth-key') RETURNING id`,
		ownerType, ownerID, endpoint,
	).Scan(&id); err != nil {
		// coverage:ignore reason: fixture insert failure, not exercised by the happy-path test
		t.Fatalf("testdb: seed push subscription: %v", err)
	}
	return id
}
