package staffauth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/sessionnotice"
	"doula-cloud/api/internal/tasknudge"
)

// EndSessionsHandler lets a Practice Owner end every session a Staff
// member holds, on every device -- offboarding, or a lost phone. This is
// deliberately not what ordinary sign-out does: sign-out ends only the
// browser making the request, this ends all of them (#154). Must be
// mounted behind staffauth.Middleware, which is what makes the 403s for
// a non-Owner or an Owner at a different Practice automatic: the caller
// must already hold a membership at :practiceId to reach RequireOwner at
// all. enq is ADR-0013's Cloud Tasks nudge for the session-notice outbox
// row QueueSessionRevoked queues below -- registered rather than fired
// directly, same reasoning as portalinvite.InviteHandler.
func EndSessionsHandler(enq tasknudge.Enqueuer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := RequireOwner(w, r)
		if !ok {
			return
		}
		if !RequireConfirmed(w, r) {
			return
		}
		actorStaffID, _ := StaffID(r.Context())
		targetStaffID := r.PathValue("staffId")

		var identityUID string
		err := tx.QueryRowContext(r.Context(),
			`SELECT s.identity_uid FROM staff s
			 JOIN practice_memberships pm ON pm.staff_id = s.id
			 WHERE pm.practice_id = $1 AND pm.staff_id = $2`,
			practiceID, targetStaffID,
		).Scan(&identityUID)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "no membership found for that staff member at this practice", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := endAllSessionsAndNotify(r.Context(), tx, identityUID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		// #473: this handler recorded nothing before -- who ended a Staff
		// member's sessions, and when, existed nowhere. Same subject kind
		// as a membership edit or removal, since it is the same
		// relationship the row names.
		if err := activity.Record(r.Context(), tx, activity.Entry{
			PracticeID:  practiceID,
			SubjectKind: "membership",
			SubjectID:   targetStaffID,
			Action:      "sessions_ended",
			Diff:        json.RawMessage("{}"),
			Actor:       activity.StaffActor(actorStaffID),
		}); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		// The target Staff member should hear that this happened (#345),
		// regardless of who took the action -- there is no self-service
		// "sign out everywhere" yet, so today that is always this Owner,
		// not the target.
		if err := sessionnotice.QueueSessionRevoked(r.Context(), tx, identityUID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		tasknudge.Register(r.Context(), tasknudge.Fire(enq, tasknudge.SessionNotice))

		w.WriteHeader(http.StatusNoContent)
	})
}

// endAllSessionsAndNotify ends every session identityUID holds and
// queues #345's session-notice mail -- the part of EndSessionsHandler
// above that #613's password reset also needs (a successful reset ends
// every existing session for the identity, same as an Owner's explicit
// "end sessions everywhere"). It deliberately does not record an
// activity row: that requires a PracticeID, and reset runs before any
// Practice is known -- unlike EndSessionsHandler, which is
// Owner-initiated and does record one.
func endAllSessionsAndNotify(ctx context.Context, tx *sql.Tx, identityUID string) error {
	if err := authn.EndAllSessions(ctx, tx, identityUID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("staffauth: end all sessions: %w", err)
	}
	if err := sessionnotice.QueueSessionRevoked(ctx, tx, identityUID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("staffauth: queue session revoked notice: %w", err)
	}
	return nil
}
