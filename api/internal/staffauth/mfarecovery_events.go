package staffauth

import (
	"context"
	"database/sql"
	"fmt"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/sessionnotice"
)

// AuthEventReason mirrors staff_auth_event_reason (00062) -- which of
// #615's recovery paths caused an enrollment removal. The third path,
// 'support', is a Platform operator's act and is written by doula-ops
// alone (#2483), so it has no constant here.
type AuthEventReason string

const (
	// AuthEventOwnerVouched is #605 §4.2.1.3: an Owner vouched, and the
	// locked-out person spent the issued code herself.
	AuthEventOwnerVouched AuthEventReason = "owner_vouched"
	// AuthEventSelfService is a sole Owner spending her own saved code.
	AuthEventSelfService AuthEventReason = "self_service"
	// AuthEventEnrolled is #606: a person enrolling her own TOTP factor,
	// outside any of the recovery paths above. Self-caused, like
	// AuthEventSelfService -- actor_staff_id equals staff_id.
	AuthEventEnrolled AuthEventReason = "enrolled"
	// AuthEventRemoved is #606: a person voluntarily removing her own
	// TOTP factor. Also self-caused; distinct from the recovery reasons
	// above because nobody else acted and nothing was lost -- she still
	// held the factor and chose to drop it.
	AuthEventRemoved AuthEventReason = "removed"
)

// clearEnrollmentAndRecord is where #615's Staff-side recovery paths
// converge: clear the identity's TOTP enrollment via the Admin SDK, end
// every live session, queue both notices, and write the
// staff_auth_events row -- all inside tx, so a failure partway through
// rolls every DB-side part back together. accounts.ClearSecondFactors is
// a real Admin SDK call outside tx, the same way reset.go's
// SpendResetHandler calls accounts.SetPassword before committing; it
// inherits that call's known risk (a commit failure after a successful
// external call would leave Identity Platform ahead of Postgres), which
// is the existing shape of every Admin SDK write in this codebase, not a
// new one.
//
// actorStaffID is the Staff member who caused the row, as
// staff_auth_events_actor_shape requires (self_service passes staffID
// for both -- she is her own actor).
func clearEnrollmentAndRecord(ctx context.Context, tx *sql.Tx, accounts authn.AccountManager, staffID, identityUID string, reason AuthEventReason, actorStaffID string) error {
	if err := accounts.ClearSecondFactors(ctx, identityUID); err != nil {
		return fmt.Errorf("staffauth: clear second factors: %w", err)
	}
	if err := endAllSessionsAndNotify(ctx, tx, identityUID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("staffauth: end sessions after recovery: %w", err)
	}
	if err := sessionnotice.QueueMFARecoveryCleared(ctx, tx, identityUID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("staffauth: queue mfa recovery cleared notice: %w", err)
	}
	if err := recordAuthEvent(ctx, tx, staffID, reason, actorStaffID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("staffauth: record auth event: %w", err)
	}
	return nil
}

// recordAuthEvent writes one staff_auth_events row (00062).
func recordAuthEvent(ctx context.Context, tx *sql.Tx, staffID string, reason AuthEventReason, actorStaffID string) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO staff_auth_events (staff_id, actor_staff_id, reason) VALUES ($1, $2, $3)`,
		staffID, actorStaffID, reason,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("staffauth: insert staff_auth_events: %w", err)
	}
	return nil
}
