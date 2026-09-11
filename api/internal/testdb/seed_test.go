package testdb_test

import (
	"database/sql"
	"testing"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/testdb"
)

// doulaRole is named once so golangci-lint's goconst check doesn't see
// three independent "doula" literals across this file's tests.
const doulaRole = "doula"

// namespacedCamille is the Portal Account identifier TestPortalUID
// expects both ways round, named once for the same goconst reason.
const namespacedCamille = "portal_camille"

// TestPortalUID proves the fixture normalizer is idempotent: it puts the
// Portal Account namespace in front of a bare name, and leaves an
// identifier that already carries it alone. Both halves matter -- a
// caller passing portalaccount.NewIdentifier()'s own output must not end
// up wearing the prefix twice, and #1024's tier check reads that prefix
// and nothing else.
func TestPortalUID(t *testing.T) {
	if got := testdb.PortalUID("camille"); got != namespacedCamille {
		t.Errorf("PortalUID(%q) = %q, want %q", "camille", got, namespacedCamille)
	}
	if got := testdb.PortalUID(namespacedCamille); got != namespacedCamille {
		t.Errorf("PortalUID(%q) = %q, want it unchanged", namespacedCamille, got)
	}
}

// TestSeedPractice proves the Practice row lands with the name passed
// in -- the fixture every other seed helper in this file builds on.
func TestSeedPractice(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Seed Practice Test")

	var name string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT name FROM practices WHERE id = $1`, practiceID,
	).Scan(&name); err != nil {
		t.Fatalf("read seeded practice: %v", err)
	}
	if name != "Seed Practice Test" {
		t.Fatalf("name = %q, want Seed Practice Test", name)
	}
}

// TestSetPracticeTimezone proves the write lands directly, bypassing
// practicetimezone.PutHandler -- the fixture visit and payments both call
// to seed a Practice a particular zone (including one that will not
// load) rather than the default SeedPractice states (#1167).
func TestSetPracticeTimezone(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Set Practice Timezone Test")

	testdb.SetPracticeTimezone(t, db, practiceID, "America/Los_Angeles")

	var zone string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT timezone FROM practices WHERE id = $1`, practiceID,
	).Scan(&zone); err != nil {
		t.Fatalf("read practice timezone: %v", err)
	}
	if zone != "America/Los_Angeles" {
		t.Fatalf("timezone = %q, want America/Los_Angeles", zone)
	}
}

// TestSeedClientsCanPay proves the practices row flips to
// stripe_connect_card_payments_status = 'active', the one column
// payments.ClientsCanPay reads -- no stripe_connect_account_id, since
// this fixture is for the readers that only ever consult that column.
func TestSeedClientsCanPay(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Seed Clients Can Pay Test")

	testdb.SeedClientsCanPay(t, db, practiceID)

	var status string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT stripe_connect_card_payments_status FROM practices WHERE id = $1`, practiceID,
	).Scan(&status); err != nil {
		t.Fatalf("read seeded card payments status: %v", err)
	}
	if status != "active" {
		t.Fatalf("stripe_connect_card_payments_status = %q, want active", status)
	}
}

// TestSeedBillingMode proves the practices row's billing_mode is set
// directly, bypassing resolveBillingMode's own write path (#271).
func TestSeedBillingMode(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Seed Billing Mode Test")

	testdb.SeedBillingMode(t, db, practiceID, "by_hand")

	var mode string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT billing_mode FROM practices WHERE id = $1`, practiceID,
	).Scan(&mode); err != nil {
		t.Fatalf("read seeded billing mode: %v", err)
	}
	if mode != "by_hand" {
		t.Fatalf("billing_mode = %q, want by_hand", mode)
	}
}

