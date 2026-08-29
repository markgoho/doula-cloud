package website_test

import (
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/website"
)

// TestNormalizeURL_AcceptsWhatStripeAccepts covers the addresses a
// Practice actually types. The social-profile rows are the point: #421
// established that a Facebook page URL satisfies Stripe's website field,
// so a rule that only admitted a Practice's own domain would refuse the
// answer most solo doulas have.
func TestNormalizeURL_AcceptsWhatStripeAccepts(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"https URL", ownSiteURL, ownSiteURL},
		{"http URL", "http://rochesterdoulas.com", "http://rochesterdoulas.com"},
		{"a Facebook page", "https://www.facebook.com/RochesterDoulas", "https://www.facebook.com/RochesterDoulas"},
		{"an Instagram profile", "https://instagram.com/rochester.doulas", "https://instagram.com/rochester.doulas"},
		{"no scheme, as she would type it", "rochesterdoulas.com", ownSiteURL},
		{"no scheme, with a path", "facebook.com/RochesterDoulas", "https://facebook.com/RochesterDoulas"},
		{"surrounding whitespace", "  https://rochesterdoulas.com  ", ownSiteURL},
		{"a query string", "https://linktr.ee/doulas?ref=1", "https://linktr.ee/doulas?ref=1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := website.NormalizeURL(c.in)
			if !ok {
				t.Fatalf("NormalizeURL(%q) rejected it, want accepted", c.in)
			}
			if got != c.want {
				t.Fatalf("NormalizeURL(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// TestNormalizeURL_RefusesWhatStripeWouldRefuse proves the malformed
// cases are stopped here rather than discovered halfway through her
// Stripe onboarding.
func TestNormalizeURL_RefusesWhatStripeWouldRefuse(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"empty", ""},
		{"whitespace only", "   "},
		{"a sentence", "my website is coming soon"},
		{"a bare word", "rochesterdoulas"},
		{"a host with no dot", "localhost"},
		{"a scheme we do not publish", "ftp://rochesterdoulas.com"},
		{"a javascript URL", "javascript:alert(1)"},
		{"a mailto", "mailto:hello@rochesterdoulas.com"},
		{"a trailing dot host", "https://rochesterdoulas."},
		{"a leading dot host", "https://.com"},
		{"no host at all", "https://"},
		{"a control character in the host", "https://roch\x7festerdoulas.com"},
		{"longer than the budget", "https://" + strings.Repeat("a", website.MaxURLLength) + ".com"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got, ok := website.NormalizeURL(c.in); ok {
				t.Fatalf("NormalizeURL(%q) = %q, accepted -- want refused", c.in, got)
			}
		})
	}
}

// TestNormalizeURL_RefusesAWithinBudgetInputThatNormalizesPastIt covers
// the one case where the prefix this function supplies is what pushes
// the result over the ceiling.
func TestNormalizeURL_RefusesAWithinBudgetInputThatNormalizesPastIt(t *testing.T) {
	// 2040 characters in, "https://" makes 2048 -- still inside. One
	// more and the normalized form is past it while the typed form was
	// not.
	host := strings.Repeat("a", website.MaxURLLength-len("https://")-len(".com")+1) + ".com"
	if got, ok := website.NormalizeURL(host); ok {
		t.Fatalf("NormalizeURL(len %d) = %q, accepted -- want refused", len(host), got)
	}
}

// TestValidate_OwnModeNeedsOnlyAURL proves a Practice declaring her own
// site is not made to write a service description she will never
// publish.
func TestValidate_OwnModeNeedsOnlyAURL(t *testing.T) {
	v, details := website.Validate(website.Request{
		Mode:   website.ModeOwn,
		OwnURL: "rochesterdoulas.com",
	})
	if details != nil {
		t.Fatalf("details = %v, want none", details)
	}
	if v.OwnURL != ownSiteURL {
		t.Fatalf("OwnURL = %q, want the normalized form", v.OwnURL)
	}
}

// TestValidate_HostedModeNeedsBothFacts proves the hosted page asks for
// exactly the two things only she has, and names each missing one
// against its own field.
func TestValidate_HostedModeNeedsBothFacts(t *testing.T) {
	_, details := website.Validate(website.Request{Mode: website.ModeHosted})
	if details["serviceDescription"] != website.MsgDescriptionNeeded {
		t.Fatalf("serviceDescription detail = %q, want %q", details["serviceDescription"], website.MsgDescriptionNeeded)
	}
	if details["cancellationPolicy"] != website.MsgPolicyNeeded {
		t.Fatalf("cancellationPolicy detail = %q, want %q", details["cancellationPolicy"], website.MsgPolicyNeeded)
	}
	if _, named := details["ownUrl"]; named {
		t.Fatalf("details named ownUrl on a hosted request: %v", details)
	}
}

