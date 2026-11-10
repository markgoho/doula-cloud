// Package lifecycletest holds what the lifecycle guardrail tests share:
// reading the module's own Go source for a source scan, and holding a
// Transition's refusals to the sentences a person reads.
package lifecycletest

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Sources returns every non-test Go file under each of trees, a path
// relative to root, keyed by its slash path relative to root.
func Sources(t *testing.T, root string, trees ...string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, tree := range trees {
		err := filepath.WalkDir(filepath.Join(root, tree), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				// coverage:ignore reason: the module's own source tree is always readable while its tests run
				return fmt.Errorf("walk %s: %w", path, err)
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				// coverage:ignore reason: path came from walking under root itself
				return fmt.Errorf("relative path of %s: %w", path, err)
			}
			out[filepath.ToSlash(rel)] = Read(t, root, rel)
			return nil
		})
		if err != nil {
			// coverage:ignore reason: the module's own source tree is always readable while its tests run
			t.Fatalf("lifecycletest: %v", err)
		}
	}
	return out
}

// Read returns file, a slash path relative to root.
func Read(t *testing.T, root, file string) string {
	t.Helper()
	// #nosec G304 -- file is a path under this module's own source tree, fixed by the calling test or listed by Sources
	src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
	if err != nil {
		// coverage:ignore reason: the failure branch ends the calling test by design; exercising it would need a testing.T substitute this repo has no precedent for
		t.Fatalf("lifecycletest: read %s: %v -- did it move? Update the test that names it.", file, err)
	}
	return string(src)
}

// Checker is a Transition: asked from a state, it answers whether the
// move is allowed and, when it is not, the sentence that refuses it.
type Checker[S any] interface {
	Check(current S) (ok bool, refusal string)
}

// Refusal is a Transition asked from a state it must refuse, and the
// sentence it must refuse with.
type Refusal[T Checker[S], S any] struct {
	Transition T
	Current    S
	Want       string
}

// RequireRefusals fails each case whose Transition allows the move from
// Current, or refuses it in other words than Want.
func RequireRefusals[T Checker[S], S any](t *testing.T, cases []Refusal[T, S]) {
	t.Helper()
	for _, c := range cases {
		if ok, refusal := c.Transition.Check(c.Current); ok || refusal != c.Want {
			// coverage:ignore reason: the failure branch fails the calling test by design; exercising it would need a testing.T substitute this repo has no precedent for
			t.Errorf("%+v.Check(%v) = (%v, %q), want (false, %q)", c.Transition, c.Current, ok, refusal, c.Want)
		}
	}
}