// TestSeedStaffAtPractice proves SeedStaffAtPractice's Staff row and
// practice_memberships row land with the roles and employment type
// passed in -- every package composing seed helpers on top of this one
// relies on that.
func TestSeedStaffAtPractice(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Seed Test Practice")
	staffID := testdb.SeedStaffAtPractice(t, db, practiceID, "seed-test-staff", []string{"owner", doulaRole}, "contractor")

	var name, email string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT name, email FROM staff WHERE id = $1`, staffID,
	).Scan(&name, &email); err != nil {
		t.Fatalf("read seeded staff: %v", err)
	}
	if name != "Test Staff seed-test-staff" {
		t.Fatalf("name = %q, want derived from identityUID", name)
	}
	if email != "seed-test-staff@example.com" {
		t.Fatalf("email = %q, want derived from identityUID", email)
	}

	var roles, employmentType string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT roles::text, employment_type FROM practice_memberships WHERE practice_id = $1 AND staff_id = $2`,
		practiceID, staffID,
	).Scan(&roles, &employmentType); err != nil {
		t.Fatalf("read seeded membership: %v", err)
	}
	if roles != "{owner,doula}" {
		t.Fatalf("roles = %q, want {owner,doula}", roles)
	}
	if employmentType != "contractor" {
		t.Fatalf("employment_type = %q, want contractor", employmentType)
	}
}

// TestSeedNamedStaffAtPractice proves the display name passed in lands on
// the Staff row unchanged, for a test that asserts a specific name comes
// back.
func TestSeedNamedStaffAtPractice(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Named Seed Test Practice")
	staffID := testdb.SeedNamedStaffAtPractice(t, db, practiceID, "named-seed-test-staff", "Jamie Doula", []string{"doula"}, "employee")

	var name string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT name FROM staff WHERE id = $1`, staffID,
	).Scan(&name); err != nil {
		t.Fatalf("read seeded staff: %v", err)
	}
	if name != "Jamie Doula" {
		t.Fatalf("name = %q, want %q", name, "Jamie Doula")
	}
}

// TestRemoveMembership proves the undo half of SeedNamedStaffAtPractice
// leaves the staff row standing and takes only the Membership -- which is
// the whole point of it, since every policy that reaches a Staff row
// through a live Membership is what a departed-Staff test is exercising.
func TestRemoveMembership(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Removal Test Practice")
	staffID := testdb.SeedNamedStaffAtPractice(t, db, practiceID, "removal-test-staff", "Maya Okonkwo", []string{"doula"}, "employee")

	testdb.RemoveMembership(t, db, staffID)

	var memberships, staffRows int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM practice_memberships WHERE staff_id = $1`, staffID,
	).Scan(&memberships); err != nil {
		t.Fatalf("count memberships: %v", err)
	}
	if memberships != 0 {
		t.Fatalf("memberships = %d, want none left", memberships)
	}
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM staff WHERE id = $1`, staffID,
	).Scan(&staffRows); err != nil {
		t.Fatalf("count staff: %v", err)
	}
	if staffRows != 1 {
		t.Fatalf("staff rows = %d, want the person herself still on file", staffRows)
	}
}

// TestSeedPortalAccount proves the portal_accounts row lands with the
// identifier and sign-in address passed in -- every package seeding an
// accepted client_portal_users row relies on this existing first, or the
// identity_uid foreign key (#616) refuses the insert.
func TestSeedPortalAccount(t *testing.T) {
	db := testdb.New(t)
	testdb.SeedPortalAccount(t, db, "portal_seed-test", "seed-test@example.com")

	var signInAddress string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT sign_in_address FROM portal_accounts WHERE identifier = $1`, "portal_seed-test",
	).Scan(&signInAddress); err != nil {
		t.Fatalf("read seeded portal account: %v", err)
	}
	if signInAddress != "seed-test@example.com" {
		t.Fatalf("sign_in_address = %q, want %q", signInAddress, "seed-test@example.com")
	}
}

