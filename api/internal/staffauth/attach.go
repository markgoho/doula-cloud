package staffauth

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"

	"github.com/google/uuid"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/attachmentstate"
)

// AttachingWrite is ADR-0008's write-side seam. It wraps an
// Engagement-scoped write handler, refuses it up front if the caller may
// not reach the Engagement at all, and, once the handler has succeeded,
// attaches the *acting* Doula to the Engagement she just wrote under,
// origin 'accrued', attached_by her own staff id.
//
// It is a mount-time wrapper rather than a call each handler remembers to
// make, for the reason #231 gave: the per-request facts an attach
// decision needs are the same ones the read gate already has, so neither
// the refusal nor the attach should become a second hand-maintained list
// that a new write endpoint can silently fall off.
//
// The refusal reuses Reader.CanAccessEngagement -- the write table
// (ADR-0008) draws exactly the same line as the read table: an Owner,
// Admin, or employee Doula reaches every Engagement at the Practice, a
// contractor Doula reaches only one she holds an open, granted attachment
// on. A refused caller gets the same 404 a read gets, rather than a 403,
// for the reason CanAccessEngagement's own doc comment gives: a 403 would
// tell a contractor an Engagement exists that she cannot see, which is
// exactly the "doesn't exist" vs. "not attached" distinction the read gate
// exists to hide.
//
// Three rules the accrual half must never break (ADR-0008):
//
//   - Only the actor attaches. A Doula merely named in someone else's
//     payload -- an Admin scheduling her onto a Visit -- is not the actor
//     and does not accrue here; that is a granted attachment, written
//     explicitly by the Visit handlers and Offer accept via
//     attachmentstate.Grant.
//   - An Owner or Admin acting on an Engagement is never attached by it.
//   - The seam never mints 'granted'. Accrual is a record of work, never
//     a key; minting granted by accident is what would silently defeat
//     #227's no-backfill promise.
//
// A write that failed attaches nothing: the wrapped ResponseWriter
// records the status the handler wrote, and anything at or above 400
// skips the insert. The insert runs inside the request transaction
// staffauth.Middleware opened, before that Middleware commits, so an
// attachment and the write that caused it land together or not at all.
func AttachingWrite(next http.Handler) http.Handler {
	return Handler(func(w http.ResponseWriter, r *http.Request, c Call) {
		tx, staffID, reader := c.Tx, c.StaffID, c.Reader
		engagementID := r.PathValue("engagementId")

		// A malformed engagementId is left for the handler's own
		// staffauth.ParseUUID to reject with its usual 400 -- querying an
		// invalid UUID here would surface as a 500 instead, which is the
		// wrong failure for a caller who simply typo'd a path segment.
		if _, err := uuid.Parse(engagementID); err == nil {
			canAccess, err := reader.CanAccessEngagement(r.Context(), tx, engagementID)
			if err != nil {
				// coverage:ignore reason: DB query failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
			if !canAccess {
				apierr.WriteError(w, "engagement not found", http.StatusNotFound)
				return
			}
		}

		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		if recorder.status >= http.StatusBadRequest {
			return
		}

		if err := attachActor(r.Context(), tx, engagementID, staffID, reader); err != nil {
			// The response is already written, so this cannot become a 500.
			// Rolling the transaction back instead is what keeps the
			// database honest: Middleware's Commit then fails and the write
			// this attachment belonged to is discarded with it, rather than
			// landing without the attachment ADR-0008 says follows it.
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			log.Printf("staffauth: attach the acting doula to the request's engagement: %v", err)
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			_ = tx.Rollback()
		}
	})
}

// attachActor makes attachmentstate.TransitionAccrue for the acting
// Doula, or does nothing at all if the caller is not a plain Doula.
// reader is the caller's own Reader, already resolved by Middleware --
// attachActor makes no practice_memberships query of its own. Accrue
// leaving an open attachment alone is what makes the seam safe to run on
// every Engagement-scoped write: one already open for this pair --
// accrued from an earlier write, or granted by an Offer or a Visit --
// stays exactly as it is, so origin can only ever travel accrued ->
// granted.
func attachActor(ctx context.Context, tx *sql.Tx, engagementID, staffID string, reader Reader) error {
	if engagementID == "" || staffID == "" {
		// coverage:ignore reason: every route this wraps carries {engagementId} behind Middleware
		return nil
	}
	if !reader.Has("doula") || reader.IsOwnerOrAdmin() {
		return nil
	}

	if err := attachmentstate.Accrue(ctx, tx, engagementID, staffID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("staffauth: accrue attachment: %w", err)
	}
	return nil
}

// statusRecorder remembers the status code a handler wrote, so
// AttachingWrite can tell a write that happened from one that was
// refused. It records only -- every write goes straight through to the
// real ResponseWriter, so nothing is buffered and nothing is delayed.
type statusRecorder struct {
	http.ResponseWriter
	status  int
	written bool
}

func (s *statusRecorder) WriteHeader(status int) {
	if !s.written {
		s.status = status
		s.written = true
	}
	s.ResponseWriter.WriteHeader(status)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	s.written = true
	return s.ResponseWriter.Write(b) //nolint:wrapcheck // implements http.ResponseWriter; callers expect the raw net/http error, not a wrapped one
}
