package portalinvite

import (
	"context"
	"database/sql"
	"fmt"
)

// RevokeReason names why a pending invitation was stopped, written onto
// the dead-lettered outbox row. It is the only account anyone gets of
// why an invitation a Practice sent never arrived, so a caller states
// its own reason rather than inheriting another act's.
type RevokeReason string

const (
	// RevokedEmailChanged is an edit that changed clients.email.
	RevokedEmailChanged RevokeReason = "revoked: client email changed"
	// RevokedRecordMerged is a merge absorbing the record the invitation
	// named (#813). The invitation is not moved to the survivor -- it is
	// re-sendable, and the address it was sent to may not even be the
	// address the fold kept -- so it is stopped here and says so.
	RevokedRecordMerged RevokeReason = "revoked: client record merged into another"
)

// RevokePending dead-letters any pending portal_invite_outbox row for
// clientID's client_portal_users row, so it never sends. Called by the
// client package inside the same transaction as an edit that changes
// clients.email (ADR-0017): ProcessPending reads the Client's address
// live at send (outbox.go:69), so a pending invite left in place would
// otherwise mail a live token to whatever address was just typed, with
// nobody having confirmed it belongs to the same recipient. A merge
// calls it for the same reason about the record it absorbs. A no-op if
// there is no pending row (no portal invite was ever sent, or the last
// one already sent/dead-lettered).
func RevokePending(ctx context.Context, tx *sql.Tx, clientID string, reason RevokeReason) error {
	if _, err := tx.ExecContext(ctx,
		`UPDATE portal_invite_outbox o
		 SET status = 'dead_lettered', last_error = $2
		 FROM client_portal_users pu
		 WHERE o.client_portal_user_id = pu.id
		   AND pu.client_id = $1
		   AND o.status = 'pending'`,
		clientID, string(reason),
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("portalinvite: revoke pending invite: %w", err)
	}
	return nil
}
