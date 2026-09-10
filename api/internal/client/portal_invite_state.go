package client

import (
	"context"
	"database/sql"
	"fmt"
)

// PortalInviteState is one Client's portal-invite state -- the same
// facts ListItem carries (PortalInviteStatus, EmailSuppressed), plus
// whether the Client has an email address on file at all, fetched for
// exactly one Client rather than a page of them (#255). Any caller that
// needs one Client's portal state -- engagement.DetailHandler's Contract
// section readout, contracts.PostSendContractHandler's send precondition
// -- calls FetchPortalInviteState rather than re-deriving it, so a
// "never invited" Client reads the same way at both the Clients list and
// every other caller.
type PortalInviteState struct {
	// Status mirrors ListItem.PortalInviteStatus exactly: nil when the
	// Client has never been invited, "accepted" once she has, otherwise
	// the pending invite's own outbox status.
	Status *string
	// EmailSuppressed mirrors ListItem.EmailSuppressed: whether the
	// Client's address is currently suppressed (#785, ADR-0029).
	EmailSuppressed bool
	// HasEmail is whether the Client has an email address on file at
	// all -- distinct from EmailSuppressed, which only applies once
	// there is an address to suppress. A Client with no address cannot
	// be invited to the portal at all, regardless of Status.
	HasEmail bool
}

// portalUserForClient is the one client_portal_users row a Staff-facing
// read means when it says "her portal account", joined against a query
// whose Client is aliased c.
//
// Since #813 a merge can leave one Client reached by more than one
// Portal Account: two logins, each accepted against a different record
// before anybody noticed the two records were the same woman. ADR-0015's
// "at most one per Practice" is still true of each login -- neither
// reaches two Clients here -- but the reverse is now reachable, and a
// plain LEFT JOIN would list her twice on a page and pick arbitrarily on
// a single read.
//
// The lateral picks exactly one, deterministically, and prefers an
// accepted row over a pending invitation. "She can sign in" is the more
// informative answer of the two, and it is the one that keeps a second
// invitation refused rather than sent.
const portalUserForClient = `
		 LEFT JOIN LATERAL (
		     SELECT pu2.id, pu2.identity_uid FROM client_portal_users pu2
		      WHERE pu2.client_id = c.id
		      ORDER BY (pu2.identity_uid IS NOT NULL) DESC, pu2.id
		      LIMIT 1
		 ) pu ON true`

// FetchPortalInviteState reads clientID's current portal-invite state,
// the single-Client counterpart of the batched query ListHandler runs
// for a whole page. Returns sql.ErrNoRows (wrapped) if clientID does not
// exist -- callers are expected to have already confirmed the Client
// exists (an Engagement's own client_id, resolved by the caller's own
// scoped read), so that case is never the ordinary "never invited" one.
func FetchPortalInviteState(ctx context.Context, tx *sql.Tx, clientID string) (PortalInviteState, error) {
	var hasPortalUser, accepted, emailSuppressed, hasEmail bool
	var outboxStatus sql.NullString
	err := tx.QueryRowContext(ctx,
		`SELECT pu.id IS NOT NULL, pu.identity_uid IS NOT NULL, latest.status,
		        `+emailSuppressedExpr+`, c.email IS NOT NULL
		 FROM clients c`+portalUserForClient+`
		 LEFT JOIN LATERAL (
		     SELECT o.status FROM portal_invite_outbox o
		     WHERE o.client_portal_user_id = pu.id
		     ORDER BY o.created_at DESC LIMIT 1
		 ) latest ON true
		 WHERE c.id = $1`,
		clientID,
	).Scan(&hasPortalUser, &accepted, &outboxStatus, &emailSuppressed, &hasEmail)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return PortalInviteState{}, fmt.Errorf("client: fetch portal invite state: %w", err)
	}
	return PortalInviteState{
		Status:          PortalInviteStatus(hasPortalUser, accepted, outboxStatus),
		EmailSuppressed: emailSuppressed,
		HasEmail:        hasEmail,
	}, nil
}
