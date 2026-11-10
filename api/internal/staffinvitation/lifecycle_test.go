package staffinvitation_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/staffinvitation"
)

// TestStateOf_ExpiryIsDerivedAtExpiresAt pins the one definition of an
// expired Invitation: pending, and expires_at at or before now. A row
// that is accepted or withdrawn keeps that state whatever the clock says.
func TestStateOf_ExpiryIsDerivedAtExpiresAt(t *testing.T) {
	expiresAt := time.Date(2027, 1, 8, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		status staffinvitation.Status
		now    time.Time
		want   staffinvitation.State
	}{
		{staffinvitation.StatusPending, expiresAt.Add(-time.Microsecond), staffinvitation.StatePending},
		{staffinvitation.StatusPending, expiresAt, staffinvitation.StateExpired},
		{staffinvitation.StatusPending, expiresAt.Add(time.Hour), staffinvitation.StateExpired},
		{staffinvitation.StatusAccepted, expiresAt.Add(time.Hour), staffinvitation.StateAccepted},
		{staffinvitation.StatusRevoked, expiresAt.Add(time.Hour), staffinvitation.StateRevoked},
		{staffinvitation.StatusRevoked, expiresAt.Add(-time.Hour), staffinvitation.StateRevoked},
	}
	for _, c := range cases {
		if got := staffinvitation.StateOf(c.status, expiresAt, c.now); got != c.want {
			t.Errorf("StateOf(%q, expires %v, now %v) = %q, want %q", c.status, expiresAt, c.now, got, c.want)
		}
	}
}

// TestTransitions_TableIsComplete proves Transitions carries exactly the
// declared moves, each starting from the states its doc comment names and
// each but accept recording its own activity action.
func TestTransitions_TableIsComplete(t *testing.T) {
	type want struct {
		requires []staffinvitation.State
		action   activity.PracticeAction
	}
	wants := map[string]want{
		"invite this address": {nil, activity.ActionStaffInvited},
		"send it again":       {[]staffinvitation.State{staffinvitation.StatePending, staffinvitation.StateExpired}, activity.ActionStaffInvitationResent},
		"accept it":           {[]staffinvitation.State{staffinvitation.StatePending}, ""},
		"revoke it":           {[]staffinvitation.State{staffinvitation.StatePending, staffinvitation.StateExpired}, activity.ActionStaffInvitationRevoked},
	}
	if len(staffinvitation.Transitions) != len(wants) {
		t.Fatalf("len(Transitions) = %d, want %d", len(staffinvitation.Transitions), len(wants))
	}
	for _, tr := range staffinvitation.Transitions {
		w, known := wants[tr.Act]
		if !known {
			t.Errorf("Transitions has unexpected entry %+v", tr)
			continue
		}
		if strings.Join(statesOf(tr.Requires), ",") != strings.Join(statesOf(w.requires), ",") {
			t.Errorf("Transition %q requires %v, want %v", tr.Act, tr.Requires, w.requires)
		}
		if tr.Action != w.action {
			t.Errorf("Transition %q records %q, want %q", tr.Act, tr.Action, w.action)
		}
	}
}

func statesOf(states []staffinvitation.State) []string {
	out := make([]string, len(states))
	for i, s := range states {
		out[i] = string(s)
	}
	return out
}

