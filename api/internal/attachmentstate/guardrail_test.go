package attachmentstate_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// apiDir is api/, whose internal and cmd trees every scan below walks.
const apiDir = "../.."

// scannedTrees are the trees under api/ that hold production Go.
var scannedTrees = []string{"internal", "cmd"}

// writer is the one file allowed to accrue, grant or end an Attachment.
const writer = "internal/attachmentstate/attachmentstate.go"

// endedPredicate is a reader asking ended_at directly instead of the
// state it derives. ended_at is a column of engagement_attachments only,
// so the pattern needs no table name to be about Attachments.
var endedPredicate = regexp.MustCompile(`\bended_at\s+IS\s+(NOT\s+)?NULL\b`)

// TestReaders_AskTheState fails a reader that goes back to ended_at
// instead of engagement_attachments.state -- "a Doula on the Engagement"
// is state = 'granted', "open, either origin" is state <> 'ended', and
// "the holder left" is engagement_holder_left -- so a difference between
// two readers is a named choice of states rather than an accident of
// which half of the predicate each remembered. Comments are not reads, so
// they are skipped. Nothing is exempt, the writer included.
func TestReaders_AskTheState(t *testing.T) {
	scanned := 0
	walkSources(t, func(file, src string) {
		scanned++
		for i, line := range strings.Split(src, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			if m := endedPredicate.FindString(line); m != "" {
				t.Errorf("%s:%d asks %q -- ask engagement_attachments.state (attachmentstate's three values) instead", file, i+1, m)
			}
		}
	})
	if scanned == 0 {
		t.Fatal("the scan read no file -- did api/ move?")
	}
}

// rawWrite is a statement that writes an engagement_attachments row.
var rawWrite = regexp.MustCompile(`(?i)\b(UPDATE|INSERT\s+INTO)\s+engagement_attachments\b`)

// writesElsewhere are the files besides the writer that hold such a
// statement, each with why it is not a lifecycle move.
var writesElsewhere = map[string]string{
	"internal/oncall/engagement_writes.go": "a Narrowing writes on_call_from/on_call_to on an open, granted Attachment; it never touches origin, attached_by or the ending",
	"internal/testdb/seed.go":              "test scaffolding: seeds rows directly, outside any route",
}

// TestWrites_GoThroughTheDeclaredTransitions is self-updating: a new file
// that inserts or updates an Attachment fails here the moment it is
// added, until it goes through attachmentstate's Transitions or says why
// it need not.
func TestWrites_GoThroughTheDeclaredTransitions(t *testing.T) {
	found := map[string]bool{}
	walkSources(t, func(file, src string) {
		if !rawWrite.MatchString(src) {
			return
		}
		found[file] = true
		if file == writer {
			return
		}
		if _, ok := writesElsewhere[file]; !ok {
			t.Errorf("%s writes engagement_attachments directly -- accrue, grant or end through attachmentstate, or name the file in writesElsewhere with the reason", file)
		}
	})
	if !found[writer] {
		t.Errorf("%s no longer writes engagement_attachments -- did the writer move?", writer)
	}
	for file := range writesElsewhere {
		if !found[file] {
			t.Errorf("%s is named in writesElsewhere but writes no Attachment -- drop it from the map", file)
		}
	}
}

// walkSources calls fn with every non-test Go file under scannedTrees, by
// its slash path relative to api/.
func walkSources(t *testing.T, fn func(file, src string)) {
	t.Helper()
	for _, tree := range scannedTrees {
		err := filepath.WalkDir(filepath.Join(apiDir, tree), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				// coverage:ignore reason: api/ is always readable while its tests run
				return fmt.Errorf("walk %s: %w", path, err)
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, err := filepath.Rel(apiDir, path)
			if err != nil {
				// coverage:ignore reason: path came from walking apiDir itself
				return fmt.Errorf("relative path of %s: %w", path, err)
			}
			fn(filepath.ToSlash(rel), readSource(t, rel))
			return nil
		})
		if err != nil {
			// coverage:ignore reason: api/ is always readable while its tests run
			t.Fatalf("walk %s: %v", tree, err)
		}
	}
}

// readSource reads file, a slash path relative to api/.
func readSource(t *testing.T, file string) string {
	t.Helper()
	// #nosec G304 -- file is a path under this repository's own api/, not anything a caller supplies
	src, err := os.ReadFile(filepath.Join(apiDir, filepath.FromSlash(file)))
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	return string(src)
}
