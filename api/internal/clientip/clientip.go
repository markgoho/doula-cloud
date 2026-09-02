// Package clientip resolves the real caller address behind Cloud Run's
// Google Front End. Extracted from contracts/sign.go (#602) once a second
// consumer (ratelimit) needed the same logic -- this repo's bar for
// pulling shared code out of its first home.
package clientip

import (
	"net"
	"net/http"
	"strings"
)

// From returns the caller's address, for use as an ESIGN signer_ip
// (contracts) or a rate-limit dimension (ratelimit). The BFF runs behind
// Cloud Run's Google Front End, which terminates the caller's own TLS
// connection and sets X-Forwarded-For's first entry to that connection's
// real peer address itself -- a caller can't spoof this the way it could
// a header GFE merely passed through, since GFE is the one writing it,
// not relaying client-supplied content. r.RemoteAddr, by contrast, is
// GFE's own proxy address at that point, not the caller's -- only useful
// as the local-dev/test fallback when there's no GFE in front of the
// process and the header is absent.
func From(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		first, _, _ := strings.Cut(xff, ",")
		return strings.TrimSpace(first)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	// coverage:ignore reason: net/http always sets RemoteAddr in host:port form, not exercised by unit tests
	if err != nil {
		// coverage:ignore reason: net/http always sets RemoteAddr in host:port form, not exercised by unit tests
		return r.RemoteAddr
	}
	return host
}