// TestSeedStaff proves the Staff row lands with the derived name and
// email, and no practice_memberships row, so a caller composing its own
// two-step staff-then-membership fixture doesn't find one already there.
func TestSeedStaff(t *testing.T) {
	db := testdb.New(t)
	staffID := testdb.SeedStaff(t, db, "seed-test-bare-staff")

	var name, email string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT name, email FROM staff WHERE id = $1`, staffID,
	).Scan(&name, &email); err != nil {
		t.Fatalf("read seeded staff: %v", err)
	}
	if name != "Test Staff seed-test-bare-staff" {
		t.Fatalf("name = %q, want derived from identityUID", name)
	}
	if email != "seed-test-bare-staff@example.com" {
		t.Fatalf("email = %q, want derived from identityUID", email)
	}

	var count int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM practice_memberships WHERE staff_id = $1`, staffID,
	).Scan(&count); err != nil {
		t.Fatalf("count memberships: %v", err)
	}
	if count != 0 {
		t.Fatalf("memberships = %d, want none for a bare SeedStaff", count)
	}
}

// TestRedactDeletedLogin proves the fixture leaves behind exactly the
// row shape staffauth.DeleteLoginHandler's own redaction does: the same
// row id, a 'deleted:<id>' identity_uid, a stamped deleted_at, and the
// two replacement strings. The outbox workers' skip-at-send rechecks
// (#892) are all written against this shape, so a drift between this
// fixture and the handler would make three packages' tests agree with
// each other and with nothing real.
func TestRedactDeletedLogin(t *testing.T) {
	db := testdb.New(t)
	staffID := testdb.SeedStaff(t, db, "seed-test-deleted-login")

	testdb.RedactDeletedLogin(t, db, staffID)

	var identityUID, name, email string
	var deleted bool
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT identity_uid, name, email, deleted_at IS NOT NULL FROM staff WHERE id = $1`, staffID,
	).Scan(&identityUID, &name, &email, &deleted); err != nil {
		t.Fatalf("read redacted staff: %v", err)
	}
	if identityUID != "deleted:"+staffID {
		t.Fatalf("identity_uid = %q, want the deleted:<id> sentinel", identityUID)
	}
	if !deleted {
		t.Fatal("deleted_at is still NULL, want it stamped")
	}
	if name != "Deleted Staff Member" || email != "deleted@deleted.invalid" {
		t.Fatalf("name/email = %q/%q, want the redaction's replacements", name, email)
	}
}

// TestSeedStaffAtNewPractice proves it seeds one fresh Practice and one
// Staff member on it with the roles and employment type given.
func TestSeedStaffAtNewPractice(t *testing.T) {
	db := testdb.New(t)
	practiceID, staffID := testdb.SeedStaffAtNewPractice(t, db, "seed-test-new-practice-owner", []string{"owner"}, "employee")

	var roles string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT roles::text FROM practice_memberships WHERE practice_id = $1 AND staff_id = $2`,
		practiceID, staffID,
	).Scan(&roles); err != nil {
		t.Fatalf("read seeded membership: %v", err)
	}
	if roles != "{owner}" {
		t.Fatalf("roles = %q, want {owner}", roles)
	}
}

