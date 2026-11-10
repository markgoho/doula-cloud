package offer_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/lifecycletest"
	"doula-cloud/api/internal/offer"
	"doula-cloud/api/internal/testdb"
)

// TestStateAt_LapseIsDerivedAtExpiresAt pins the one definition of what
// an Offer is: an 'offered' row is expired from its expires_at on, and a
// row already closed keeps its state whatever the clock says.
func TestStateAt_LapseIsDerivedAtExpiresAt(t *testing.T) {
	expiresAt := time.Date(2027, 1, 8, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		stored offer.State
		at     time.Time
		want   offer.State
	}{
		{offer.StateOffered, expiresAt.Add(-time.Microsecond), offer.StateOffered},
		{offer.StateOffered, expiresAt, offer.StateExpired},
		{offer.StateOffered, expiresAt.Add(time.Hour), offer.StateExpired},
		{offer.StateAccepted, expiresAt.Add(time.Hour), offer.StateAccepted},
		{offer.StateDeclined, expiresAt.Add(time.Hour), offer.StateDeclined},
		{offer.StateWithdrawn, expiresAt.Add(-time.Hour), offer.StateWithdrawn},
		{offer.StateSuperseded, expiresAt.Add(time.Hour), offer.StateSuperseded},
		{offer.StateExpired, expiresAt.Add(-time.Hour), offer.StateExpired},
	}
	for _, c := range cases {
		got := offer.StateAt(c.stored, expiresAt, c.at)
		if got != c.want {
			t.Errorf("StateAt(%q, expires %v, at %v) = %q, want %q", c.stored, expiresAt, c.at, got, c.want)
		}
		if open := offer.OpenAt(c.stored, expiresAt, c.at); open != (c.want == offer.StateOffered) {
			t.Errorf("OpenAt(%q, expires %v, at %v) = %v, want %v", c.stored, expiresAt, c.at, open, !open)
		}
	}
}

// TestTransitions_TableIsComplete proves Transitions carries exactly the
// declared moves, each starting from the states its doc comment names,
// ending where it says, and recording the decider
// engagement_offers_lifecycle admits for that state.
func TestTransitions_TableIsComplete(t *testing.T) {
	open := []offer.State{offer.StateOffered}
	wants := []offer.Transition{
		{Act: "offer this work", To: offer.StateOffered},
		{Act: "accept it", Requires: open, To: offer.StateAccepted, By: offer.DecidedByStaff},
		{Act: "decline it", Requires: open, To: offer.StateDeclined, By: offer.DecidedByStaff},
		{Act: "decline it", Requires: open, To: offer.StateDeclined, By: offer.DecidedByInvitee},
		{Act: "withdraw it", Requires: open, To: offer.StateWithdrawn, By: offer.DecidedByStaff},
		{Act: "give it to someone else", Requires: open, To: offer.StateSuperseded, By: offer.DecidedByStaff},
		{Act: "withdraw it", Requires: open, To: offer.StateWithdrawn, By: offer.DecidedByCompletion},
		{Act: "let it lapse", Requires: []offer.State{offer.StateExpired}, To: offer.StateExpired, By: offer.DecidedByLapse},
	}
	if len(offer.Transitions) != len(wants) {
		t.Fatalf("len(Transitions) = %d, want %d", len(offer.Transitions), len(wants))
	}
	for i, got := range offer.Transitions {
		if fmt.Sprint(got) != fmt.Sprint(wants[i]) {
			t.Errorf("Transitions[%d] = %+v, want %+v", i, got, wants[i])
		}
	}
}

// TestTransition_CheckSpeaksToThePerson proves Check's two outcomes, and
// that a refusal is the sentence a person reads: what the Offer is, and
// what that stops.
func TestTransition_CheckSpeaksToThePerson(t *testing.T) {
	if ok, refusal := offer.TransitionAccept.Check(offer.StateOffered); !ok || refusal != "" {
		t.Fatalf("Accept.Check(offered) = (%v, %q), want (true, \"\")", ok, refusal)
	}
	lifecycletest.RequireRefusals(t, []lifecycletest.Refusal[offer.Transition, offer.State]{
		{Transition: offer.TransitionAccept, Current: offer.StateExpired, Want: "This Offer has expired, so you cannot accept it."},
		{Transition: offer.TransitionAccept, Current: offer.StateSuperseded, Want: "This work has gone to someone else, so you cannot accept it."},
		{Transition: offer.TransitionDecline, Current: offer.StateAccepted, Want: "This Offer has been accepted, so you cannot decline it."},
		{Transition: offer.TransitionDeclineByToken, Current: offer.StateWithdrawn, Want: "This Offer has been withdrawn, so you cannot decline it."},
		{Transition: offer.TransitionWithdraw, Current: offer.StateDeclined, Want: "This Offer has been declined, so you cannot withdraw it."},
		{Transition: offer.TransitionLapse, Current: offer.StateOffered, Want: "This Offer is still open, so you cannot let it lapse."},
		{Transition: offer.TransitionAccept, Current: offer.State("archived"), Want: "This Offer is archived, so you cannot accept it."},
	})
}

