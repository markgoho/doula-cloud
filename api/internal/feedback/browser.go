package feedback

import "regexp"

// UnknownBrowser is what ParseBrowser returns for a User-Agent it does
// not recognize -- a real value the row can carry rather than an empty
// string a reader has to know means "unparsed".
const UnknownBrowser = "Unknown browser"

// browserPattern is one recognized browser family: a regular expression
// whose first capture group is its major version, tried against the raw
// User-Agent header. Order matters -- Edge's User-Agent also contains
// "Chrome/", and Chrome's also contains "Safari/", so each entry is
// tried only after the families whose own token it is a substring of.
var browserPatterns = []struct {
	name string
	re   *regexp.Regexp
}{
	{"Edge", regexp.MustCompile(`Edg/(\d+)`)},
	{"Chrome", regexp.MustCompile(`Chrome/(\d+)`)},
	{"Firefox", regexp.MustCompile(`Firefox/(\d+)`)},
	// Chrome's own User-Agent carries "Safari/<version>" with no
	// "Version/" token before it, so requiring both is what keeps this
	// entry from matching a Chrome request that reached here only
	// because a future reorder put it first.
	{"Safari", regexp.MustCompile(`Version/(\d+).*Safari/`)},
}

// ParseBrowser reads userAgent -- the request's raw User-Agent header --
// and returns the browser family plus its major version, e.g. "Safari
// 18" (#1523's own example). The raw header is never stored; this is the
// whole of what the BFF keeps from it. Returns UnknownBrowser for a
// header naming none of the four families above, including an empty
// string.
func ParseBrowser(userAgent string) string {
	for _, p := range browserPatterns {
		if m := p.re.FindStringSubmatch(userAgent); m != nil {
			return p.name + " " + m[1]
		}
	}
	return UnknownBrowser
}
