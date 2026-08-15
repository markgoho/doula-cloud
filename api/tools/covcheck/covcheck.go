package main

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const ignoreMarker = "coverage:ignore"

// Block is one segment of a Go cover profile: a range of lines in a file,
// with the number of times it executed during the test run.
type Block struct {
	File      string
	StartLine int
	EndLine   int
	Count     int
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
	count, err := strconv.Atoi(fields[2])
	if err != nil {
		return Block{}, fmt.Errorf("covcheck: malformed profile line: %q: %w", line, err)
	}

	return Block{File: file, StartLine: startLine, EndLine: endLine, Count: count}, nil
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
func findViolations(blocks []Block, skipPrefixes []string, readLines func(file string) ([]string, error)) ([]Violation, error) {
	linesByFile := map[string][]string{}
	var violations []Violation

	for _, b := range blocks {
		if b.Count != 0 {
			continue
		}
		if hasAnyPrefix(b.File, skipPrefixes) {
			continue
		}

		fileLines, ok := linesByFile[b.File]
		if !ok {
			var err error
			fileLines, err = readLines(b.File)
			if err != nil {
				return nil, fmt.Errorf("covcheck: read %s: %w", b.File, err)
			}
			linesByFile[b.File] = fileLines
		}

		if !hasIgnoreComment(fileLines, b.StartLine, b.EndLine) {
			violations = append(violations, Violation{File: b.File, StartLine: b.StartLine, EndLine: b.EndLine})
		}
	}

	return violations, nil
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

	return false
}
