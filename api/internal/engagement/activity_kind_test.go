package engagement

import (
	"os"
	"regexp"
	"testing"

	"doula-cloud/api/internal/activity"
)

var engagementActionConstant = regexp.MustCompile(`(?m)^\s*(Action[A-Za-z0-9]+)\s+EngagementAction\s*=\s*"([a-z_]+)"`)

// Every action the Engagement subject records belongs to exactly one
// kind of the Activity filter (#2755); a new action fails here until it
// is placed.
func TestActivityKinds_EveryEngagementActionIsInExactlyOne(t *testing.T) {
	src, err := os.ReadFile("../activity/actions.go")
	if err != nil {
		t.Fatal(err)
	}
	found := engagementActionConstant.FindAllStringSubmatch(string(src), -1)
	if len(found) < 40 {
		t.Fatalf("scan found %d EngagementAction constants, want the whole vocabulary", len(found))
	}
	homes := map[string][]string{}
	for kind, list := range activityKinds {
		for _, a := range list {
			homes[string(a)] = append(homes[string(a)], kind)
		}
	}
	for _, m := range found {
		if got := homes[m[2]]; len(got) != 1 {
			t.Errorf("%s (%q) is in kinds %v, want exactly one", m[1], m[2], got)
		}
		delete(homes, m[2])
	}
	for action, kinds := range homes {
		t.Errorf("kinds %v name %q, which is not an EngagementAction in actions.go", kinds, action)
	}
}

func TestActivityKindActions_RefusesAnUnknownKind(t *testing.T) {
	if _, ok := activityKindActions("everything"); ok {
		t.Error("an unknown kind was accepted")
	}
	if all, ok := activityKindActions(""); !ok || all != nil {
		t.Errorf("no kind = %v, %v, want nil and ok", all, ok)
	}
	_ = activity.ActionVisitLogged
}
