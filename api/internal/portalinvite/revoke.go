package portalinvite

import (
	"context"
	"database/sql"
	"fmt"
)

// RevokePending dead-letters any pending portal_invite_outbox row for
// clientID's client_portal_users row, so it never sends. Called by the
// client package inside the same transaction as an edit that changes
// clients.email (ADR-0017): ProcessPending reads the Client's address
// live at send (outbox.go:69), so a pending invite left in place would
// otherwise mail a live token to whatever address was just typed, with
// nobody having confirmed it belongs to the same recipient. A no-op if
// there is no pending row (no portal invite was ever sent, or the last
// one already sent/dead-lettered).
func RevokePending(ctx context.Context, tx *sql.Tx, clientID string) error {
	if _, err := tx.ExecContext(ctx,
		`UPDATE portal_invite_outbox o
		 SET status = 'dead_lettered', last_error = 'revoked: client email changed'
		 FROM client_portal_users pu
		 WHERE o.client_portal_user_id = pu.id
		   AND pu.client_id = $1
		   AND o.status = 'pending'`,
		clientID,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("portalinvite: revoke pending invite: %w", err)
	}
	return nil
}
