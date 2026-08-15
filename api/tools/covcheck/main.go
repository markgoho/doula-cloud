// Command covcheck enforces 100% line coverage on a Go cover profile,
// allowing justified exceptions marked with a "coverage:ignore" comment
// (e.g. "// coverage:ignore reason: log.Fatal on listener startup failure")
// either directly above or within the uncovered line(s).
//
// Usage:
//
//	go test ./... -coverprofile=coverage.out
//	go run ./tools/covcheck -profile=coverage.out -module=doula-cloud/api -skip=doula-cloud/api/tools/
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	profilePath := flag.String("profile", "coverage.out", "path to the go test -coverprofile output")
	modulePrefix := flag.String("module", "doula-cloud/api", "Go module path prefix to strip when resolving files on disk")
	skip := flag.String("skip", "", "comma-separated package path prefixes to exclude from the coverage requirement (still tested, just not gated)")
	flag.Parse()

	var skipPrefixes []string
	if *skip != "" {
		skipPrefixes = strings.Split(*skip, ",")
	}

	f, err := os.Open(*profilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "covcheck: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	blocks, err := parseProfile(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "covcheck: %v\n", err)
		os.Exit(1)
	}

	violations, err := findViolations(blocks, skipPrefixes, func(file string) ([]string, error) {
		return readDiskFile(strings.TrimPrefix(file, *modulePrefix+"/"))
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "covcheck: %v\n", err)
		os.Exit(1)
	}

	if len(violations) == 0 {
		fmt.Println("covcheck: 100% line coverage (excluding justified coverage:ignore exceptions)")
		return
	}

	fmt.Fprintln(os.Stderr, "covcheck: uncovered lines with no coverage:ignore justification:")
	for _, v := range violations {
		fmt.Fprintf(os.Stderr, "  %s:%d-%d\n", v.File, v.StartLine, v.EndLine)
	}
	os.Exit(1)
}

func readDiskFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return strings.Split(string(data), "\n"), nil
}
