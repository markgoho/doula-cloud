package offer_test

import (
	"testing"
	"time"

	"doula-cloud/api/internal/offer"
	"doula-cloud/api/internal/testdb"
)

// TestOfferStateAt_AgreesWithStateAt proves 00174's offer_state_at and
// offer_open_at say what offer.StateAt and offer.OpenAt say, for every
// state and either side of the edge -- the SQL and the Go are one rule
// written twice, and this is what keeps them one.
func TestOfferStateAt_AgreesWithStateAt(t *testing.T) {
	f := newFixture(t)
	expiresAt := time.Date(2027, 1, 8, 12, 0, 0, 0, time.UTC)
	states := []offer.State{
		offer.StateOffered, offer.StateAccepted, offer.StateDeclined,
		offer.StateWithdrawn, offer.StateSuperseded, offer.StateExpired,
	}
	for _, stored := range states {
		for _, at := range []time.Time{expiresAt.Add(-time.Microsecond), expiresAt, expiresAt.Add(time.Microsecond)} {
			var got offer.State
			var open bool
			if err := f.db.Admin.QueryRowContext(t.Context(),
				`SELECT offer_state_at($1::offer_state, $2, $3)::text, offer_open_at($1::offer_state, $2, $3)`,
				string(stored), expiresAt, at,
			).Scan(&got, &open); err != nil {
				t.Fatalf("offer_state_at: %v", err)
			}
			if want := offer.StateAt(stored, expiresAt, at); got != want {
				t.Errorf("offer_state_at(%q, %v, %v) = %q, StateAt says %q", stored, expiresAt, at, got, want)
			}
			if want := offer.OpenAt(stored, expiresAt, at); open != want {
				t.Errorf("offer_open_at(%q, %v, %v) = %v, OpenAt says %v", stored, expiresAt, at, open, want)
			}
		}
	}
}

// TestSchema_RefusesTheIllegalCombinations proves
// engagement_offers_lifecycle (00174) holds even for a writer that
// bypasses every route and every policy: the superuser connection. Each
// state carries exactly the who and when it means.
func TestSchema_RefusesTheIllegalCombinations(t *testing.T) {
	f := newFixture(t)
	offerID := f.makeOffer(t, offerBody(f.doulaID, 45000))

	refused := map[string]string{
		"a decision with no instant": `UPDATE engagement_offers SET state = 'declined', decided_by = staff_id, decided_by_kind = 'staff' WHERE id = $1`,
		"a decision with no decider": `UPDATE engagement_offers SET state = 'declined', decided_at = offered_at WHERE id = $1`,
		"a staff decision naming nobody": `UPDATE engagement_offers
		    SET state = 'accepted', decided_at = offered_at, decided_by_kind = 'staff' WHERE id = $1`,
		"a decision once the Offer had run out": `UPDATE engagement_offers
		    SET state = 'accepted', decided_at = expires_at, decided_by = staff_id, decided_by_kind = 'staff' WHERE id = $1`,
		"an invitee accepting": `UPDATE engagement_offers
		    SET state = 'accepted', decided_at = offered_at, decided_by_kind = 'invitee' WHERE id = $1`,
		"an invitee naming a Staff member": `UPDATE engagement_offers
		    SET state = 'declined', decided_at = offered_at, decided_by = staff_id, decided_by_kind = 'invitee' WHERE id = $1`,
		"a completion declining": `UPDATE engagement_offers
		    SET state = 'declined', decided_at = offered_at, decided_by_kind = 'completion' WHERE id = $1`,
		"a lapse dated after it lapsed": `UPDATE engagement_offers
		    SET state = 'expired', decided_at = expires_at + interval '1 day', decided_by_kind = 'lapse' WHERE id = $1`,
		"a lapse with no decider": `UPDATE engagement_offers SET state = 'expired', decided_at = expires_at WHERE id = $1`,
		"a lapse decided by a Staff member": `UPDATE engagement_offers
		    SET state = 'expired', decided_at = expires_at, decided_by = staff_id, decided_by_kind = 'staff' WHERE id = $1`,
		"a withdrawal called a lapse": `UPDATE engagement_offers
		    SET state = 'withdrawn', decided_at = offered_at, decided_by_kind = 'lapse' WHERE id = $1`,
		"an open Offer with a decider":  `UPDATE engagement_offers SET decided_by_kind = 'staff', decided_by = staff_id WHERE id = $1`,
		"an open Offer with an instant": `UPDATE engagement_offers SET decided_at = offered_at WHERE id = $1`,
	}
	for name, query := range refused {
		if _, err := f.db.Admin.ExecContext(t.Context(), query, offerID); err == nil {
			t.Errorf("%s: the write succeeded, want the schema to refuse it", name)
		}
	}

	// And the legal ones still pass, each on an Offer of its own.
	legal := map[string]string{
		"a Staff member's decision": `UPDATE engagement_offers
		    SET state = 'superseded', decided_at = offered_at, decided_by = staff_id, decided_by_kind = 'staff' WHERE id = $1`,
		"an invitee's decline": `UPDATE engagement_offers
		    SET state = 'declined', decided_at = offered_at, decided_by_kind = 'invitee' WHERE id = $1`,
		"the completion's withdrawal": `UPDATE engagement_offers
		    SET state = 'withdrawn', decided_at = offered_at, decided_by_kind = 'completion' WHERE id = $1`,
		"a lapse at its expires_at": `UPDATE engagement_offers
		    SET state = 'expired', decided_at = expires_at, decided_by_kind = 'lapse' WHERE id = $1`,
	}
	for name, query := range legal {
		staffID := testdb.SeedStaffAtPractice(t, f.db, f.practiceID, "uid-"+name, []string{doulaRole}, contractorType)
		id := f.makeOffer(t, offerBody(staffID, 45000))
		if _, err := f.db.Admin.ExecContext(t.Context(), query, id); err != nil {
			t.Errorf("%s was refused: %v", name, err)
		}
	}
}
