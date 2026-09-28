package clientauth

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/feedback"
)

// MsgFeedbackKindNeeded is the kind field on the Feedback form -- the
// same sentence staffauth.MsgFeedbackKindNeeded uses, kept as a separate
// local constant so apierr's TestDetailsWording -- which resolves an
// identifier only against its own package's top-level constants -- can
// actually read it rather than skip an unresolved cross-package
// selector.
const MsgFeedbackKindNeeded = "Select the kind of feedback"

// MsgFeedbackTextTooLong is the text field, refused past
// feedback.MaxTextRunes characters -- staffauth.MsgFeedbackTextTooLong's
// own sentence, kept local for the same reason.
const MsgFeedbackTextTooLong = "Shorten the feedback to 5,000 characters or less"

// MsgFeedbackEngagementIDInvalid answers an engagementId that is not
// even a well-formed id.
const MsgFeedbackEngagementIDInvalid = "engagementId does not name an Engagement"

// MsgFeedbackEngagementNotHeld is #1523's own refusal: an engagementId
// naming a real Engagement this Portal Account does not reach.
const MsgFeedbackEngagementNotHeld = "engagementId names an Engagement you do not hold"

// PortalFeedbackRequest is the whole body of a Client's send (#1523).
// EngagementID is nil when the sending screen was not inside one
// Engagement; FeedbackHandler checks a given one against the caller's
// own Portal Account the same way clientauth.Middleware checks a path
// :engagementId, rather than trusting it.
type PortalFeedbackRequest struct {
	feedback.ClientInput
	EngagementID *string `json:"engagementId"`
}

// PortalFeedbackResponse is the whole of what a send answers: the new
// piece's id and nothing else (#1523's own AC).
type PortalFeedbackResponse struct {
	ID string `json:"id"`
}

// FeedbackHandler lets a signed-in Client send a piece of Feedback
// (#1523) from any portal screen. Authenticated the same way
// EndAllSessionsHandler and RequestAddressChangeHandler are -- a live
// portal session checked against portal_accounts rather than run through
// clientauth.Middleware -- because a Portal Account reaches Clients at
// more than one Practice (ADR-0015) and most of this route's screens
// name no single Engagement at all.
//
// Not wrapped in idempotency.Wrap, the same gap message.ClientCreateHandler's
// own doc comment already names: idempotency_keys (00027) scopes a key
// by (practice_id, staff_id), both NOT NULL, and a Portal caller has
// neither. Q2's own resolution -- "redundant feedback is better than no
// feedback" (#1498) -- is also why closing that gap is not worth doing
// here even where a practice_id happens to be on hand.
func FeedbackHandler(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, uid, _, ok := authn.Begin(w, r, db, authn.TierPortal)
		if !ok {
			return
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()

		// A __session cookie is one cookie for two populations
		// (ADR-0026), so a signed-in Staff member's session reaches this
		// route with a perfectly valid uid that names no Portal Account.
		holds, err := isPortalAccount(r.Context(), tx, uid)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !holds {
			apierr.WriteError(w, msgNotAPortalAccount, http.StatusForbidden)
			return
		}

		var req PortalFeedbackRequest
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}

		if !feedback.ValidKind(req.Kind) {
			apierr.WriteFieldError(w, http.StatusBadRequest, apierr.CodeInvalidArgument, "kind", MsgFeedbackKindNeeded)
			return
		}
		if !feedback.ValidText(req.Text) {
			apierr.WriteFieldError(w, http.StatusBadRequest, apierr.CodeInvalidArgument, "text", MsgFeedbackTextTooLong)
			return
		}

		var clientID, practiceID *string
		if req.EngagementID != nil {
			eid := *req.EngagementID
			if _, err := uuid.Parse(eid); err != nil {
				apierr.WriteError(w, MsgFeedbackEngagementIDInvalid, http.StatusBadRequest)
				return
			}
			// resolveOwningClient sets app.current_identity_uid and, once
			// it finds the owning row, app.current_client_id -- the same
			// resolution clientauth.Middleware runs off :engagementId in
			// the path, reused here against one carried in the body
			// instead.
			cid, owns, err := resolveOwningClient(r.Context(), tx, uid, eid)
			if err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
			if !owns {
				apierr.WriteError(w, MsgFeedbackEngagementNotHeld, http.StatusForbidden)
				return
			}
			pid, err := practiceIDOfEngagement(r.Context(), tx, eid)
			if err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
			clientID, practiceID = &cid, &pid
		} else {
			// No Engagement to resolve, but the insert policy still needs
			// app.current_identity_uid set to check portal_account against
			// it -- resolveOwningClient's own first step, done directly
			// since there is no Engagement here to also resolve.
			if _, err := tx.ExecContext(r.Context(), `SELECT set_config('app.current_identity_uid', $1, true)`, uid); err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
		}

		portalAccount := uid
		id := uuid.NewString()
		row := feedback.Row{
			ID:            id,
			Kind:          req.Kind,
			Text:          req.Text,
			PageURL:       req.Page.URL,
			RouteID:       req.Page.Route.ID,
			AppBuild:      req.AppBuild,
			ScreenWidth:   req.ScreenWidth,
			Browser:       feedback.ParseBrowser(r.Header.Get("User-Agent")),
			PortalAccount: &portalAccount,
			PracticeID:    practiceID,
			ClientID:      clientID,
			SentAt:        clock.Now(r.Context()),
		}
		if err := feedback.Insert(r.Context(), tx, row); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB commit failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		committed = true

		apierr.WriteJSON(w, http.StatusCreated, PortalFeedbackResponse{ID: id})
	})
}

// practiceIDOfEngagement reads which Practice engagementID belongs to.
// Run after resolveOwningClient has already set app.current_client_id to
// the caller's own resolved Client, so this SELECT is itself scoped by
// engagements_client_visibility -- the ownership already proved, read
// again rather than threaded through as a second return value, since
// resolveOwningClient's own signature is shared with clientauth.Middleware
// and not this route's to widen.
func practiceIDOfEngagement(ctx context.Context, tx *sql.Tx, engagementID string) (string, error) {
	var practiceID string
	if err := tx.QueryRowContext(ctx, `SELECT practice_id FROM engagements WHERE id = $1`, engagementID).Scan(&practiceID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return "", fmt.Errorf("clientauth: read engagement practice: %w", err)
	}
	return practiceID, nil
}
