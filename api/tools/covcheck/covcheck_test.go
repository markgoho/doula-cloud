package main

import (
	"strings"
	"testing"
)

const testFile = "foo.go"

// TestDeliberateFailure_CIReportSmokeTest is a temporary test used only to
// verify the go-test-action job summary still renders on a red run. It is
// reverted immediately after that's confirmed.
func TestDeliberateFailure_CIReportSmokeTest(t *testing.T) {
	t.Fatal("deliberate failure to exercise the CI report's red path")
}

func TestParseProfile(t *testing.T) {
	input := "mode: set\n" +
		"doula-cloud/api/main.go:10.2,12.3 2 1\n" +
		"doula-cloud/api/main.go:14.2,16.3 1 0\n"

	blocks, err := parseProfile(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parseProfile: %v", err)
	}
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[0].File != "doula-cloud/api/main.go" || blocks[0].StartLine != 10 || blocks[0].EndLine != 12 || blocks[0].Count != 1 {
		t.Fatalf("unexpected block 0: %+v", blocks[0])
	}
	if blocks[1].StartLine != 14 || blocks[1].EndLine != 16 || blocks[1].Count != 0 {
		t.Fatalf("unexpected block 1: %+v", blocks[1])
	}
}

func TestParseProfile_RejectsMalformedLine(t *testing.T) {
	_, err := parseProfile(strings.NewReader("mode: set\nnot a valid profile line\n"))
	if err == nil {
		t.Fatal("expected error for malformed profile line, got nil")
	}
}

func lines(ss ...string) []string { return ss }

func TestFindViolations_FlagsUncoveredBlock(t *testing.T) {
	blocks := []Block{
		{File: testFile, StartLine: 2, EndLine: 2, Count: 0},
	}
	src := map[string][]string{
		testFile: lines(`package foo`, `func bar() {}`, ``),
	}

	violations, err := findViolations(blocks, nil, fakeReader(src))
	if err != nil {
		t.Fatalf("findViolations: %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d: %+v", len(violations), violations)
	}
}

func TestFindViolations_ExcusesBlockWithPrecedingIgnoreComment(t *testing.T) {
	blocks := []Block{
		{File: testFile, StartLine: 3, EndLine: 3, Count: 0},
	}
	src := map[string][]string{
		testFile: lines(
			`package foo`,
			`// coverage:ignore reason: unreachable listener failure`,
			`func bar() {}`,
		),
	}

	violations, err := findViolations(blocks, nil, fakeReader(src))
	if err != nil {
		t.Fatalf("findViolations: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations, got %d: %+v", len(violations), violations)
	}
}

func TestFindViolations_ExcusesBlockWithInlineIgnoreComment(t *testing.T) {
	blocks := []Block{
		{File: testFile, StartLine: 1, EndLine: 1, Count: 0},
	}
	src := map[string][]string{
		testFile: lines(`log.Fatal(err) // coverage:ignore reason: unreachable`),
	}

	violations, err := findViolations(blocks, nil, fakeReader(src))
	if err != nil {
		t.Fatalf("findViolations: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations, got %d: %+v", len(violations), violations)
	}
}

func TestFindViolations_IgnoresCoveredBlocks(t *testing.T) {
	blocks := []Block{
		{File: testFile, StartLine: 1, EndLine: 1, Count: 3},
	}
	src := map[string][]string{
		testFile: lines(`func bar() {}`),
	}

	violations, err := findViolations(blocks, nil, fakeReader(src))
	if err != nil {
		t.Fatalf("findViolations: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations, got %d", len(violations))
	}
}

func TestFindViolations_SkipsMatchingPrefix(t *testing.T) {
	blocks := []Block{
		{File: "tools/covcheck/main.go", StartLine: 1, EndLine: 1, Count: 0},
	}
	src := map[string][]string{
		"tools/covcheck/main.go": lines(`func main() {}`),
	}

	violations, err := findViolations(blocks, []string{"tools/"}, fakeReader(src))
	if err != nil {
		t.Fatalf("findViolations: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations, got %d: %+v", len(violations), violations)
	}
}

func fakeReader(src map[string][]string) func(string) ([]string, error) {
	return func(file string) ([]string, error) {
		return src[file], nil
	}
}