// TestValidate_HostedModeTrimsAndKeepsAPreviouslyDeclaredURL proves the
// switch is reversible without retyping: a Practice moving to the hosted
// page keeps the URL she declared before, normalized.
func TestValidate_HostedModeTrimsAndKeepsAPreviouslyDeclaredURL(t *testing.T) {
	v, details := website.Validate(website.Request{
		Mode:               website.ModeHosted,
		OwnURL:             "  rochesterdoulas.com ",
		ServiceDescription: "  Birth and postpartum doula support in Monroe County.  ",
		CancellationPolicy: " Two weeks' notice for a full refund. ",
	})
	if details != nil {
		t.Fatalf("details = %v, want none", details)
	}
	if v.ServiceDescription != "Birth and postpartum doula support in Monroe County." {
		t.Fatalf("ServiceDescription = %q, want it trimmed", v.ServiceDescription)
	}
	if v.CancellationPolicy != "Two weeks' notice for a full refund." {
		t.Fatalf("CancellationPolicy = %q, want it trimmed", v.CancellationPolicy)
	}
	if v.OwnURL != ownSiteURL {
		t.Fatalf("OwnURL = %q, want the earlier declaration carried forward", v.OwnURL)
	}
}

// TestValidate_HostedModeDropsAnUnusableCarriedURL proves a malformed
// leftover never fails a hosted publish: mode 'hosted' does not ask for
// a URL, so one that cannot be parsed is dropped rather than refused.
func TestValidate_HostedModeDropsAnUnusableCarriedURL(t *testing.T) {
	v, details := website.Validate(website.Request{
		Mode:               website.ModeHosted,
		OwnURL:             "coming soon",
		ServiceDescription: "Birth support.",
		CancellationPolicy: policyText,
	})
	if details != nil {
		t.Fatalf("details = %v, want none", details)
	}
	if v.OwnURL != "" {
		t.Fatalf("OwnURL = %q, want it dropped", v.OwnURL)
	}
}

// TestValidate_OwnModeDistinguishesMissingFromMalformed proves the two
// URL failures read differently: one asks her for an answer, the other
// tells her the answer is not a web address.
func TestValidate_OwnModeDistinguishesMissingFromMalformed(t *testing.T) {
	_, missing := website.Validate(website.Request{Mode: website.ModeOwn})
	if missing["ownUrl"] != website.MsgURLRequired {
		t.Fatalf("missing detail = %q, want %q", missing["ownUrl"], website.MsgURLRequired)
	}
	_, malformed := website.Validate(website.Request{Mode: website.ModeOwn, OwnURL: "coming soon"})
	if malformed["ownUrl"] != website.MsgURLMalformed {
		t.Fatalf("malformed detail = %q, want %q", malformed["ownUrl"], website.MsgURLMalformed)
	}
}

// TestValidate_BudgetIsEnforcedInRunesNotBytes proves the count the
// server enforces is the count the screen shows her -- a page of
// accented characters is 500 characters, not 500 bytes.
func TestValidate_BudgetIsEnforcedInRunesNotBytes(t *testing.T) {
	atBudget := strings.Repeat("é", website.MaxFactLength)
	_, details := website.Validate(website.Request{
		Mode:               website.ModeHosted,
		ServiceDescription: atBudget,
		CancellationPolicy: atBudget,
	})
	if details != nil {
		t.Fatalf("details = %v at exactly the budget, want none", details)
	}

	overBudget := strings.Repeat("é", website.MaxFactLength+1)
	_, over := website.Validate(website.Request{
		Mode:               website.ModeHosted,
		ServiceDescription: overBudget,
		CancellationPolicy: overBudget,
	})
	if over["serviceDescription"] != website.MsgTooLong || over["cancellationPolicy"] != website.MsgTooLong {
		t.Fatalf("details = %v, want both fields over budget", over)
	}
}

// TestValidate_BudgetAppliesInOwnModeToo proves the facts carried
// forward by an 'own' declaration are checked as well: sending something
// the database would refuse has to fail here rather than as a 500.
func TestValidate_BudgetAppliesInOwnModeToo(t *testing.T) {
	_, details := website.Validate(website.Request{
		Mode:               website.ModeOwn,
		OwnURL:             ownSiteURL,
		ServiceDescription: strings.Repeat("a", website.MaxFactLength+1),
	})
	if details["serviceDescription"] != website.MsgTooLong {
		t.Fatalf("details = %v, want the carried description refused", details)
	}
}

