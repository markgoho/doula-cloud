// Package clientip resolves the real caller address behind Cloud Run's
// Google Front End and the Firebase Hosting rewrite. Extracted from
// contracts/sign.go (#602) once a second consumer (ratelimit) needed the
// same logic -- this repo's bar for pulling shared code out of its first
// home.
//
// # Why the header is read from the right (#1865)
//
// X-Forwarded-For is a list every hop appends to. Google's front end
// does not check or remove what the caller sent: its load balancer
// documentation says it appends after "any value already present" and
// "does not verify any IP addresses that precede" its own entries
// (https://docs.cloud.google.com/load-balancing/docs/https, read
// 2026-10-24). So the left-most entry is whatever text the caller chose,
// and only the entries on the right, written by a proxy from the address
// of the connection it accepted, can be trusted.
//
// A request reaches the BFF one of two ways, and a caller picks which:
//
//   - Direct to the run.app URL. Cloud Run's ingress is "all" (gcloud
//     run services describe doula-api, 2026-10-24), because Stripe's
//     webhooks and Cloud Scheduler call it there, and because the
//     Hosting rewrite itself needs it (see below). Google's front end
//     appends the caller's address: "<caller text>, <caller>".
//   - Through the Firebase Hosting rewrite of /api/**. Hosting does not
//     pass the caller's text on (observed, below): it writes the
//     browser's address, then Google's front end appends Hosting's:
//     "<caller>, <Hosting edge>".
//
// A fixed count of hops to skip is right for only one of the two, so
// From decides by address instead: it skips the right-most entry only
// when that entry is inside hostingEdgeRanges, and never more than that
// one, since Hosting is one hop. A direct caller cannot put a Hosting
// address in the right-most place, because Google's front end writes
// that one from the TCP connection.
//
// # What was observed
//
// 2026-10-24, Cloud Logging's httpRequest.remoteIp for doula-api over
// the previous 30 days: every request that came through Hosting arrived
// from 66.102.0.0/20, 66.249.64.0/19, 74.125.0.0/16 or 192.178.0.0/15
// (#1865), and also from 64.233.160.0/19 (#1867): six of the deploy
// smoke test's Hosting requests to /api/hello, 2026-09-27 to 2026-10-24,
// came from 64.233.172.x, each beside the same run's direct request from
// a GitHub runner. Until #1867 those six collapsed to Google's address.
// All five are in Google's own published ranges
// (https://www.gstatic.com/ipranges/goog.json, created
// 2026-10-24T01:06:28) and none overlaps a range a Google Cloud customer
// can rent (https://www.gstatic.com/ipranges/cloud.json, same date), so
// no caller can open a connection from one with a VM of its own.
//
// # Why the ranges, and not something stronger (#1867, ADR-0056)
//
// Ingress cannot narrow the direct path. Cloud Run's ingress
// documentation (https://docs.cloud.google.com/run/docs/securing/ingress,
// read 2026-10-24) does not list Firebase Hosting among the sources that
// count as internal, and lists it among the services that call the
// default run.app URL, so "internal", "internal-and-cloud-load-balancing"
// and turning off the default URL all refuse the rewrite. Nor does Hosting
// add anything a direct caller cannot forge: a Cloud Run rewrite takes
// serviceId, region and pinTag only, firebase.json's headers attribute
// sets response headers, and Hosting presents no Google-signed token
// (https://firebase.google.com/docs/hosting/full-config, read
// 2026-10-24).
//
// So the ranges stay, with one residual that ADR-0056 accepts: they are
// Google's shared egress, not Hosting's alone, and a Google-run service
// that connects from inside them and lets its user set X-Forwarded-For
// could call the run.app URL with a made-up entry, which From would
// return. No such service is verified to exist.
//
// The ranges going stale is checked every day:
// .github/workflows/hosting-edge-check.yml runs cmd/hostingedge, which
// fails when an address from Google's own egress reaches doula-api from
// outside hostingEdgeRanges, or a range here stops being Google's own.
// Run over the same 30 days without 64.233.160.0/19, it names each of
// the six 64.233.172.x addresses. A failure is a range to add
// here, with its evidence above, or the residual in use.
//
// 2026-10-24, a request with a caller-written entry, on each path and
// on each side of the #1865 fix: four GET /api/offers/{offerId}
// requests from 8.47.98.19, each with a made-up first X-Forwarded-For
// entry, read back from rate_limit_buckets' offer_read:ip:* rows (#1865
// has the probe and the rows):
//
//   - Old code (first entry), through Hosting, sent 198.51.100.186:
//     counted against 8.47.98.19. Hosting dropped the caller's text.
//   - Old code, direct to run.app, sent 198.51.100.187: a row keyed
//     198.51.100.187. Google's front end kept the caller's text, so the
//     old code returned an address the caller chose.
//   - New code, through Hosting (198.51.100.188) and direct
//     (198.51.100.189): no row for either; both counted against
//     8.47.98.19, in the same row as the first request (count 3).
//
// So Hosting adds exactly one hop on the right, and From returns the
// connecting address on both paths.
package clientip

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// hostingEdgeRanges holds the address ranges the Firebase Hosting
// rewrite connects to Cloud Run from -- the only proxy From skips past.
// See the package comment for where each range was observed.
var hostingEdgeRanges = []netip.Prefix{
	netip.MustParsePrefix("64.233.160.0/19"),
	netip.MustParsePrefix("66.102.0.0/20"),
	netip.MustParsePrefix("66.249.64.0/19"),
	netip.MustParsePrefix("74.125.0.0/16"),
	netip.MustParsePrefix("192.178.0.0/15"),
}

// HostingEdgeRanges returns a copy of hostingEdgeRanges, for the daily
// check in cmd/hostingedge that compares them with the addresses Hosting
// actually connects from. The copy keeps the check from changing what
// From trusts.
func HostingEdgeRanges() []netip.Prefix {
	return append([]netip.Prefix(nil), hostingEdgeRanges...)
}

// From returns the caller's address, for use as an ESIGN signer_ip
// (contracts), the terms_accepted record (staffauth), or a rate-limit
// dimension (ratelimit): the right-most X-Forwarded-For entry, or the
// one before it when the right-most is a Hosting edge. With no header
// -- the local and e2e stack, with no proxy in front of the process --
// it is r.RemoteAddr's host.
func From(r *http.Request) string {
	entries := forwardedFor(r)
	if n := len(entries); n > 0 {
		if n > 1 && isHostingEdge(entries[n-1]) {
			return entries[n-2]
		}
		return entries[n-1]
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

// isHostingEdge reports whether entry is an address inside hostingEdgeRanges.
// An entry that is not an address at all is caller-written text, never
// a proxy, so it is not skipped.
func isHostingEdge(entry string) bool {
	addr, err := netip.ParseAddr(entry)
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	for _, prefix := range hostingEdgeRanges {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}
