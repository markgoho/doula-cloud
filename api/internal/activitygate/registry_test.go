package activitygate

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

/*
#1148's class guard, in the mold of app/src's own usage specs
(spelling.usage.spec.ts, activityPhrases.usage.spec.ts): a test that
reads the *write* side and fails when this side stops matching it.

The bug it closes is not "membership was missing a Rule". It is that a
whole event family could be written for months, be refused by this gate,
and be dropped from #486's practice-wide feed without one test going red
-- because "no Rule registered" and "deliberately not on the feed" looked
identical from here. They no longer do: every subject kind a write site
records must appear in exactly one of registry (it reaches the feed, on
these terms) or unregistered (it does not, for this stated reason), and a
new kind in neither fails here until somebody decides which it is.

What it reads is both shapes a write site can name a subject kind in:
the exported Subject* constants in activity/actions.go, which is how
almost every writer names one, and any `SubjectKind: "literal"` an
activity.Entry is built with directly, which is how clientfieldtemplate
still does it. Lexical rather than a go/ast parse, for the reason
activityPhrases.usage.spec.ts gives for its own regexes: gofmt already
puts one constant per line, and parsing would buy nothing.
*/

// subjectConstant matches `SubjectMembership = "membership"` and the
// typed form alike, anywhere in a const block or at top level.
var subjectConstant = regexp.MustCompile(`(?m)^\s*(?:const\s+)?Subject[A-Za-z0-9]+(?:\s+[A-Za-z]+)?\s*=\s*"([a-z0-9_]+)"`)

// subjectKindLiteral matches an activity.Entry built with a bare string
// for its SubjectKind field, which the constant scan above cannot see.
var subjectKindLiteral = regexp.MustCompile(`SubjectKind:\s*"([a-z0-9_]+)"`)

// TestRegistry_CoversEverySubjectKindTheWriteSideRecords is the guard
// itself. A kind in neither map is the #1148 failure, so it names that
// ticket in its own message rather than leaving the next reader to work
// out what "not registered" is supposed to mean.
func TestRegistry_CoversEverySubjectKindTheWriteSideRecords(t *testing.T) {
	for kind, where := range writtenSubjectKinds(t) {
		_, registered := registry[kind]
		_, excused := unregistered[kind]
		switch {
		case registered && excused:
			t.Errorf("subject kind %q (%s) is both registered and listed as deliberately unregistered; it must be exactly one", kind, where)
		case !registered && !excused:
			t.Errorf("subject kind %q is written by %s but appears in neither registry nor unregistered.\n"+
				"An unregistered kind is refused by this gate, so every row of it is dropped from the practice-wide feed silently -- that is #1148.\n"+
				"Register a Rule for it, or add it to unregistered with the reason it stays off the feed.", kind, where)
		}
	}
}

// TestUnregistered_HoldsNoKindTheWriteSideStoppedWriting is the other
// direction, the same both-ways equality activityPhrases.usage.spec.ts
// asserts: an excuse for a kind nothing writes any more is a stale
// comment, and a stale comment about visibility is worth failing over.
func TestUnregistered_HoldsNoKindTheWriteSideStoppedWriting(t *testing.T) {
	written := writtenSubjectKinds(t)
	for kind, reason := range unregistered {
		if _, ok := written[kind]; !ok {
			t.Errorf("unregistered holds %q (%q) but no write site names it any more; drop the entry", kind, reason)
		}
		if strings.TrimSpace(reason) == "" {
			t.Errorf("unregistered holds %q with no reason; the reason is the whole point of the map", kind)
		}
	}
}

// TestWrittenSubjectKinds_ScanFindsTheKindsThatExist proves the scan
// itself still finds something -- a regex that silently matched nothing
// would pass both tests above while guarding nothing at all.
func TestWrittenSubjectKinds_ScanFindsTheKindsThatExist(t *testing.T) {
	if got := len(writtenSubjectKinds(t)); got < 4 {
		t.Fatalf("scanned %d subject kinds, want at least the four that exist (engagement, client, practice, membership) -- the scan itself has broken", got)
	}
}

// writtenSubjectKinds maps each subject kind api/internal records to a
// human description of where it was found, for the failure message.
func writtenSubjectKinds(t *testing.T) map[string]string {
	t.Helper()
	found := map[string]string{}

	actions, err := os.ReadFile(filepath.Join("..", "activity", "actions.go"))
	if err != nil {
		t.Fatalf("read activity/actions.go: %v", err)
	}
	for _, m := range subjectConstant.FindAllStringSubmatch(string(actions), -1) {
		found[m[1]] = "a Subject* constant in activity/actions.go"
	}

	root := ".."
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk %s: %w", path, err)
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path) //nolint:gosec // path comes from WalkDir over this package's own parent directory, never from input
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		for _, m := range subjectKindLiteral.FindAllStringSubmatch(string(src), -1) {
			if _, already := found[m[1]]; !already {
				found[m[1]] = "a SubjectKind literal in " + filepath.ToSlash(path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk api/internal: %v", err)
	}
	return found
}
