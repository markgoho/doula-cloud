package webhookarrival

import (
	"io"
	"log"
	"net/http"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/stripemode"
)

// MaxBodyBytes bounds how much of a Stripe webhook request body is read.
const MaxBodyBytes = 1 << 20

// OpenStripe is how every Stripe webhook route opens: it reads the body,
// has verify check the Stripe-Signature header against it, and refuses an
// event whose livemode does not match the keys' mode. Each refusal marks
// its outcome and answers 400, and reports ok=false, the caller's signal
// to return. identify reads the event's id and livemode flag for the log
// line a mode mismatch writes, which carries label.
func OpenStripe[E any](
	w http.ResponseWriter,
	r *http.Request,
	label string,
	mode stripemode.Mode,
	verify func(body []byte, sigHeader string) (E, error),
	identify func(E) (id string, livemode bool),
) (event E, ok bool) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxBodyBytes))
	if err != nil {
		Mark(r.Context(), BodyRefused)
		apierr.WriteError(w, "invalid request body", http.StatusBadRequest)
		return event, false
	}

	event, err = verify(body, r.Header.Get("Stripe-Signature"))
	if err != nil {
		Mark(r.Context(), SignatureRefused)
		apierr.WriteError(w, "invalid signature", http.StatusBadRequest)
		return event, false
	}

	id, livemode := identify(event)
	if !mode.Matches(livemode) {
		log.Printf("%s: refusing event %s: livemode %v, keys are in %s mode", label, id, livemode, mode)
		Mark(r.Context(), LivemodeRefused)
		apierr.WriteError(w, "Event mode does not match the Stripe keys", http.StatusBadRequest)
		return event, false
	}
	return event, true
}
