package offer_test

import (
	"testing"

	"doula-cloud/api/internal/staffauth"
)

// These tests exercise 00041's two new policies on engagement_offers
// directly through db.App and set_config, rather than through a handler
// -- proving the SQL scopes visibility itself, not merely agreeing with
// the Go checks by coincidence.

// With no session variable set at all, an Offer is invisible: neither the
// practice-tier policy, the token door, nor the worker door matches.
func TestRLS_OffersFailClosedWithNothingSet(t *testing.T) {
	f := newFixture(t)
	f.makeOffer(t, offerBody(f.doulaID, 45000))

	var count int
	if err := f.db.App.QueryRowContext(t.Context(), `SELECT count(*) FROM engagement_offers`).Scan(&count); err != nil {
		t.Fatalf("count offers: %v", err)
	}
	if count != 0 {
		t.Fatalf("offers visible with no context = %d, want 0", count)
	}
}

// The token door opens exactly the Offers whose Invitation carries the
// presented token -- and nothing else, not even another Offer at the same
// Practice.
func TestRLS_TokenDoorOpensOnlyItsOwnOffers(t *testing.T) {
	f := newFixture(t)
	emailOfferID, token, _ := seedEmailOffer(t, f)
	f.makeOffer(t, offerBody(f.doulaID, 45000))

	tx, err := f.db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(t.Context(),
		`SELECT set_config('app.invite_token_digest', $1, true)`, staffauth.TokenDigest(token),
	); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	rows, err := tx.QueryContext(t.Context(), `SELECT id FROM engagement_offers`)
	if err != nil {
		t.Fatalf("select offers: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var visible []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		visible = append(visible, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate: %v", err)
	}
	if len(visible) != 1 || visible[0] != emailOfferID {
		t.Fatalf("visible through the token door = %v, want only %q", visible, emailOfferID)
	}
}

// The notification worker's door is read-and-stamp only, and it opens
// nothing without the trusted flag the process-outbox endpoint sets.
func TestRLS_WorkerDoorNeedsTheTrustedFlag(t *testing.T) {
	f := newFixture(t)
	offerID, _, _ := seedEmailOffer(t, f)

	tx, err := f.db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	var count int
	if err := tx.QueryRowContext(t.Context(),
		`SELECT count(*) FROM engagement_offers WHERE id = $1`, offerID).Scan(&count); err != nil {
		t.Fatalf("count before: %v", err)
	}
	if count != 0 {
		t.Fatalf("offers visible without the flag = %d, want 0", count)
	}

	if _, err := tx.ExecContext(t.Context(),
		`SELECT set_config('app.notification_worker_trusted', 'true', true)`); err != nil {
		t.Fatalf("set_config: %v", err)
	}
	if err := tx.QueryRowContext(t.Context(),
		`SELECT count(*) FROM engagement_offers WHERE id = $1`, offerID).Scan(&count); err != nil {
		t.Fatalf("count after: %v", err)
	}
	if count != 1 {
		t.Fatalf("offers visible with the flag = %d, want 1", count)
	}
}

// A second Practice's Offers stay invisible under the practice-tier
// policy, the ordinary tenancy fence every other table follows.
func TestRLS_PracticeTierFencesAnotherPractice(t *testing.T) {
	f := newFixture(t)
	mine := f.makeOffer(t, offerBody(f.doulaID, 45000))

	otherPractice := seedPractice(t, f.db)
	otherOwner := seedMember(t, f.db, otherPractice, "uid-other-owner", []string{ownerRole}, employeeType)
	otherDoula := seedMember(t, f.db, otherPractice, "uid-other-doula", []string{doulaRole}, contractorType)
	otherEngagement := seedEngagement(t, f.db, otherPractice)
	if _, err := f.db.Admin.ExecContext(t.Context(),
		`INSERT INTO engagement_offers
		     (engagement_id, staff_id, employment_type, amount_cents, client_first_initial, client_area,
		      due_date, offered_by, expires_at)
		 VALUES ($1, $2, 'contractor', 45000, 'R', 'Elsewhere', now() + interval '90 days', $3, now() + interval '7 days')`,
		otherEngagement, otherDoula, otherOwner,
	); err != nil {
		t.Fatalf("seed other practice offer: %v", err)
	}

	tx, err := f.db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(t.Context(),
		`SELECT set_config('app.current_practice_id', $1, true)`, f.practiceID); err != nil {
		t.Fatalf("set_config: %v", err)
	}

	var visible string
	var count int
	if err := tx.QueryRowContext(t.Context(),
		`SELECT count(*), max(id::text) FROM engagement_offers`).Scan(&count, &visible); err != nil {
		t.Fatalf("count offers: %v", err)
	}
	if count != 1 || visible != mine {
		t.Fatalf("visible = %d rows (%q), want only this practice's %q", count, visible, mine)
	}
}
