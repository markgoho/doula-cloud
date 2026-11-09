package offer_test

import (
	"testing"

	"doula-cloud/api/internal/outbox"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// These tests exercise 00041's two new policies on engagement_offers
// directly through db.App and set_config, rather than through a handler
// -- proving the SQL scopes visibility itself, not merely agreeing with
// the Go checks by coincidence.

const countOffersQuery = `SELECT count(*) FROM engagement_offers WHERE id = $1`

// With no session variable set at all, an Offer is invisible: neither the
// practice-tier policy, the token door, nor the worker door matches.
func TestRLS_OffersFailClosedWithNothingSet(t *testing.T) {
	f := newFixture(t)
	f.makeOffer(t, offerBody(f.doulaID, 45000))

	testdb.RequireFailsClosed(t, f.db, "engagement_offers")
}

// The token door opens exactly the Offers whose Invitation carries the
// presented token -- and nothing else, not even another Offer at the same
// Practice.
func TestRLS_TokenDoorOpensOnlyItsOwnOffers(t *testing.T) {
	f := newFixture(t)
	emailOfferID, token, _ := seedEmailOffer(t, f)
	f.makeOffer(t, offerBody(f.doulaID, 45000))

	tx := f.db.BeginAs(t, testdb.SessionVar{Name: "app.invite_token_digest", Value: staffauth.TokenDigest(token)})

	visible := testdb.Column[string](t, tx, `SELECT id FROM engagement_offers`)
	if len(visible) != 1 || visible[0] != emailOfferID {
		t.Fatalf("visible through the token door = %v, want only %q", visible, emailOfferID)
	}
}

// The notification worker's door is read-and-stamp only, and it opens
// nothing without the trusted flag the process-outbox endpoint sets.
func TestRLS_WorkerDoorNeedsTheTrustedFlag(t *testing.T) {
	f := newFixture(t)
	offerID, _, _ := seedEmailOffer(t, f)

	if count := testdb.Scalar[int](t, f.db.App, countOffersQuery, offerID); count != 0 {
		t.Fatalf("offers visible without the flag = %d, want 0", count)
	}
	tx := f.db.BeginAs(t, testdb.SessionVar{Name: outbox.NotificationDoor, Value: "true"})
	if count := testdb.Scalar[int](t, tx, countOffersQuery, offerID); count != 1 {
		t.Fatalf("offers visible with the flag = %d, want 1", count)
	}
}

// A second Practice's Offers stay invisible under the practice-tier
// policy, the ordinary tenancy fence every other table follows.
func TestRLS_PracticeTierFencesAnotherPractice(t *testing.T) {
	f := newFixture(t)
	mine := f.makeOffer(t, offerBody(f.doulaID, 45000))

	otherPractice := testdb.SeedPractice(t, f.db, "Test Practice")
	otherOwner := testdb.SeedStaffAtPractice(t, f.db, otherPractice, "uid-other-owner", []string{ownerRole}, employeeType)
	otherDoula := testdb.SeedStaffAtPractice(t, f.db, otherPractice, "uid-other-doula", []string{doulaRole}, contractorType)
	_, otherEngagement := testdb.SeedEngagement(t, f.db, otherPractice)
	if _, err := f.db.Admin.ExecContext(t.Context(),
		`INSERT INTO engagement_offers
		     (engagement_id, staff_id, employment_type, amount_cents, client_first_initial, client_area,
		      due_date, offered_by, expires_at)
		 VALUES ($1, $2, 'contractor', 45000, 'R', 'Elsewhere', now() + interval '90 days', $3, now() + interval '7 days')`,
		otherEngagement, otherDoula, otherOwner,
	); err != nil {
		t.Fatalf("seed other practice offer: %v", err)
	}

	tx := f.db.BeginAs(t, testdb.Practice(f.practiceID))

	visible := testdb.Column[string](t, tx, `SELECT id::text FROM engagement_offers`)
	if len(visible) != 1 || visible[0] != mine {
		t.Fatalf("visible = %v, want only this practice's %q", visible, mine)
	}
}