// TestSeedContractorAtPractice proves the seeded Staff row carries the
// contractor Doula shape -- `doula` role, contractor employment -- at
// the given Practice.
func TestSeedContractorAtPractice(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Seed Contractor Test Practice")
	staffID := testdb.SeedContractorAtPractice(t, db, practiceID, "seed-test-contractor")

	var roles, employmentType string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT roles::text, employment_type FROM practice_memberships WHERE practice_id = $1 AND staff_id = $2`,
		practiceID, staffID,
	).Scan(&roles, &employmentType); err != nil {
		t.Fatalf("read seeded membership: %v", err)
	}
	if roles != "{doula}" {
		t.Fatalf("roles = %q, want {doula}", roles)
	}
	if employmentType != "contractor" {
		t.Fatalf("employment_type = %q, want contractor", employmentType)
	}
}

// TestSeedNamedClient proves the Client row lands with the given name
// and email, and that an empty email is stored as NULL rather than an
// empty string.
func TestSeedNamedClient(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Seed Named Client Test Practice")
	clientID := testdb.SeedNamedClient(t, db, practiceID, "Jamie Client", "jamie@example.com")

	var name string
	var email *string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT given_name, email FROM clients WHERE id = $1`, clientID,
	).Scan(&name, &email); err != nil {
		t.Fatalf("read seeded client: %v", err)
	}
	if name != "Jamie Client" {
		t.Fatalf("given_name = %q, want %q", name, "Jamie Client")
	}
	if email == nil || *email != "jamie@example.com" {
		t.Fatalf("email = %v, want jamie@example.com", email)
	}

	noEmailID := testdb.SeedNamedClient(t, db, practiceID, "No Email Client", "")
	var noEmail *string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT email FROM clients WHERE id = $1`, noEmailID,
	).Scan(&noEmail); err != nil {
		t.Fatalf("read seeded no-email client: %v", err)
	}
	if noEmail != nil {
		t.Fatalf("email = %q, want NULL for an empty email argument", *noEmail)
	}
}

// TestSeedNamedEngagement proves the Client's name and email land as
// given, and the Engagement defaults to the schema's "intake" status.
func TestSeedNamedEngagement(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Seed Named Engagement Test Practice")
	clientID, engagementID := testdb.SeedNamedEngagement(t, db, practiceID, "Jordan Client", "jordan@example.com")

	var name, status string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT c.given_name, e.status::text FROM clients c JOIN engagements e ON e.client_id = c.id WHERE c.id = $1 AND e.id = $2`,
		clientID, engagementID,
	).Scan(&name, &status); err != nil {
		t.Fatalf("read seeded client and engagement: %v", err)
	}
	if name != "Jordan Client" {
		t.Fatalf("given_name = %q, want %q", name, "Jordan Client")
	}
	if status != "intake" {
		t.Fatalf("status = %q, want intake (the schema default)", status)
	}
}

// TestSeedEngagementInStatus proves the Engagement lands at the explicit
// status given, not the schema's default.
func TestSeedEngagementInStatus(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Seed Engagement In Status Test Practice")
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Robin Client", "robin@example.com", "active")

	var status string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status::text FROM engagements WHERE id = $1`, engagementID,
	).Scan(&status); err != nil {
		t.Fatalf("read seeded engagement: %v", err)
	}
	if status != "active" {
		t.Fatalf("status = %q, want active", status)
	}
}

// TestSeedBirthOutcome proves the helper puts the dateless 'unknown' on
// an Engagement that has none -- the state #940's
// engagements_completed_is_explained demands before a completion is
// allowed through, which is why two other packages' tests call this.
func TestSeedBirthOutcome(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Seed Birth Outcome Test Practice")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)

	testdb.SeedBirthOutcome(t, db, engagementID)

	var outcome string
	var endedOn *string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT birth_outcome::text, pregnancy_ended_on::text FROM engagements WHERE id = $1`, engagementID,
	).Scan(&outcome, &endedOn); err != nil {
		t.Fatalf("read seeded engagement: %v", err)
	}
	if outcome != "unknown" || endedOn != nil {
		t.Fatalf("outcome=%q endedOn=%v, want unknown with no date", outcome, endedOn)
	}
}