// TestTransition_CheckSpeaksToThePerson proves Check's two outcomes, and
// that a refusal is the sentence a person reads: what the Invitation is,
// and what that stops.
func TestTransition_CheckSpeaksToThePerson(t *testing.T) {
	if ok, refusal := staffinvitation.TransitionRevoke.Check(staffinvitation.StateExpired); !ok || refusal != "" {
		t.Fatalf("Revoke.Check(expired) = (%v, %q), want (true, \"\") -- a lapsed Invitation can still be withdrawn", ok, refusal)
	}
	cases := []struct {
		transition staffinvitation.Transition
		current    staffinvitation.State
		want       string
	}{
		{staffinvitation.TransitionRevoke, staffinvitation.StateAccepted, "This invitation has been accepted, so you cannot revoke it."},
		{staffinvitation.TransitionRevoke, staffinvitation.StateRevoked, "This invitation has been revoked, so you cannot revoke it."},
		{staffinvitation.TransitionAccept, staffinvitation.StateExpired, "This invitation has expired, so you cannot accept it."},
		{staffinvitation.TransitionInvite, staffinvitation.StatePending, "This invitation is still waiting for an answer, so you cannot invite this address."},
		{staffinvitation.TransitionAccept, staffinvitation.State("archived"), "This invitation is archived, so you cannot accept it."},
	}
	for _, c := range cases {
		ok, refusal := c.transition.Check(c.current)
		if ok || refusal != c.want {
			t.Errorf("%q.Check(%q) = (%v, %q), want (false, %q)", c.transition.Act, c.current, ok, refusal, c.want)
		}
	}
}

// TestEveryInvitationWriterNamesItsTransition is the source-scan
// guardrail contracts/lifecycle_test.go holds over Contracts: any non-test
// file under internal/ that writes practice_invitations must name a
// staffinvitation Transition, and no staffauth or staffinvite file that
// reads the table may spell its own expiry -- a stored 'expired' or a
// hand-written expires_at
// comparison is the second definition 00170 removed. Self-updating: a new
// writer or reader anywhere in internal/ is scanned the moment it exists.
func TestEveryInvitationWriterNamesItsTransition(t *testing.T) {
	writes := []string{"UPDATE practice_invitations", "INSERT INTO practice_invitations"}
	ownExpiry := []string{"status = 'expired'", "expires_at <=", "expires_at >"}
	// The readers that judge expiry, each of which must ask StateOf.
	judges := map[string]bool{
		filepath.Join("..", "staffauth", "accept.go"):       false,
		filepath.Join("..", "staffauth", "invitelookup.go"): false,
		filepath.Join("..", "staffauth", "staff.go"):        false,
		filepath.Join("..", "staffinvite", "outbox.go"):     false,
	}

	writers := 0
	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// coverage:ignore reason: internal/ is always readable while its tests run
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path) //nolint:gosec // path comes from WalkDir over this module's own internal/ directory, never from input
		if err != nil {
			// coverage:ignore reason: a file WalkDir listed a moment ago
			return fmt.Errorf("read %s: %w", path, err)
		}
		text := string(src)
		if _, ok := judges[path]; ok {
			judges[path] = strings.Contains(text, "staffinvitation.StateOf(")
		}
		if !strings.Contains(text, "practice_invitations") {
			return nil
		}
		for _, w := range writes {
			if strings.Contains(text, w) {
				writers++
				if !strings.Contains(text, "staffinvitation.Transition") {
					t.Errorf("%s writes %q but names no staffinvitation Transition -- a write to an Invitation must go through the declared lifecycle", path, w)
				}
				break
			}
		}
		// The expiry scan stays inside the two packages that read the
		// Invitation itself: offer's queries join practice_invitations
		// for an address and compare an Offer's own expires_at, which is
		// not this one.
		dir := filepath.Base(filepath.Dir(path))
		if dir != "staffauth" && dir != "staffinvite" {
			return nil
		}
		for _, e := range ownExpiry {
			if strings.Contains(text, e) {
				t.Errorf("%s reads practice_invitations and spells its own expiry (%q) -- ask staffinvitation.StateOf instead", path, e)
			}
		}
		return nil
	})
	if err != nil {
		// coverage:ignore reason: internal/ is always readable while its tests run
		t.Fatalf("walk internal/: %v", err)
	}
	if writers == 0 {
		t.Fatal("no file under internal/ writes practice_invitations -- did invite.go/accept.go move?")
	}
	for path, asks := range judges {
		if !asks {
			t.Errorf("%s does not call staffinvitation.StateOf -- it judges an Invitation's expiry and must use the one derivation", path)
		}
	}
}