// TestEveryOfferReaderAsksTheOneDerivation is the source-scan guardrail
// contracts/lifecycle_test.go holds over Contracts. No non-test file
// under internal/ that touches engagement_offers may write the state
// itself or spell its own idea of "open" -- a bare state = 'offered', a
// hand-written expires_at comparison, a Go comparison with "offered", or
// a decision dated by SQL now(). Those are the second definitions 00174
// removed: a guard without the expires_at half read a lapsed Offer as
// open whenever the lazy materialization had not run yet. Only
// lifecycle.go, which declares the moves, spells them. And each reader
// that judges openness must actually ask the derivation. Self-updating:
// a new file anywhere in internal/ is scanned the moment it exists.
func TestEveryOfferReaderAsksTheOneDerivation(t *testing.T) {
	ownDefinition := []string{
		"SET state", "state = 'offered'", "state <> 'offered'", "state != 'offered'",
		"expires_at >", "expires_at <", `== "offered"`, `!= "offered"`, "decided_at = now()",
	}
	asks := []string{"offer_open_at(", "offer_state_at(", "StateAt(", "OpenAt(", ".update("}
	// The files that decide whether an Offer is open, each of which must
	// ask the derivation.
	judges := map[string]bool{
		"create.go": false, "decide.go": false, "withdraw.go": false, "preaccount.go": false,
		"list.go": false, "nodoula.go": false, "target.go": false, "outbox.go": false,
		"staffnotice/staffnotice.go": false,
	}

	scanned := 0
	for file, text := range lifecycletest.Sources(t, "..", ".") {
		if file == "offer/lifecycle.go" {
			continue
		}
		key := strings.TrimPrefix(file, "offer/")
		if _, ok := judges[key]; ok {
			for _, a := range asks {
				if strings.Contains(text, a) {
					judges[key] = true
				}
			}
		}
		if !strings.Contains(text, "engagement_offers") {
			continue
		}
		scanned++
		for _, own := range ownDefinition {
			if strings.Contains(text, own) {
				t.Errorf("%s touches engagement_offers and spells %q itself -- ask offer_open_at / offer_state_at (or offer.StateAt), and move the state through an offer Transition", file, own)
			}
		}
	}
	if scanned == 0 {
		t.Fatal("no file under internal/ touches engagement_offers -- did the walk move?")
	}
	for path, asked := range judges {
		if !asked {
			t.Errorf("%s does not ask the derivation -- it judges whether an Offer is open and must use offer_open_at, offer_state_at, or offer.StateAt", path)
		}
	}
}

// decision is the who and when engagement_offers_lifecycle ties to an
// Offer's state, read straight from the row.
type decision struct {
	state     string
	decidedBy *string
	kind      *string
	decidedAt *time.Time
	expiresAt time.Time
}

func readDecision(t *testing.T, db *testdb.DB, offerID string) decision {
	t.Helper()
	var d decision
	if err := db.Admin.QueryRowContext(t.Context(),
		`SELECT state::text, decided_by::text, decided_by_kind::text, decided_at, expires_at
		   FROM engagement_offers WHERE id = $1`, offerID,
	).Scan(&d.state, &d.decidedBy, &d.kind, &d.decidedAt, &d.expiresAt); err != nil {
		t.Fatalf("read decision: %v", err)
	}
	return d
}

// wantLapsed fails unless d is a lapse recorded as one: expired, dated at
// its own expires_at, decided by the lapse and by no Staff member.
func wantLapsed(t *testing.T, d decision) {
	t.Helper()
	if d.state != stateExpired || d.kind == nil || *d.kind != string(offer.DecidedByLapse) || d.decidedBy != nil {
		t.Fatalf("lapsed offer = state %q, kind %v, decided_by %v; want expired, by the lapse, by nobody", d.state, d.kind, d.decidedBy)
	}
	if d.decidedAt == nil || !d.decidedAt.Equal(d.expiresAt) {
		t.Fatalf("lapsed offer decided_at = %v, want its expires_at %v", d.decidedAt, d.expiresAt)
	}
}

