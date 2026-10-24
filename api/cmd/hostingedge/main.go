// Command hostingedge is the daily check that clientip's
// hostingEdgeRanges still match the addresses the Firebase Hosting
// rewrite connects to doula-api from (#1867). clientip.From skips the
// right-most X-Forwarded-For entry only when it is inside those ranges,
// so a Hosting edge outside them makes From return Google's address for
// every caller behind it: they share one ratelimit.IPRule bucket, and
// signer_ip and terms_accepted record the edge. That happened from
// 2026-09-27 to 2026-10-24 with 64.233.172.x, and nothing reported it.
//
// Usage:
//
//	hostingedge <goog.json> <cloud.json> < remote-ips
//
// The two files are Google's published lists, fetched fresh by the
// workflow (https://www.gstatic.com/ipranges/goog.json and cloud.json):
// goog.json holds every range Google uses and cloud.json the part a
// Google Cloud customer can rent, so goog.json minus cloud.json is
// Google's own egress. Standard input is one httpRequest.remoteIp per
// line, read from doula-api's request log with /api/internal/** left
// out: Cloud Scheduler and Cloud Tasks call those routes from Google's
// own egress too (35.187.132.x, 34.116.39.x, 107.178.194.x and more,
// read 2026-10-24), but each one carries a Google-signed token
// (ADR-0037) and none of them reads clientip.
//
// It fails when:
//
//   - an address from Google's own egress is outside hostingEdgeRanges.
//     That is either a Hosting edge the list is missing, or another
//     Google service calling the run.app URL direct -- the residual
//     ADR-0056 accepts -- and either one is worth a person's look.
//   - a range in hostingEdgeRanges is no longer inside goog.json, or now
//     overlaps cloud.json. Either way it is no longer Google's own
//     egress, and in the second case a customer could rent an address
//     in it and choose the address From returns.
//
// What it cannot see: a Hosting edge inside cloud.json. Such an address
// looks like any customer's, and it could not be trusted anyway, since a
// customer could rent the address next to it.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/netip"
	"os"
	"slices"
	"strings"

	"doula-cloud/api/internal/clientip"
)

const usage = "hostingedge: usage: hostingedge <goog.json> <cloud.json> < remote-ips"

func main() {
	// coverage:ignore reason: exits the process, not exercised by unit tests
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		// coverage:ignore reason: exits the process, not exercised by unit tests
		log.Fatal(err)
	}
}

// rangeList is the shape of goog.json and cloud.json: each prefix holds
// one of the two fields.
type rangeList struct {
	CreationTime string `json:"creationTime"`
	Prefixes     []struct {
		IPv4 string `json:"ipv4Prefix"`
		IPv6 string `json:"ipv6Prefix"`
	} `json:"prefixes"`
}

func run(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) != 2 {
		return errors.New(usage)
	}
	googRanges, created, err := readRanges(args[0])
	if err != nil {
		return err
	}
	cloudRanges, _, err := readRanges(args[1])
	if err != nil {
		return err
	}
	edges := clientip.HostingEdgeRanges()

	var report strings.Builder
	for _, edge := range edges {
		if !slices.ContainsFunc(googRanges, func(p netip.Prefix) bool { return covers(p, edge) }) {
			fmt.Fprintf(&report, "%s: in hostingEdgeRanges but no longer inside goog.json\n", edge)
		}
		if slices.ContainsFunc(cloudRanges, edge.Overlaps) {
			fmt.Fprintf(&report, "%s: in hostingEdgeRanges but now overlaps cloud.json, so a customer can rent an address in it\n", edge)
		}
	}

	counts := map[netip.Addr]int{}
	var order []netip.Addr
	total := 0
	scanner := bufio.NewScanner(stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		addr, err := netip.ParseAddr(line)
		if err != nil {
			return fmt.Errorf("hostingedge: remote IP %q: %w", line, err)
		}
		addr = addr.Unmap()
		total++
		if counts[addr] == 0 {
			order = append(order, addr)
		}
		counts[addr]++
	}
	// coverage:ignore reason: a strings.Reader or a pipe never fails mid-read in a unit test
	if err := scanner.Err(); err != nil {
		// coverage:ignore reason: a strings.Reader or a pipe never fails mid-read in a unit test
		return fmt.Errorf("hostingedge: reading remote IPs: %w", err)
	}

	for _, addr := range order {
		if inAny(googRanges, addr) && !inAny(cloudRanges, addr) && !inAny(edges, addr) {
			fmt.Fprintf(&report, "%s: %d requests from Google's own egress, outside hostingEdgeRanges\n", addr, counts[addr])
		}
	}

	failed := report.Len() > 0
	fmt.Fprintf(&report, "checked %d requests from %d addresses against %d hostingEdgeRanges (goog.json created %s)\n",
		total, len(order), len(edges), created)
	if _, err := io.WriteString(stdout, report.String()); err != nil {
		return fmt.Errorf("hostingedge: writing the report: %w", err)
	}
	if failed {
		return errors.New("hostingedge: hostingEdgeRanges no longer matches Google's published ranges or the addresses Hosting connects from; see api/internal/clientip")
	}
	return nil
}

// readRanges parses one of Google's published range lists.
func readRanges(path string) ([]netip.Prefix, string, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // the path is this binary's own argument, named by the workflow that runs it
	if err != nil {
		return nil, "", fmt.Errorf("hostingedge: %w", err)
	}
	var list rangeList
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, "", fmt.Errorf("hostingedge: %s: %w", path, err)
	}
	if len(list.Prefixes) == 0 {
		return nil, "", fmt.Errorf("hostingedge: %s holds no prefixes", path)
	}
	prefixes := make([]netip.Prefix, 0, len(list.Prefixes))
	for _, entry := range list.Prefixes {
		text := entry.IPv4 + entry.IPv6
		prefix, err := netip.ParsePrefix(text)
		if err != nil {
			return nil, "", fmt.Errorf("hostingedge: %s: %w", path, err)
		}
		prefixes = append(prefixes, prefix)
	}
	return prefixes, list.CreationTime, nil
}

// covers reports whether outer holds every address in inner.
func covers(outer, inner netip.Prefix) bool {
	return outer.Bits() <= inner.Bits() && outer.Contains(inner.Addr())
}

func inAny(prefixes []netip.Prefix, addr netip.Addr) bool {
	return slices.ContainsFunc(prefixes, func(p netip.Prefix) bool { return p.Contains(addr) })
}
