package activity_test

import (
	"os"
	"regexp"
	"slices"
	"testing"

	"doula-cloud/api/internal/activity"
)

/*
#1148's vocabulary guard. MembershipActions() is a hand-written slice --
Go has no way to enumerate a typed constant group -- so a sixth
MembershipAction constant could be declared and left out of it, and every
reader that trusts the function to be the whole set (the practice feed's
own coverage of "every Membership event", activitygate's action test)
would quietly stop covering it.

So this reads the constants back out of actions.go, lexically, the same
way app/src's activityPhrases.usage.spec.ts reads this file's
EngagementAction group: gofmt already puts one constant per line, and
parsing properly would buy nothing over a regex. Equality is asserted in
both directions -- a constant missing from the function fails, and so
does a function entry naming a value no constant declares.
*/

// membershipConstant matches a MembershipAction constant's value, in the
// typed form (`ActionSessionsEnded MembershipAction = "sessions_ended"`)
// and in the untyped continuation form gofmt allows inside the same
// const group (`ActionRolesChanged MembershipAction = "roles_changed"` is
// typed; a bare `ActionFoo = "foo"` beside it would not be, and is caught
// by TestMembershipActions_DeclaresEveryConstantWithItsType below).
var membershipConstant = regexp.MustCompile(`(?m)^\s*(Action[A-Za-z0-9]+)\s+MembershipAction\s*=\s*"([a-z0-9_]+)"`)

// anyActionConstant matches every Action* constant in the file, typed or
// not, so an untyped one declared inside the Membership group can be told
// apart from the typed ones around it.
var anyActionConstant = regexp.MustCompile(`(?m)^\s*(Action[A-Za-z0-9]+)(?:\s+([A-Za-z]+))?\s*=\s*"([a-z0-9_]+)"`)

func TestMembershipActions_HoldsEveryConstant(t *testing.T) {
	declared := declaredMembershipValues(t)
	if len(declared) < 5 {
		t.Fatalf("scanned %d MembershipAction constants, want at least the five that exist -- the scan itself has broken", len(declared))
	}

	returned := []string{}
	for _, a := range activity.MembershipActions() {
		returned = append(returned, string(a))
	}
	slices.Sort(declared)
	slices.Sort(returned)
	if !slices.Equal(declared, returned) {
		t.Fatalf("MembershipActions() = %v, but actions.go declares %v -- a Membership action left out of the function is one no reader of the set covers", returned, declared)
	}
}

func TestMembershipActions_Sorted(t *testing.T) {
	got := activity.MembershipActions()
	if !slices.IsSorted(got) {
		t.Fatalf("MembershipActions() = %v, want sorted -- a caller building a query string needs a deterministic order", got)
	}
}

// TestMembershipActions_DeclaresEveryConstantWithItsType closes the one
// hole the scan above cannot see: an untyped `ActionFoo = "foo"` inside
// the Membership const group converts implicitly at every write site the
// typed form does, and membershipConstant would never match it. Named
// here so the failure says what to do -- give the constant its type.
func TestMembershipActions_DeclaresEveryConstantWithItsType(t *testing.T) {
	source := readActionsSource(t)
	untyped := []string{}
	for _, m := range anyActionConstant.FindAllStringSubmatch(source, -1) {
		if m[2] == "" {
			untyped = append(untyped, m[1]+` = "`+m[3]+`"`)
		}
	}
	if len(untyped) > 0 {
		t.Fatalf("these action constants declare no type, so no vocabulary guard can see them: %v", untyped)
	}
}

func declaredMembershipValues(t *testing.T) []string {
	t.Helper()
	out := []string{}
	for _, m := range membershipConstant.FindAllStringSubmatch(readActionsSource(t), -1) {
		out = append(out, m[2])
	}
	return out
}

func readActionsSource(t *testing.T) string {
	t.Helper()
	source, err := os.ReadFile("actions.go")
	if err != nil {
		// coverage:ignore reason: the file is in this package; a missing one is a broken checkout, not a case under test
		t.Fatalf("read actions.go: %v", err)
	}
	return string(source)
}