// TestSeedEngagementInStatus_CompletedSetsBothFacts proves a "completed"
// status also sets ending_reason to 'care_complete' and birth_outcome to
// 'unknown' -- #940's engagements_completed_is_explained CHECK demands
// both on every completed row, so a caller of this fixture must not have
// to know that just to get one. 'unknown' with no date is the pair the
// CHECK's neighbor engagements_outcome_is_dated (00093) accepts without
// a pregnancy_ended_on, which is why the fixture invents no date.
func TestSeedEngagementInStatus_CompletedSetsBothFacts(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Seed Completed Engagement Test Practice")
	_, engagementID := testdb.SeedEngagementInStatus(t, db, practiceID, "Sam Client", "sam@example.com", "completed")

	var status, endingReason, birthOutcome string
	var endedOn *string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT status::text, ending_reason::text, birth_outcome::text, pregnancy_ended_on::text
		   FROM engagements WHERE id = $1`, engagementID,
	).Scan(&status, &endingReason, &birthOutcome, &endedOn); err != nil {
		t.Fatalf("read seeded engagement: %v", err)
	}
	if status != "completed" {
		t.Fatalf("status = %q, want completed", status)
	}
	if endingReason != "care_complete" {
		t.Fatalf("ending_reason = %q, want care_complete", endingReason)
	}
	if birthOutcome != "unknown" {
		t.Fatalf("birth_outcome = %q, want unknown", birthOutcome)
	}
	if endedOn != nil {
		t.Fatalf("pregnancy_ended_on = %q, want no date", *endedOn)
	}
}

// TestSeedEngagementWithKind proves the Engagement lands with the
// explicit kind given, not the 'birth' every other seed helper here
// hardcodes.
func TestSeedEngagementWithKind(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Seed Engagement With Kind Test Practice")
	_, engagementID := testdb.SeedEngagementWithKind(t, db, practiceID, "Sasha Client", "sasha@example.com", "postpartum")

	var kind string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT kind::text FROM engagements WHERE id = $1`, engagementID,
	).Scan(&kind); err != nil {
		t.Fatalf("read seeded engagement: %v", err)
	}
	if kind != "postpartum" {
		t.Fatalf("kind = %q, want postpartum", kind)
	}
}

// TestSeedActivity proves the row it writes lands through the real
// activity.Record path, readable back with the subject and action given.
func TestSeedActivity(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Seed Activity Test Practice")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	staffID := testdb.SeedStaffAtPractice(t, db, practiceID, "seed-activity-staff", []string{doulaRole}, "employee")

	testdb.SeedActivity(t, db, practiceID, activity.SubjectEngagement, engagementID, "seed_test_action", activity.StaffActor(staffID))

	var action, actorKind, actorStaffID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT action, actor_kind::text, actor_staff_id FROM activity WHERE practice_id = $1 AND subject_id = $2`,
		practiceID, engagementID,
	).Scan(&action, &actorKind, &actorStaffID); err != nil {
		t.Fatalf("read seeded activity: %v", err)
	}
	if action != "seed_test_action" {
		t.Fatalf("action = %q, want seed_test_action", action)
	}
	if actorKind != "staff" {
		t.Fatalf("actor_kind = %q, want staff", actorKind)
	}
	if actorStaffID != staffID {
		t.Fatalf("actor_staff_id = %q, want %q", actorStaffID, staffID)
	}
}

// TestSeedGrantedAttachment proves it seeds an open (unended) attachment
// with origin "granted".
func TestSeedGrantedAttachment(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Seed Granted Attachment Test Practice")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	staffID := testdb.SeedStaffAtPractice(t, db, practiceID, "seed-granted-attachment-staff", []string{doulaRole}, "contractor")

	testdb.SeedGrantedAttachment(t, db, engagementID, staffID)

	var origin string
	var endedAt *string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT origin::text, ended_at::text FROM engagement_attachments WHERE engagement_id = $1 AND staff_id = $2`,
		engagementID, staffID,
	).Scan(&origin, &endedAt); err != nil {
		t.Fatalf("read seeded attachment: %v", err)
	}
	if origin != "granted" {
		t.Fatalf("origin = %q, want granted", origin)
	}
	if endedAt != nil {
		t.Fatalf("ended_at = %v, want NULL (open)", *endedAt)
	}
}

// TestPracticeOfClient proves the read-back helper answers with the
// Practice the Client was seeded under. The fixture builders hand back
// the Client and not the Practice behind it, so a test that needs both
// -- clientauth's two-Practice ledger case, portalinvite's same-Practice
// reuse refusal -- asks this rather than declaring its own copy of the
// query.
func TestPracticeOfClient(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Practice Of Client Test Practice")
	clientID, _ := testdb.SeedEngagement(t, db, practiceID)

	if got := testdb.PracticeOfClient(t, db, clientID); got != practiceID {
		t.Fatalf("PracticeOfClient = %q, want the Practice the Client was seeded under, %q", got, practiceID)
	}
}

