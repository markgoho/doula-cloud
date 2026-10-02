package staffauth

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/feedback"
)

// MsgFounderRouteNotFound is the one sentence every founder route gives
// a caller it will not serve and a piece it cannot find. They are the
// same sentence on purpose (#1526): a Staff member who is not the
// founder must not be able to tell "this route is not yours" from "there
// is no such piece", or the refusal itself would say the route exists.
const MsgFounderRouteNotFound = "not found"

// MsgFounderMFARequired is what the founder's own session is told when
// it carries no second factor. writeMFARequired's sentence names a
// Practice, and these routes have none.
const MsgFounderMFARequired = "this page requires a second sign-in factor"

// FounderConfig is what the founder routes read from the environment.
type FounderConfig struct {
	// StaffID is FOUNDER_STAFF_ID: the `staff.id` of the one person the
	// founder routes serve. Empty refuses everybody.
	StaffID string
	// FeedbackRepo is GITHUB_FEEDBACK_REPO, "owner/name". The read page's
	// link to a piece's issue is built from it, so the app never names
	// the repository itself. Empty leaves the link off.
	FeedbackRepo string
}

// FounderOnly admits one person: the Staff member whose id is
// founderStaffID, on a session that carries a second factor (#1526,
// decided on #1499). It is OwnerOnly's counterpart for a route with no
// Practice in it. OwnerOnly is a role list GatedRouter.Get checks
// downstream of Middleware, and Middleware needs a {practiceId}; a
// founder route has none, so this is a wrapper of its own, mounted
// through OpenGet in the GET /api/staff/session shape.
//
// # The order of its refusals
//
//  1. No live Staff session: authn.Begin's 401.
//  2. A session whose identity resolves to no `staff` row: requireSelf's
//     404, the answer every pre-Practice route gives (#1182).
//  3. Anybody but the founder, or no founder configured: 404, carrying
//     the same sentence a missing piece does, so the route's existence
//     is not revealed.
//  4. The founder with no second factor: 403 MFA_REQUIRED, which the app
//     already routes into TOTP enrollment (mfaenroll.go).
//
// The second factor is checked last. Checked earlier, a Staff member
// with no second factor would be told to enroll for a route she was
// never going to be let into, which says the route is real.
//
// requireMFA in Middleware is a Practice's own setting and cannot apply
// here, so the second factor is unconditional: there is no switch that
// turns it off.
//
// # Past RLS
//
// No Staff or Client policy admits a `feedback` row (00118). Once every
// refusal above has passed, this opens the one door that does
// (feedback.OpenReader, 00122) on the request's transaction, and hands
// that transaction to h through Tx. Nothing before that point can read a
// piece.
//
// h owns the commit. A handler that writes (the read record) commits
// before it answers, so a read is never served without its record; the
// rollback here is what ends a transaction a handler left open.
func FounderOnly(db *sql.DB, founderStaffID string, h http.Handler) http.Handler {
	founderStaffID = strings.ToLower(strings.TrimSpace(founderStaffID))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, uid, secondFactor, ok := authn.Begin(w, r, db, authn.TierStaff)
		if !ok {
			return
		}
		defer func() { _ = tx.Rollback() }()

		self, ok := requireSelf(w, r, tx, uid)
		if !ok {
			return
		}
		if founderStaffID == "" || self.ID != founderStaffID {
			apierr.WriteError(w, MsgFounderRouteNotFound, http.StatusNotFound)
			return
		}
		if !secondFactor {
			apierr.Write(w, http.StatusForbidden, apierr.CodeMFARequired, MsgFounderMFARequired, nil)
			return
		}

		if err := feedback.OpenReader(r.Context(), tx, self.ID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		ctx := context.WithValue(r.Context(), staffIDKey, self.ID)
		ctx = context.WithValue(ctx, txKey, tx)
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}

// MountFounderRoutes registers the founder read page's two routes
// (#1526). They are pre-Practice, in mountSessionRoutes' shape, and they
// are mounted apart from Mount only because they need configuration no
// other staffauth route does.
//
// Not rate limited, for the reason GET /api/staff/session is not: both
// are gated by authn.Begin's own session check, and then by FounderOnly.
func MountFounderRoutes(g *GatedRouter, db *sql.DB, cfg FounderConfig) {
	const reason = "no {practiceId}, so no Membership for a role declaration to be about -- FounderOnly gates it instead: one Staff id, and a second factor"
	g.OpenGet("/api/staff/feedback", reason,
		FounderOnly(db, cfg.StaffID, ListFeedbackHandler(cfg.FeedbackRepo)))
	g.OpenGet("/api/staff/feedback/{feedbackId}", reason,
		FounderOnly(db, cfg.StaffID, ReadFeedbackHandler(cfg.FeedbackRepo)))
}
