package website

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// SiteBaseURL is where a hosted Practice page is published. The same
// value hugo/hugo.toml carries as its baseURL, and the two have to
// agree: this is the address the BFF hands Stripe, and Hugo is what
// makes something answer at it.
//
// A constant rather than an environment variable, because it is a fact
// about the product and not about a deployment -- there is one Doula
// Cloud website, at one address, and a per-environment value would only
// mean telling Stripe about a host that does not serve Practice pages.
const SiteBaseURL = "https://doula.cloud"

// HostedPageURL is the public address of the page published for a
// Practice under slug. Assigned once and never recomputed (00046), so
// this composes rather than derives.
func HostedPageURL(slug string) string {
	return SiteBaseURL + "/p/" + slug
}

// StripeProfile is what a Practice's website declaration contributes to
// her Stripe Connect account, resolved to the single URL Stripe is told
// about whichever answer she gave.
//
// Declared is the gate #442 enforces: Stripe's hosted onboarding accepts
// an empty website field, lets her finish every remaining step, and
// returns her "done" with card_payments restricted and nothing on screen
// saying why (#421). A Practice with no declaration must not be sent
// into that flow at all.
//
// ProductDescription is Stripe's internal-only risk field, and it is
// empty for a Practice who declared her own site: #440 asks for a
// service description only from a Practice publishing a page here, and
// inventing one for the others would be putting words in her mouth on a
// form Stripe underwrites her against. She types it in Stripe's own flow
// instead.
//
// PageFailed is #443's second gate, and it is a narrower thing than
// Declared: she has answered, and a probe of the page we publish for
// her found nothing there. Sending her into onboarding then would hand
// Stripe a URL that 404s, and #382 established the review of that URL
// is ongoing with no published SLA -- so the rejection would arrive
// weeks later with no visible cause. False for a Practice on her own
// website: what is at the far end of an address she gave us is hers to
// keep working, and probing it would be checking up on her.
type StripeProfile struct {
	Declared           bool
	URL                string
	ProductDescription string
	PageFailed         bool
}

// ReadStripeProfile resolves practiceID's declaration into what Stripe
// should be told. A Practice who has not answered comes back with
// Declared false and nothing else -- not an error, because "has she
// answered?" is the question the caller is asking.
func ReadStripeProfile(ctx context.Context, tx *sql.Tx, practiceID string) (StripeProfile, error) {
	var (
		mode        string
		ownURL      sql.NullString
		description sql.NullString
		slug        sql.NullString
		pageState   sql.NullString
	)
	err := tx.QueryRowContext(ctx,
		`SELECT mode, own_url, service_description, slug, page_state
		   FROM practice_websites WHERE practice_id = $1`, practiceID,
	).Scan(&mode, &ownURL, &description, &slug, &pageState)
	if errors.Is(err, sql.ErrNoRows) {
		return StripeProfile{}, nil
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return StripeProfile{}, fmt.Errorf("website: read stripe profile: %w", err)
	}

	if mode == ModeHosted {
		return StripeProfile{
			Declared:           true,
			URL:                HostedPageURL(slug.String),
			ProductDescription: strings.TrimSpace(description.String),
			PageFailed:         pageState.String == PageStateFailed,
		}, nil
	}
	return StripeProfile{Declared: true, URL: ownURL.String}, nil
}