// TestSeedEngagement proves the Client and Engagement it inserts are
// actually linked to each other and to practiceID -- every package
// deciding Engagement-scoped access against a fixture relies on that
// shape (#706 collapsed the near-identical per-package copies into this).
func TestSeedEngagement(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Seed Engagement Test Practice")
	clientID, engagementID := testdb.SeedEngagement(t, db, practiceID)

	var gotClientID, gotPracticeID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT client_id, practice_id FROM engagements WHERE id = $1`, engagementID,
	).Scan(&gotClientID, &gotPracticeID); err != nil {
		t.Fatalf("read seeded engagement: %v", err)
	}
	if gotClientID != clientID {
		t.Fatalf("engagement.client_id = %q, want the seeded Client %q", gotClientID, clientID)
	}
	if gotPracticeID != practiceID {
		t.Fatalf("engagement.practice_id = %q, want %q", gotPracticeID, practiceID)
	}

	var clientPracticeID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT practice_id FROM clients WHERE id = $1`, clientID,
	).Scan(&clientPracticeID); err != nil {
		t.Fatalf("read seeded client: %v", err)
	}
	if clientPracticeID != practiceID {
		t.Fatalf("client.practice_id = %q, want %q", clientPracticeID, practiceID)
	}
}

// TestSeedAttachment proves origin and ended_at land as given -- both
// origins, both open and ended -- the four combinations ADR-0008's
// attachment-narrowing tests distinguish between.
func TestSeedAttachment(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Seed Attachment Test Practice")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	staffID := testdb.SeedStaffAtPractice(t, db, practiceID, "seed-attachment-staff", []string{doulaRole}, "contractor")

	testdb.SeedAttachment(t, db, engagementID, staffID, "granted", false)

	var origin string
	var endedAt *string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT origin::text, ended_at::text FROM engagement_attachments WHERE engagement_id = $1 AND staff_id = $2`,
		engagementID, staffID,
	).Scan(&origin, &endedAt); err != nil {
		t.Fatalf("read seeded attachment: %v", err)
	}
	if origin != "granted" {
		t.Fatalf("origin = %q, want granted", origin)
	}
	if endedAt != nil {
		t.Fatalf("ended_at = %v, want NULL (open)", *endedAt)
	}
}

// TestSeedAttachment_Ended proves ended=true actually sets ended_at,
// rather than leaving the attachment open.
func TestSeedAttachment_Ended(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Seed Ended Attachment Test Practice")
	_, engagementID := testdb.SeedEngagement(t, db, practiceID)
	staffID := testdb.SeedStaffAtPractice(t, db, practiceID, "seed-ended-attachment-staff", []string{doulaRole}, "contractor")

	testdb.SeedAttachment(t, db, engagementID, staffID, "accrued", true)

	var origin string
	var endedAt *string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT origin::text, ended_at::text FROM engagement_attachments WHERE engagement_id = $1 AND staff_id = $2`,
		engagementID, staffID,
	).Scan(&origin, &endedAt); err != nil {
		t.Fatalf("read seeded attachment: %v", err)
	}
	if origin != "accrued" {
		t.Fatalf("origin = %q, want accrued", origin)
	}
	if endedAt == nil {
		t.Fatal("ended_at = NULL, want set (ended)")
	}
}