// TestValidate_RefusesAnUnknownMode proves 'undeclared' is a shape the
// API reports and not one a caller may write: there is no route back to
// having never answered.
func TestValidate_RefusesAnUnknownMode(t *testing.T) {
	for _, mode := range []string{"", "undeclared", "Own", "draft"} {
		_, details := website.Validate(website.Request{Mode: mode})
		if details["mode"] != website.MsgInvalidMode {
			t.Fatalf("mode %q: details = %v, want %q", mode, details, website.MsgInvalidMode)
		}
	}
}

// TestFormatUpdatedAt_IsEmptyWhenNobodyHasAnswered proves the screen is
// handed an empty string rather than the zero time, so it has one thing
// to test for rather than two.
func TestFormatUpdatedAt_IsEmptyWhenNobodyHasAnswered(t *testing.T) {
	if got := website.FormatUpdatedAt(time.Time{}, false); got != "" {
		t.Fatalf("FormatUpdatedAt(invalid) = %q, want empty", got)
	}
	at := time.Date(2026, 8, 29, 14, 30, 0, 0, time.UTC)
	if got := website.FormatUpdatedAt(at, true); got != "2026-08-29T14:30:00Z" {
		t.Fatalf("FormatUpdatedAt = %q, want RFC 3339 in UTC", got)
	}
}

// TestSlugify covers the shapes a real Practice name arrives in. The
// slug is minted once and is then a URL Stripe holds under an ongoing
// review (#382), so what this function returns is not a display string
// that can be tidied up later.
const plainSlug = "rochester-doulas"

func TestSlugify(t *testing.T) {
	const practiceID = "3f2a91c4-77b1-4f0e-9d21-8c6b5a4e3d10"

	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{"a plain name", "Rochester Doulas", plainSlug},
		{"punctuation and runs of space", "Rochester  Doulas, LLC.", "rochester-doulas-llc"},
		{"already hyphenated", "Rochester-Doulas", plainSlug},
		{"leading and trailing noise", "  ...Rochester Doulas!  ", plainSlug},
		{"digits survive", "Birth 24/7", "birth-24-7"},
		{
			"a long name is cut on a word boundary",
			"The Greater Rochester and Monroe County Birth and Postpartum Doula Collective",
			"the-greater-rochester-and-monroe-county-birth-and-postpartum",
		},
		// Not transliterated: an id-derived slug is ugly and stable,
		// where a guess at what the letters sound like in English is
		// neither.
		{"a name in another script falls back to the id", "助産師", "practice-3f2a91c4"},
		{"a name of only punctuation falls back too", "!!!", "practice-3f2a91c4"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := website.Slugify(tc.in, practiceID); got != tc.want {
				t.Fatalf("Slugify(%q) = %q, want %q", tc.in, got, tc.want)
			}
			if len(website.Slugify(tc.in, practiceID)) > website.MaxSlugLength {
				t.Fatalf("Slugify(%q) is longer than %d", tc.in, website.MaxSlugLength)
			}
		})
	}
}

// TestSlugify_ShortIDIsUsedWhole guards the fallback against an id that
// is shorter than the eight characters it slices.
func TestSlugify_ShortIDIsUsedWhole(t *testing.T) {
	if got := website.Slugify("!!!", "abc"); got != "practice-abc" {
		t.Fatalf("Slugify with a short id = %q, want %q", got, "practice-abc")
	}
}

// TestSlugCandidate proves the collision sequence a Practice sharing a
// name with another one walks through. A counted suffix rather than a
// random string: the slug is a public URL she reads out loud.
func TestSlugCandidate(t *testing.T) {
	const practiceID = "3f2a91c4-77b1-4f0e-9d21-8c6b5a4e3d10"

	if got := website.SlugCandidate("Rochester Doulas", practiceID, 0); got != plainSlug {
		t.Fatalf("first candidate = %q", got)
	}
	if got := website.SlugCandidate("Rochester Doulas", practiceID, 1); got != "rochester-doulas-2" {
		t.Fatalf("second candidate = %q", got)
	}
	if got := website.SlugCandidate("Rochester Doulas", practiceID, 8); got != "rochester-doulas-9" {
		t.Fatalf("ninth candidate = %q", got)
	}

	// A name already at the ceiling gives way to the suffix rather than
	// producing something the column would refuse.
	long := "The Greater Rochester and Monroe County Birth and Postpartum Doula Collective"
	got := website.SlugCandidate(long, practiceID, 1)
	if len(got) > website.MaxSlugLength {
		t.Fatalf("SlugCandidate(long, 1) = %q, longer than %d", got, website.MaxSlugLength)
	}
	if !strings.HasSuffix(got, "-2") {
		t.Fatalf("SlugCandidate(long, 1) = %q, want a -2 suffix", got)
	}
}
