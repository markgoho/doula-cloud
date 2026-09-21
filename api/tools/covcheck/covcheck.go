package main

import (
	"bufio"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
)

const ignoreMarker = "coverage:ignore"

// Block is one segment of a Go cover profile: a range of lines in a file,
// the number of statements it holds, and the number of times it executed
// during the test run.
type Block struct {
	File      string
	StartLine int
	EndLine   int
	// NumStmt is what tells two Go 1.27 ranges apart from one block cut in
	// two: 1.27 splits a block at each comment line and gives every piece
	// the whole block's count (see coalesceSplitBlocks).
	NumStmt int
	Count   int
}

// Violation is a Block with zero coverage that has no coverage:ignore
// justification.
type Violation struct {
	File      string
	StartLine int
	EndLine   int
}

// parseProfile reads a Go cover profile (as produced by
// `go test -coverprofile`) and returns its blocks. The leading "mode: ..."
// line is skipped.
func parseProfile(r io.Reader) ([]Block, error) {
	var blocks []Block
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}

		block, err := parseProfileLine(line)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("covcheck: read profile: %w", err)
	}
	return blocks, nil
}

// parseProfileLine parses a single line of the form
// "file.go:startLine.startCol,endLine.endCol numStmt count".
func parseProfileLine(line string) (Block, error) {
	fileAndRest := strings.SplitN(line, ":", 2)
	if len(fileAndRest) != 2 {
		return Block{}, fmt.Errorf("covcheck: malformed profile line: %q", line)
	}
	file := fileAndRest[0]

	fields := strings.Fields(fileAndRest[1])
	if len(fields) != 3 {
		return Block{}, fmt.Errorf("covcheck: malformed profile line: %q", line)
	}

	positions := strings.SplitN(fields[0], ",", 2)
	if len(positions) != 2 {
		return Block{}, fmt.Errorf("covcheck: malformed profile line: %q", line)
	}

	startLine, err := lineNumber(positions[0])
	if err != nil {
		return Block{}, fmt.Errorf("covcheck: malformed profile line: %q: %w", line, err)
	}
	endLine, err := lineNumber(positions[1])
	if err != nil {
		return Block{}, fmt.Errorf("covcheck: malformed profile line: %q: %w", line, err)
	}
	numStmt, err := strconv.Atoi(fields[1])
	if err != nil {
		return Block{}, fmt.Errorf("covcheck: malformed profile line: %q: %w", line, err)
	}
	count, err := strconv.Atoi(fields[2])
	if err != nil {
		return Block{}, fmt.Errorf("covcheck: malformed profile line: %q: %w", line, err)
	}

	return Block{File: file, StartLine: startLine, EndLine: endLine, NumStmt: numStmt, Count: count}, nil
}

// lineNumber extracts the line number from a "line.col" position.
func lineNumber(pos string) (int, error) {
	lineStr, _, ok := strings.Cut(pos, ".")
	if !ok {
		return 0, fmt.Errorf("malformed position: %q", pos)
	}
	n, err := strconv.Atoi(lineStr)
	if err != nil {
		return 0, fmt.Errorf("malformed position: %q: %w", pos, err)
	}
	return n, nil
}

// findViolations returns the uncovered blocks that have no coverage:ignore
// justification, either on the line immediately before the block or on any
// line within the block itself. readLines returns the 0-indexed lines of
// the given source file.
//
// Both halves of that rule were written against Go 1.26's block shapes,
// and the repo's thousands of markers are placed to match them. Go 1.27
// reports the same code in different shapes (#1409), so before judging,
// covcheck maps a 1.27 profile back onto the blocks 1.26 would have
// reported -- coalesceSplitBlocks and hasIgnoreCommentAboveOpeningBrace --
// rather than asking every marker in the repo to move. A 1.26 profile
// passes through both unchanged.
func findViolations(blocks []Block, skipPrefixes []string, readLines func(file string) ([]string, error)) ([]Violation, error) {
	uncoveredByFile := map[string][]Block{}
	var files []string
	for _, b := range blocks {
		if b.Count != 0 || hasAnyPrefix(b.File, skipPrefixes) {
			continue
		}
		if _, seen := uncoveredByFile[b.File]; !seen {
			files = append(files, b.File)
		}
		uncoveredByFile[b.File] = append(uncoveredByFile[b.File], b)
	}

	var violations []Violation
	for _, file := range files {
		fileLines, err := readLines(file)
		if err != nil {
			return nil, fmt.Errorf("covcheck: read %s: %w", file, err)
		}
		for _, b := range coalesceSplitBlocks(uncoveredByFile[file], fileLines) {
			if !hasIgnoreComment(fileLines, b.StartLine, b.EndLine) {
				violations = append(violations, Violation{File: b.File, StartLine: b.StartLine, EndLine: b.EndLine})
			}
		}
	}

	return violations, nil
}

