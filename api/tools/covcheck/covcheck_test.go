package main

import (
	"strings"
	"testing"
)

const testFile = "foo.go"

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
	if blocks[0].File != "doula-cloud/api/main.go" || blocks[0].StartLine != 10 || blocks[0].EndLine != 12 || blocks[0].NumStmt != 2 || blocks[0].Count != 1 {
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

// Go 1.27 starts an if/for body's coverage block at the body's first
// statement, not at the line holding its opening brace the way 1.26 did.
// The marker the repo writes above the `if` is then two lines above the
// block rather than one, and must still excuse it. Block shapes below are
// copied from real 1.26.5 and 1.27.1 profiles of the same source.
func TestFindViolations_ExcusesGo127BodyBlockWithMarkerAboveItsIf(t *testing.T) {
	src := map[string][]string{
		testFile: lines(
			`func F(ok bool) int {`,                   // 1
			`	// coverage:ignore reason: unreachable`, // 2
			`	if !ok {`,  // 3
			`		return 0`, // 4
			`	}`,         // 5
		),
	}
	for name, block := range map[string]Block{
		"go1.26 shape: block starts on the brace line":      {File: testFile, StartLine: 3, EndLine: 5, Count: 0},
		"go1.27 shape: block starts at the first statement": {File: testFile, StartLine: 4, EndLine: 5, Count: 0},
	} {
		t.Run(name, func(t *testing.T) {
			violations, err := findViolations([]Block{block}, nil, fakeReader(src))
			if err != nil {
				t.Fatalf("findViolations: %v", err)
			}
			if len(violations) != 0 {
				t.Fatalf("expected 0 violations, got %+v", violations)
			}
		})
	}
}

// A plain comment opening the body sits between the brace and the first
// statement; 1.26 counted it inside the block, so the walk back to the
// brace has to step over it.
func TestFindViolations_ExcusesGo127BodyBlockAfterALeadingComment(t *testing.T) {
	blocks := []Block{{File: testFile, StartLine: 5, EndLine: 6, Count: 0}}
	src := map[string][]string{
		testFile: lines(
			`	// coverage:ignore reason: unreachable`, // 1
			`	if !ok {`, // 2
			`		// nothing reaches this through the mount`, // 3
			``,         // 4
			`		return`, // 5
			`	}`,       // 6
		),
	}

	violations, err := findViolations(blocks, nil, fakeReader(src))
	if err != nil {
		t.Fatalf("findViolations: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations, got %+v", violations)
	}
}

// Go 1.27 also cuts one straight-line block into several ranges at each
// comment line, where 1.26 ran the block on through it -- every range
// carrying the whole block's statement count (main.go's 29-statement
// block came back as thirteen ranges, 185.2,188.1 through 268.2,319.1).
// One marker excused the 1.26 block; it must excuse all of its pieces.
func TestFindViolations_ExcusesEveryFragmentOfAGo127SplitBlock(t *testing.T) {
	blocks := []Block{
		{File: testFile, StartLine: 3, EndLine: 4, NumStmt: 3, Count: 0},
		{File: testFile, StartLine: 6, EndLine: 7, NumStmt: 3, Count: 0},
		{File: testFile, StartLine: 9, EndLine: 9, NumStmt: 3, Count: 0},
	}
	src := map[string][]string{
		testFile: lines(
			`func main() {`, // 1
			`	// coverage:ignore reason: needs a real DATABASE_URL`, // 2
			`	db := open()`,        // 3
			`	// first comment`,    // 4
			`	// continues`,        // 5
			`	mailer := build(db)`, // 6
			``,                     // 7
			`	// second comment`,   // 8
			`	serve(mailer)`,       // 9
		),
	}

	violations, err := findViolations(blocks, nil, fakeReader(src))
	if err != nil {
		t.Fatalf("findViolations: %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("expected 0 violations, got %+v", violations)
	}
}

// Two uncovered blocks separated only by a comment are not one block
// unless they carry the same statement count: an if-body, a comment, and
// the next statement are two 1.26 blocks, and the marker above the second
// must not excuse the first.
func TestFindViolations_DoesNotMergeBlocksWithDifferentStatementCounts(t *testing.T) {
	blocks := []Block{
		{File: testFile, StartLine: 2, EndLine: 3, NumStmt: 1, Count: 0},
		{File: testFile, StartLine: 5, EndLine: 5, NumStmt: 2, Count: 0},
	}
	src := map[string][]string{
		testFile: lines(
			`	if x {`, // 1
			`		foo()`, // 2
			`	}`,      // 3
			`	// coverage:ignore reason: excuses bar only`, // 4
			`	bar()`, // 5
		),
	}

	violations, err := findViolations(blocks, nil, fakeReader(src))
	if err != nil {
		t.Fatalf("findViolations: %v", err)
	}
	if len(violations) != 1 || violations[0].StartLine != 2 {
		t.Fatalf("expected only the if-body flagged, got %+v", violations)
	}
}

// Nor are two ranges merged across a line of code, whatever their counts.
func TestFindViolations_DoesNotMergeAcrossCode(t *testing.T) {
	blocks := []Block{
		{File: testFile, StartLine: 2, EndLine: 2, NumStmt: 1, Count: 0},
		{File: testFile, StartLine: 5, EndLine: 5, NumStmt: 1, Count: 0},
	}
	src := map[string][]string{
		testFile: lines(
			`	// coverage:ignore reason: excuses the line below only`, // 1
			`	x := 1`,   // 2
			`	y := 2`,   // 3
			`	// note`,  // 4
			`	return y`, // 5
		),
	}

	violations, err := findViolations(blocks, nil, fakeReader(src))
	if err != nil {
		t.Fatalf("findViolations: %v", err)
	}
	if len(violations) != 1 || violations[0].StartLine != 5 {
		t.Fatalf("expected only the second range flagged, got %+v", violations)
	}
}

// The walk back stops at the brace: a marker two lines above a block
// whose preceding line is ordinary code, not the body's opener, excuses
// nothing, the same as under 1.26.
func TestFindViolations_DoesNotReachPastOrdinaryCode(t *testing.T) {
	blocks := []Block{{File: testFile, StartLine: 3, EndLine: 3, Count: 0}}
	src := map[string][]string{
		testFile: lines(
			`	// coverage:ignore reason: excuses the line below only`, // 1
			`	x := 1`,   // 2
			`	return x`, // 3
		),
	}

	violations, err := findViolations(blocks, nil, fakeReader(src))
	if err != nil {
		t.Fatalf("findViolations: %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %+v", violations)
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