// TestSeedPortalUser proves it mints a fresh Portal Account and attaches
// it to clientID in one call.
func TestSeedPortalUser(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Seed Portal User Test Practice")
	clientID := testdb.SeedNamedClient(t, db, practiceID, "Portal Test Client", "portal-user-client@example.com")

	testdb.SeedPortalUser(t, db, "seed-portal-user-uid", clientID)

	var identityUID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT identity_uid FROM client_portal_users WHERE client_id = $1`, clientID,
	).Scan(&identityUID); err != nil {
		t.Fatalf("read seeded client_portal_users row: %v", err)
	}
	if identityUID != "seed-portal-user-uid" {
		t.Fatalf("identity_uid = %q, want %q", identityUID, "seed-portal-user-uid")
	}
}

// TestSeedPendingPortalInvite proves both rows land: client_portal_users
// with no identity_uid, and its own portal_invite_outbox row -- the pair
// portalinvite.invite() leaves behind, distinct from TestSeedPortalUser's
// accepted (identity_uid set) shape above.
func TestSeedPendingPortalInvite(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Seed Pending Portal Invite Test Practice")
	clientID := testdb.SeedNamedClient(t, db, practiceID, "Pending Invite Client", "pending-invite-client@example.com")

	clientPortalUserID := testdb.SeedPendingPortalInvite(t, db, clientID)

	var identityUID sql.NullString
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT identity_uid FROM client_portal_users WHERE id = $1 AND client_id = $2`,
		clientPortalUserID, clientID,
	).Scan(&identityUID); err != nil {
		t.Fatalf("read seeded client_portal_users row: %v", err)
	}
	if identityUID.Valid {
		t.Fatalf("identity_uid = %q, want NULL (pending, not accepted)", identityUID.String)
	}

	var outboxCount int
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT count(*) FROM portal_invite_outbox WHERE client_portal_user_id = $1`,
		clientPortalUserID,
	).Scan(&outboxCount); err != nil {
		t.Fatalf("count seeded outbox rows: %v", err)
	}
	if outboxCount != 1 {
		t.Fatalf("outbox rows = %d, want 1", outboxCount)
	}
}

// TestSeedPracticeRate proves the row lands for the given kind and
// amount, readable back the way practicerate.GetRatesHandler reads it.
func TestSeedPracticeRate(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Seed Practice Rate Test Practice")

	testdb.SeedPracticeRate(t, db, practiceID, "birth", 15000)

	var amountCents int64
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT amount_cents FROM practice_rates WHERE practice_id = $1 AND kind = 'birth'::engagement_kind`, practiceID,
	).Scan(&amountCents); err != nil {
		t.Fatalf("read seeded practice rate: %v", err)
	}
	if amountCents != 15000 {
		t.Fatalf("amount_cents = %d, want 15000", amountCents)
	}
}

// TestSeedPushSubscription proves the row lands with the given endpoint,
// readable back by its returned id.
func TestSeedPushSubscription(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Seed Push Subscription Test Practice")
	staffID := testdb.SeedStaffAtPractice(t, db, practiceID, "seed-push-staff", []string{doulaRole}, "employee")

	id := testdb.SeedPushSubscription(t, db, "staff", staffID, "https://push.example.com/seed-test")

	var endpoint string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT endpoint FROM push_subscriptions WHERE id = $1`, id,
	).Scan(&endpoint); err != nil {
		t.Fatalf("read seeded push subscription: %v", err)
	}
	if endpoint != "https://push.example.com/seed-test" {
		t.Fatalf("endpoint = %q, want the endpoint given", endpoint)
	}
}

// TestAttachPortalUser proves the client_portal_users row lands pointed
// at the given Portal Account and Client -- the shape a multi-Practice
// Portal Account's second (and later) row takes (#309, ADR-0015).
func TestAttachPortalUser(t *testing.T) {
	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Attach Test Practice")
	testdb.SeedPortalAccount(t, db, "portal_attach-test", "attach-test@example.com")

	var clientID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`INSERT INTO clients (practice_id, given_name, email) VALUES ($1, 'Attach Test Client', 'client@example.com') RETURNING id`,
		practiceID,
	).Scan(&clientID); err != nil {
		t.Fatalf("seed client: %v", err)
	}

	testdb.AttachPortalUser(t, db, "portal_attach-test", clientID)

	var identityUID string
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT identity_uid FROM client_portal_users WHERE client_id = $1`, clientID,
	).Scan(&identityUID); err != nil {
		t.Fatalf("read attached client_portal_users row: %v", err)
	}
	if identityUID != "portal_attach-test" {
		t.Fatalf("identity_uid = %q, want %q", identityUID, "portal_attach-test")
	}
}