// coalesceSplitBlocks rejoins the ranges Go 1.27 cut one block into.
// 1.26 ran a straight-line block on through comment lines; 1.27 ends a
// range at each one and starts the next after it, giving every piece the
// whole block's statement count (#1409: main.go's 29-statement block came
// back as thirteen ranges, 185.2,188.1 through 268.2,319.1, each "29 0").
// A marker excused the 1.26 block as a whole, so the pieces are judged as
// one again. Two uncovered ranges join only when they carry the same
// statement count and nothing but blank lines and comments lies between
// them -- a real 1.26 block boundary always sits on a line of code (a
// brace, a statement), so this never merges two blocks 1.26 kept apart.
// blocks must all be uncovered and from one file.
func coalesceSplitBlocks(blocks []Block, fileLines []string) []Block {
	sorted := slices.Clone(blocks)
	slices.SortFunc(sorted, func(a, b Block) int { return a.StartLine - b.StartLine })

	var merged []Block
	for _, b := range sorted {
		if n := len(merged); n > 0 {
			prev := &merged[n-1]
			if prev.NumStmt == b.NumStmt && onlyCommentsBetween(fileLines, prev.EndLine, b.StartLine) {
				prev.EndLine = max(prev.EndLine, b.EndLine)
				continue
			}
		}
		merged = append(merged, b)
	}
	return merged
}

// onlyCommentsBetween reports whether every line strictly between the
// 1-indexed lines after and before is blank or a comment.
func onlyCommentsBetween(fileLines []string, after, before int) bool {
	for line := after + 1; line < before; line++ {
		if line-1 >= len(fileLines) {
			return false
		}
		if !isBlankOrComment(fileLines[line-1]) {
			return false
		}
	}
	return true
}

func isBlankOrComment(line string) bool {
	trimmed := strings.TrimSpace(line)
	return trimmed == "" || strings.HasPrefix(trimmed, "//")
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

func hasIgnoreComment(fileLines []string, startLine, endLine int) bool {
	// Check the line immediately preceding the block (1-indexed startLine-1).
	if precedingIdx := startLine - 2; precedingIdx >= 0 && precedingIdx < len(fileLines) {
		if strings.Contains(fileLines[precedingIdx], ignoreMarker) {
			return true
		}
	}

	// Check every line within the block itself.
	for idx := startLine - 1; idx <= endLine-1 && idx < len(fileLines); idx++ {
		if idx < 0 {
			continue
		}
		if strings.Contains(fileLines[idx], ignoreMarker) {
			return true
		}
	}

	return hasIgnoreCommentAboveOpeningBrace(fileLines, startLine)
}

// hasIgnoreCommentAboveOpeningBrace recovers the block start Go 1.26 used
// to report. 1.26 began an if/for/else body's block on the line holding
// its opening brace, so the repo's idiom -- the marker on the line above
// the `if` -- was the line immediately preceding the block. Go 1.27
// begins the same block at the body's first statement instead (#1409:
// `if !ok {` on line 6 moved from `6.9,8.3` to `7.3,8.1`), which leaves
// the marker two lines up and every such exception unexcused.
//
// So: walk up from the line above the block over blank lines and comments
// (1.26 counted those inside the block, so a marker among them excuses it
// too), and if that walk ends on a line that opens a body, check it and
// the line above it, exactly the two lines 1.26 checked. The walk stops at
// the first line of ordinary code, so it never reaches further than the
// block 1.26 reported did.
func hasIgnoreCommentAboveOpeningBrace(fileLines []string, startLine int) bool {
	idx := startLine - 2
	for ; idx >= 0 && idx < len(fileLines); idx-- {
		trimmed := strings.TrimSpace(fileLines[idx])
		if strings.Contains(trimmed, ignoreMarker) {
			return true
		}
		if !isBlankOrComment(trimmed) {
			break
		}
	}
	if idx < 0 || idx >= len(fileLines) || !strings.HasSuffix(strings.TrimSpace(fileLines[idx]), "{") {
		return false
	}
	return idx > 0 && strings.Contains(fileLines[idx-1], ignoreMarker)
}
