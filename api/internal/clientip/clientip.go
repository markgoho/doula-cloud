// Package clientip resolves the real caller address behind Cloud Run's
// Google Front End and the Firebase Hosting rewrite. Extracted from
// contracts/sign.go (#602) once a second consumer (ratelimit) needed the
// same logic -- this repo's bar for pulling shared code out of its first
// home.
//
// # Why the header is read from the right (#1865)
//
// X-Forwarded-For is a list every hop appends to. A proxy does not check
// or remove what the caller sent: Google's load balancer documentation
// says it appends after "any value already present" and "does not verify
// any IP addresses that precede" its own entries
// (https://docs.cloud.google.com/load-balancing/docs/https, read
// 2026-10-24). So the left-most entry is whatever text the caller chose,
// and only the entries on the right, written by a proxy from the address
// of the connection it accepted, can be trusted.
//
// A request reaches the BFF one of two ways, and a caller picks which:
//
//   - Direct to the run.app URL. Cloud Run's ingress is "all" (gcloud
//     run services describe doula-api, 2026-10-24), because Stripe's
//     webhooks and Cloud Scheduler call it there. Google's front end
//     appends the caller's address: "<caller text>, <caller>".
//   - Through the Firebase Hosting rewrite of /api/**. Hosting appends
//     the browser's address, then Google's front end appends Hosting's:
//     "<caller text>, <caller>, <Hosting edge>".
//
// A fixed count of hops to skip is right for only one of the two, so
// From skips by address instead: walking from the right, it passes over
// each entry inside hostingEdge and returns the first one outside it.
// A direct caller cannot put a Hosting address in the right-most place,
// because Google's front end writes that one from the TCP connection.
//
// # What was observed
//
// 2026-10-24, Cloud Logging's httpRequest.remoteIp for doula-api over
// the previous 30 days: every request that came through Hosting arrived
// from 66.102.0.0/20, 66.249.64.0/19, 74.125.0.0/16 or 192.178.0.0/15.
// All four are in Google's own published ranges
// (https://www.gstatic.com/ipranges/goog.json, created
// 2026-10-24T01:06:28) and none overlaps a range a Google Cloud customer
// can rent (https://www.gstatic.com/ipranges/cloud.json, same date), so
// no caller can open a connection from one with a VM of its own.
//
// Not yet observed: the header itself, as the container receives it,
// for a request with a caller-written entry. #1865 holds the probe that
// settles it; record its result and date here.
package clientip

import (
	"net"
	"net/http"
	"net/netip"
	"slices"
	"strings"
)

// hostingEdge holds the address ranges the Firebase Hosting rewrite
// connects to Cloud Run from -- the only proxy From skips past. See the
// package comment for where each range was observed.
var hostingEdge = []netip.Prefix{
	netip.MustParsePrefix("66.102.0.0/20"),
	netip.MustParsePrefix("66.249.64.0/19"),
	netip.MustParsePrefix("74.125.0.0/16"),
	netip.MustParsePrefix("192.178.0.0/15"),
}

// From returns the caller's address, for use as an ESIGN signer_ip
// (contracts), the terms_accepted record (staffauth), or a rate-limit
// dimension (ratelimit): the right-most X-Forwarded-For entry that is
// not a Hosting edge, or the left-most entry if every one is. With no
// header -- the local and e2e stack, with no proxy in front of the
// process -- it is r.RemoteAddr's host.
func From(r *http.Request) string {
	for i, entry := range slices.Backward(forwardedFor(r)) {
		if i == 0 || !isHostingEdge(entry) {
			return entry
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	// coverage:ignore reason: net/http always sets RemoteAddr in host:port form, not exercised by unit tests
	if err != nil {
		// coverage:ignore reason: net/http always sets RemoteAddr in host:port form, not exercised by unit tests
		return r.RemoteAddr
	}
	return host
}

// forwardedFor flattens every X-Forwarded-For line into one list, left
// to right, without the blanks a caller-written prefix may contain.
func forwardedFor(r *http.Request) []string {
	var entries []string
	for _, line := range r.Header.Values("X-Forwarded-For") {
		for entry := range strings.SplitSeq(line, ",") {
			if entry = strings.TrimSpace(entry); entry != "" {
				entries = append(entries, entry)
			}
		}
	}
	return entries
}

// isHostingEdge reports whether entry is an address inside hostingEdge.
// An entry that is not an address at all is caller-written text, never
// a proxy, so it is not skipped.
func isHostingEdge(entry string) bool {
	addr, err := netip.ParseAddr(entry)
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	for _, prefix := range hostingEdge {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}
