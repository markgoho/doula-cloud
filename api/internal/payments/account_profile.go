package payments

import (
	"strings"
	"unicode"
)

// AccountProfile is everything the BFF already knows about a Practice at
// the moment her Connect account is created, so Stripe's hosted form
// never has to ask her for it.
//
// #442 walked the difference against the Sandbox. An account created
// without these fields reports four requirements the Owner has to
// satisfy by hand -- configuration.merchant.mcc,
// configuration.merchant.statement_descriptor.descriptor,
// defaults.profile.business_url and defaults.profile.product_description
// -- and the hosted flow grows a whole business-details step to collect
// them. An account created with them reports none of the four: the
// walked flow went business type, personal details, bank details, public
// details, and never asked for an industry, a website or a product
// description at all. business_url moves to awaiting_action_from
// "stripe", which is #382's ongoing review rather than anything left for
// her to do.
//
// Two of the four are worth more than the clicks they save. The industry
// list has no doula or birth-work category, so a Practice picking one is
// guessing; and the descriptor Stripe derives when it is not told one
// comes from the website URL, which put FACEBOOK.COM/ROCHESTER onto a
// walked account's Clients' card statements (#421). Setting them is what
// makes both go right by default.
type AccountProfile struct {
	PracticeID   string
	PracticeName string
	// The website she declared, or the address of the page published for
	// her (#440). Never empty: PostConnectHandler refuses to create an
	// account for a Practice who has not answered.
	BusinessURL string
	// Stripe's internal-only risk description. Empty for a Practice who
	// declared her own site -- see website.StripeProfile.
	ProductDescription string
	// The text on her Clients' card statements, already normalized to
	// what Stripe accepts. Empty when the Practice's name cannot make a
	// legal one, in which case Stripe asks her for it as it does today.
	StatementDescriptor string
}

// DoulaMCC is the Merchant Category Code every Practice is created
// under: 8099, "Medical Services and Health Practitioners, Not
// Elsewhere Classified", which Stripe's own industry picker labels
// Personal services -> Health and wellness coaching.
//
// Chosen by us rather than by the Practice because Stripe's list has no
// doula or birth-work entry at all (#442's walk), so every Practice
// would be picking the same nearest thing, and any two picking
// differently would be an accident rather than a decision.
const DoulaMCC = "8099"

// The bounds Stripe enforces on a statement descriptor, established by
// probing the Sandbox rather than read off a doc page (#442):
//
//	"ABCD"                    -> must be at least 5 characters
//	"ABCDEFGHIJKLMNOPQRSTUVW" -> must be at most 22 characters
//	"12345"                   -> must contain at least one Latin character
//	"AB*CD EF"                -> cannot include *
const (
	minStatementDescriptor = 5
	maxStatementDescriptor = 22
)

// descriptorForbidden are the characters Stripe refuses inside a
// statement descriptor. Each was refused by name on a live create call;
// a backslash, tested alongside them, was accepted, so it is not here.
const descriptorForbidden = `<>"'*`

// StatementDescriptor turns a Practice's name into the text that appears
// on her Clients' card statements, or returns empty when the name cannot
// make one Stripe will accept.
//
// Empty is a real answer and not a failure. It means the account is
// created without a descriptor, Stripe asks for one in its own flow, and
// she is exactly where today's code leaves every Practice -- so a name
// made only of punctuation costs her one extra field rather than a
// failed onboarding.
//
// The result is a default, not a decision taken away from her: every
// Practice gets a full Stripe dashboard (Dashboard: full), where the
// descriptor is hers to change, and Stripe's own public-details step
// shows it to her before she submits.
func StatementDescriptor(practiceName string) string {
	// Whitespace collapses first, because a name split across two spaces
	// would otherwise spend its budget on them, and a card statement
	// renders a run of spaces as a gap nobody can read.
	cleaned := strings.Join(strings.Fields(practiceName), " ")
	cleaned = strings.Map(func(r rune) rune {
		if strings.ContainsRune(descriptorForbidden, r) {
			return -1
		}
		return r
	}, cleaned)
	// Dropping a character can strand a space at either end, and can
	// leave two where there was one.
	cleaned = strings.Join(strings.Fields(cleaned), " ")

	if len([]rune(cleaned)) > maxStatementDescriptor {
		cut := strings.TrimSpace(string([]rune(cleaned)[:maxStatementDescriptor]))
		// Back off to the last whole word where there is one to back off
		// to. A card statement reading "ROCHESTER BIRTH AND PO" looks like
		// a system that ran out of room; "ROCHESTER BIRTH AND" looks like a
		// name. One long unbroken word has no boundary to find, and a
		// boundary that leaves too little is worse than the blunt cut, so
		// both fall back to it.
		if space := strings.LastIndex(cut, " "); space >= minStatementDescriptor {
			cut = strings.TrimSpace(cut[:space])
		}
		cleaned = cut
	}
	if len([]rune(cleaned)) < minStatementDescriptor {
		return ""
	}
	if !strings.ContainsFunc(cleaned, func(r rune) bool {
		return unicode.In(r, unicode.Latin)
	}) {
		return ""
	}
	return cleaned
}