// TestAcceptHandler_ALapsedSiblingExpiresRatherThanBeingSuperseded: an
// acceptance closes every other Offer on the work still open, and an
// Offer to someone else that ran out before the acceptance was not open.
// Before 00174 the supersession's guard read only the stored state, and
// lockOwnOffer recorded a lapse on the accepted Offer alone, so the
// sibling was stamped 'superseded' by the accepter.
func TestAcceptHandler_ALapsedSiblingExpiresRatherThanBeingSuperseded(t *testing.T) {
	f := newFixture(t)
	secondID := testdb.SeedStaffAtPractice(t, f.db, f.practiceID, "uid-doula-2", []string{doulaRole}, contractorType)
	lapsed := f.makeOffer(t, offerBody(secondID, 45000))
	accepted := f.makeOffer(t, offerBody(f.doulaID, 45000))
	// After the second Offer is sent, whose create records the
	// Engagement's lapses: the sibling runs out with nothing having
	// written that down.
	expireOffer(t, f.db, lapsed)

	expectStatus(t, do(t, http.MethodPost, f.offerURL(accepted, "accept"), f.doulaSession, nil), http.StatusOK)

	wantLapsed(t, readDecision(t, f.db, lapsed))
	if d := readDecision(t, f.db, accepted); d.state != "accepted" || d.kind == nil || *d.kind != string(offer.DecidedByStaff) ||
		d.decidedBy == nil || *d.decidedBy != f.doulaID {
		t.Fatalf("accepted offer = %+v, want accepted by her", d)
	}
}

// TestCloseOnCompletion_ALapsedOfferExpiresRatherThanBeingWithdrawn:
// completion withdraws what is still open, and only that. Before 00174
// its guard read only the stored state, so an Offer that had run out
// before the work completed was recorded as withdrawn by the completion.
func TestCloseOnCompletion_ALapsedOfferExpiresRatherThanBeingWithdrawn(t *testing.T) {
	f := newFixture(t)
	secondID := testdb.SeedStaffAtPractice(t, f.db, f.practiceID, "uid-doula-2", []string{doulaRole}, contractorType)
	lapsed := f.makeOffer(t, offerBody(secondID, 45000))
	expireOffer(t, f.db, lapsed)
	open := f.makeOffer(t, offerBody(f.doulaID, 45000))

	completeEngagement(t, f)

	wantLapsed(t, readDecision(t, f.db, lapsed))
	d := readDecision(t, f.db, open)
	if d.state != stateWithdrawn || d.kind == nil || *d.kind != string(offer.DecidedByCompletion) || d.decidedBy != nil {
		t.Fatalf("open offer = state %q, kind %v, decided_by %v; want withdrawn by the completion, by nobody", d.state, d.kind, d.decidedBy)
	}
	if d.decidedAt == nil || !d.decidedAt.Before(d.expiresAt) {
		t.Fatalf("withdrawn offer decided_at = %v, want before its expires_at %v", d.decidedAt, d.expiresAt)
	}
}

// TestDecisions_RecordWhoDecided: each decision names its decider, so
// the NULL decided_by of a decline through the emailed link says which of
// the deciders without one it was.
func TestDecisions_RecordWhoDecided(t *testing.T) {
	f := newFixture(t)
	declined := f.makeOffer(t, offerBody(f.doulaID, 45000))
	expectStatus(t, do(t, http.MethodPost, f.offerURL(declined, "decline"), f.doulaSession, nil), http.StatusOK)
	if d := readDecision(t, f.db, declined); d.kind == nil || *d.kind != string(offer.DecidedByStaff) || d.decidedBy == nil || *d.decidedBy != f.doulaID {
		t.Fatalf("declined offer = %+v, want declined by her", d)
	}

	secondID := testdb.SeedStaffAtPractice(t, f.db, f.practiceID, "uid-doula-2", []string{doulaRole}, contractorType)
	withdrawn := f.makeOffer(t, offerBody(secondID, 45000))
	expectStatus(t, do(t, http.MethodPost, f.offerURL(withdrawn, "withdraw"), f.ownerSession, nil), http.StatusOK)
	if d := readDecision(t, f.db, withdrawn); d.kind == nil || *d.kind != string(offer.DecidedByStaff) || d.decidedBy == nil || *d.decidedBy != f.ownerID {
		t.Fatalf("withdrawn offer = %+v, want withdrawn by the Owner", d)
	}

	byToken, token, code := seedEmailOffer(t, f)
	expectStatus(t, do(t, http.MethodPost, f.srv+"/api/offers/"+byToken+"/decline", "",
		offer.DeclineByTokenRequest{Token: token, Code: code}), http.StatusOK)
	if d := readDecision(t, f.db, byToken); d.kind == nil || *d.kind != string(offer.DecidedByInvitee) || d.decidedBy != nil {
		t.Fatalf("offer declined by token = %+v, want declined by the invitee, by no Staff member", d)
	}
}
