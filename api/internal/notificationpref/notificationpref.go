// Package notificationpref is the Client-portal-side push-notification
// preference #303 builds: a durable, reviewable "is push on for this
// Engagement" setting, so a Client can turn it off (and back on) without
// signing out, and so api/internal/message's push path has something to
// check before it sends. Relies on clientauth.Middleware having already
// resolved the caller's Client/Engagement/identity and opened a
// request-scoped *sql.Tx.
//
// The wire contract is a single "enabled" boolean, not the raw "muted"
// column: the row itself is only ever created once she makes an explicit
// choice (00067_notification_preferences.sql), so "no row" and "muted"
// both read as enabled=false here -- the distinction that matters to the
// send-path filter (never registered vs. explicitly opted out) lives in
// whether a row exists at all, not in what this handler reports to the
// screen that only ever needs one bit.
package notificationpref

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clientauth"
)

// PreferenceResponse is the read/write shape both handlers below share.
type PreferenceResponse struct {
	Enabled bool `json:"enabled"`
}

// GetHandler reports whether push is currently on for the caller's own
// Engagement. Must be mounted behind clientauth.Middleware.
func GetHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, has := clientauth.Tx(r.Context())
		// coverage:ignore reason: clientauth.Middleware always sets a tx before this handler runs
		if !has {
			apierr.WriteError(w, clientauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		identityUID, _ := clientauth.IdentityUID(r.Context())
		engagementID, _ := clientauth.EngagementID(r.Context())

		enabled, err := readEnabled(r.Context(), tx, identityUID, engagementID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, clientauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		writeResponse(w, enabled)
	})
}

// SetRequest is the body of a PUT request to durably turn push on or off.
type SetRequest struct {
	Enabled bool `json:"enabled"`
}

// SetHandler durably turns push on or off for the caller's own Engagement,
// and records who did it, when, and to which value (CLAUDE.md's audit
// trail expectation). Must be mounted behind clientauth.Middleware.
func SetHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, has := clientauth.Tx(r.Context())
		// coverage:ignore reason: clientauth.Middleware always sets a tx before this handler runs
		if !has {
			apierr.WriteError(w, clientauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		identityUID, _ := clientauth.IdentityUID(r.Context())
		engagementID, _ := clientauth.EngagementID(r.Context())
		clientID, _ := clientauth.ClientID(r.Context())

		var req SetRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierr.WriteError(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if err := upsertPreference(r.Context(), tx, identityUID, engagementID, !req.Enabled); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, clientauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if err := recordPreferenceChange(r.Context(), tx, engagementID, clientID, req.Enabled); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, clientauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		writeResponse(w, req.Enabled)
	})
}

// writeResponse encodes a PreferenceResponse, shared by both handlers.
func writeResponse(w http.ResponseWriter, enabled bool) {
	w.Header().Set("Content-Type", "application/json")
	// coverage:ignore reason: response encoding failure, not exercised by unit tests
	if err := json.NewEncoder(w).Encode(PreferenceResponse{Enabled: enabled}); err != nil {
		apierr.WriteError(w, clientauth.MsgInternalError, http.StatusInternalServerError)
	}
}

// readEnabled reads identityUID/engagementID's push preference. No row at
// all -- she has never made a choice -- reports enabled=false, the same as
// an explicit mute: #303 AC1 requires the explanation screen to run before
// the very first browser permission prompt, so the client-side register
// helper (registerPushSubscriptionIfEnabled) must never treat "never
// decided" as "on".
func readEnabled(ctx context.Context, tx *sql.Tx, identityUID, engagementID string) (bool, error) {
	var muted bool
	err := tx.QueryRowContext(ctx,
		`SELECT muted FROM notification_preferences WHERE identity_uid = $1 AND engagement_id = $2 AND channel = 'push'`,
		identityUID, engagementID,
	).Scan(&muted)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return false, fmt.Errorf("notificationpref: read preference: %w", err)
	}
	return !muted, nil
}

// upsertPreference durably records identityUID/engagementID's push
// preference, creating the row the first time she ever makes a choice or
// updating it in place on every later one.
func upsertPreference(ctx context.Context, tx *sql.Tx, identityUID, engagementID string, muted bool) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO notification_preferences (identity_uid, engagement_id, channel, muted, updated_at)
		 VALUES ($1, $2, 'push', $3, now())
		 ON CONFLICT (identity_uid, engagement_id, channel) DO UPDATE SET
		     muted = excluded.muted,
		     updated_at = excluded.updated_at`,
		identityUID, engagementID, muted,
	)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return fmt.Errorf("notificationpref: upsert preference: %w", err)
	}
	return nil
}

// recordPreferenceChange writes #303's Activity row for the Client's own
// choice -- actor_kind 'client', engagementID as subject_id, mirroring
// contracts/sign.go's recordContractSigned: this handler runs behind
// clientauth.Middleware, which sets only app.current_client_id, never
// app.current_practice_id -- activity's RLS policy needs the latter, so
// activity.ScopeToPractice sets it here, immediately before the one Record
// call that needs it.
func recordPreferenceChange(ctx context.Context, tx *sql.Tx, engagementID, clientID string, enabled bool) error {
	var practiceID string
	if err := tx.QueryRowContext(ctx,
		`SELECT practice_id FROM engagements WHERE id = $1`, engagementID,
	).Scan(&practiceID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests -- clientauth.Middleware already confirmed this row exists
		return fmt.Errorf("notificationpref: resolve engagement for preference change: %w", err)
	}
	if err := activity.ScopeToPractice(ctx, tx, practiceID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("notificationpref: scope to practice for preference change: %w", err)
	}

	action := activity.ActionPushNotificationsDisabled
	if enabled {
		action = activity.ActionPushNotificationsEnabled
	}
	diff, err := json.Marshal(struct {
		Enabled bool `json:"enabled"`
	}{Enabled: enabled})
	// coverage:ignore reason: the literal struct above always marshals cleanly, not exercised by unit tests
	if err != nil {
		// coverage:ignore reason: the literal struct above always marshals cleanly, not exercised by unit tests
		return fmt.Errorf("notificationpref: marshal preference diff: %w", err)
	}

	if err := activity.Record(ctx, tx, activity.Entry{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectEngagement,
		SubjectID:   engagementID,
		Action:      string(action),
		Diff:        diff,
		Actor:       activity.ClientActor(clientID),
	}); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("notificationpref: record preference change: %w", err)
	}
	return nil
}
