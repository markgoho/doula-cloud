package staffauth

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/feedback"
	"doula-cloud/api/internal/ratelimit"
)

// MsgFeedbackKindNeeded is the kind field on the Feedback form -- a
// missing value and one that names none of the three radio choices both
// refuse with this same sentence (#1523).
const MsgFeedbackKindNeeded = "Select the kind of feedback"

// MsgFeedbackTextTooLong is the text field, refused past
// feedback.MaxTextRunes characters.
const MsgFeedbackTextTooLong = "Shorten the feedback to 5,000 characters or less"

// MsgFeedbackPracticeIDInvalid answers a practiceId that is not even a
// well-formed id -- a malformed body rather than a Practice she does not
// hold, which FeedbackHandler tells apart with its own 403 below.
const MsgFeedbackPracticeIDInvalid = "practiceId does not name a Practice"

// MsgFeedbackPracticeNotHeld is #1523's own refusal: a practiceId naming
// a real Practice the caller holds no membership at. Not a Details entry
// -- there is no form control a body-only field maps onto, so this stays
// the summary Message only, the same shape MsgWorkStateRequired takes.
const MsgFeedbackPracticeNotHeld = "practiceId names a Practice you do not hold a membership at"

// FeedbackRequest is the whole body of a Staff member's send (#1523).
// PracticeID is nil from /account, and set only when the screen that
// sent it was under practices/[practiceId] -- FeedbackHandler checks it
// against the caller's own membership rather than trusting it, and the
// roles it stores come from that membership, never from this request.
type FeedbackRequest struct {
	feedback.ClientInput
	PracticeID *string `json:"practiceId"`
}

// FeedbackResponse is the whole of what a send answers: the new piece's
// id and nothing else (#1523's own AC).
type FeedbackResponse struct {
	ID string `json:"id"`
}

// feedbackRules is #1523's own sizing: 20 per hour, per sender.
// SessionCookieRule is the closest fit this toolkit offers to "per
// sender" for an already-signed-in write -- the same choice
// verifyRequestRules and mfaRecoveryRotateRules make for a low-risk,
// session-gated self-service action, and unlike those two this one has
// an AC that names a number rather than "generous".
var feedbackRules = []ratelimit.Rule{
	ratelimit.SessionCookieRule(20, time.Hour),
}

// FeedbackHandler lets a signed-in Staff member send a piece of Feedback
// (#1523) from any screen, pre-Practice by shape (mountSessionRoutes):
// the route carries no {practiceId}, and a practiceId in the body is
// optional and independently checked, exactly like UpdateWorkStateHandler's
// own self-only shape above it in this file's neighbors.
//
// Not wrapped in idempotency.Wrap. idempotency_keys (00027) scopes a key
// by (practice_id, staff_id), both NOT NULL -- a piece sent from /account
// carries no practice_id at all, so no key could ever be formed for that
// call, the same gap message.ClientCreateHandler's own doc comment
// records for a Portal caller with neither. It is also not a gap worth
// closing here: Q2's own resolution is "redundant feedback is better
// than no feedback" (#1498), so a retried send landing twice is not the
// failure this route exists to prevent.
func FeedbackHandler(db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, uid, _, ok := authn.Begin(w, r, db, authn.TierStaff)
		if !ok {
			return
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()

		var req FeedbackRequest
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

		self, ok := requireSelf(w, r, tx, uid)
		if !ok {
			return
		}

		var practiceID *string
		var rolesLiteral *string
		if req.PracticeID != nil {
			pid := *req.PracticeID
			if _, err := uuid.Parse(pid); err != nil {
				apierr.WriteError(w, MsgFeedbackPracticeIDInvalid, http.StatusBadRequest)
				return
			}
			roles, found, err := staffRolesAt(r.Context(), tx, self.ID, pid)
			if err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
			if !found {
				apierr.WriteError(w, MsgFeedbackPracticeNotHeld, http.StatusForbidden)
				return
			}
			practiceID = &pid
			rolesLiteral = feedback.BuildRolesLiteral(roles)
		}

		staffID := self.ID
		id := uuid.NewString()
		row := feedback.Row{
			ID:           id,
			Kind:         req.Kind,
			Text:         req.Text,
			PageURL:      req.Page.URL,
			RouteID:      req.Page.Route.ID,
			AppBuild:     req.AppBuild,
			ScreenWidth:  req.ScreenWidth,
			Browser:      feedback.ParseBrowser(r.Header.Get("User-Agent")),
			StaffID:      &staffID,
			PracticeID:   practiceID,
			RolesLiteral: rolesLiteral,
			SentAt:       clock.Now(r.Context()),
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

		apierr.WriteJSON(w, http.StatusCreated, FeedbackResponse{ID: id})
	})
}

// staffRolesAt reads staffID's roles at practiceID off practice_memberships
// itself, rather than trusting anything the request sent -- #1523's own
// line: "the role(s), checked against practice_memberships, never taken
// from the body". found is false when no membership row exists at all,
// which FeedbackHandler refuses with MsgFeedbackPracticeNotHeld; a
// membership with zero roles still reports found=true and an empty
// slice, since holding a membership with no roles assigned yet is a real
// state (00002's own "a membership can start with zero roles").
func staffRolesAt(ctx context.Context, tx *sql.Tx, staffID, practiceID string) (roles []string, found bool, err error) {
	var literal string
	err = tx.QueryRowContext(ctx,
		`SELECT array_to_string(roles, ',') FROM practice_memberships WHERE staff_id = $1 AND practice_id = $2`,
		staffID, practiceID,
	).Scan(&literal)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, false, fmt.Errorf("staffauth: read membership roles: %w", err)
	}
	if literal == "" {
		return []string{}, true, nil
	}
	return strings.Split(literal, ","), true, nil
}
