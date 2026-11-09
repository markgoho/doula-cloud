package portal_test

import "testing"

// TestDetailHandler_NeverShowsTheBirthOutcome is ADR-0015's "staff-only,
// never Client-facing" made checkable (#293). Nadia Haddad does not need
// the software to tell her what happened; she needs it to stop asking
// her about a pregnancy -- so the birth outcome and the date it ended
// are recorded on her Engagement and reach no portal response at all,
// not even as a null field.
func TestDetailHandler_NeverShowsTheBirthOutcome(t *testing.T) {
	e := newEnvWithOutcome(t, outcomeLoss, "2027-03-04")

	fields := e.detailFields()
	for _, key := range []string{"birthOutcome", "pregnancyEndedOn"} {
		if _, present := fields[key]; present {
			t.Fatalf("portal detail carries %q; ADR-0015 keeps the birth outcome staff-only", key)
		}
	}
}

// TestDetailHandler_ByBirthOutcome holds two things the Practice's
// recorded outcome decides.
//
// #2003 and #2211: a due date forecasts a birth still to come, so once a
// Practice records any birth outcome -- a live birth, a loss or unknown --
// "Your care" says nothing about it. The BFF leaves the key out, so no
// screen can show it, and it is absent alike after every outcome, so its
// absence tells the Client nothing about the staff-only outcome
// (ADR-0015). No care phase crosses on any Engagement (#2007).
//
// ADR-0015's standing rule as the portal sees it (#294): the Engagement
// read resolves "does this Engagement have a living or expected baby?"
// and hands the portal that answer alone. A loss and an 'unknown' both
// stop the Birth Plan being offered -- no nav item, no hub link, no empty
// state -- while a live birth and an Engagement with nothing recorded yet
// are offered one exactly as before.
func TestDetailHandler_ByBirthOutcome(t *testing.T) {
	tests := []struct {
		name          string
		outcome       string
		endedOn       string
		wantDueDate   bool
		wantBirthPlan bool
	}{
		{"nothing recorded yet -- the pregnancy is still expected", "", "", true, true},
		{"a live birth", outcomeLiveBirth, "2027-06-10", false, true},
		{"a loss", outcomeLoss, "2027-03-04", false, false},
		{"an unknown outcome", "unknown", "", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnvWithOutcome(t, tt.outcome, tt.endedOn)

			fields := e.detailFields()
			if _, present := fields["dueDate"]; present != tt.wantDueDate {
				t.Fatalf("dueDate present = %v, want %v with birth outcome %q", present, tt.wantDueDate, tt.outcome)
			}
			// #2007: no care phase crosses on any Engagement, whatever its outcome.
			if _, present := fields["status"]; present {
				t.Fatalf("status present with birth outcome %q, want it absent on every Engagement", tt.outcome)
			}
			if got := e.detail().OffersBirthPlan; got != tt.wantBirthPlan {
				t.Fatalf("offersBirthPlan = %v, want %v with birth outcome %q", got, tt.wantBirthPlan, tt.outcome)
			}
		})
	}
}
