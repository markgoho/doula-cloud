package lifecycletest_test

import (
	"strings"
	"testing"

	"doula-cloud/api/internal/lifecycletest"
)

func TestSources_ReadsEveryNonTestGoFileByItsSlashPath(t *testing.T) {
	src := lifecycletest.Sources(t, "..", "lifecycletest")
	if len(src) != 1 {
		t.Fatalf("Sources = %d files, want only lifecycletest.go", len(src))
	}
	if !strings.Contains(src["lifecycletest/lifecycletest.go"], "package lifecycletest") {
		t.Errorf("Sources keys = %v, want lifecycletest/lifecycletest.go with its source", src)
	}
}

func TestRead_ReadsANamedFile(t *testing.T) {
	if got := lifecycletest.Read(t, ".", "lifecycletest.go"); !strings.HasPrefix(got, "// Package lifecycletest") {
		t.Error("Read did not return this package's source")
	}
}

type door string

func (d door) Check(current string) (bool, string) {
	if current == string(d) {
		return true, ""
	}
	return false, "This door is " + current + ", so you cannot open it."
}

func TestRequireRefusals_PassesWhenEveryCaseIsRefusedInItsWords(t *testing.T) {
	lifecycletest.RequireRefusals(t, []lifecycletest.Refusal[door, string]{
		{Transition: "closed", Current: "locked", Want: "This door is locked, so you cannot open it."},
	})
}
