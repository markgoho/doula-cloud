package client

import (
	"context"
	"database/sql"
	"fmt"

	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/portalinvite"
)

// PortalInviteState is one Client's portal-invite state -- the same
// facts ListItem carries (PortalInviteStatus, PortalInviteDelivery,
// EmailSuppressed), plus whether the Client has an email address on file
// at all, fetched for exactly one Client rather than a page of them
// (#255). Any caller that needs one Client's portal state --
// engagement.DetailHandler's Contract section readout,
// contracts.PostSendContractHandler's send precondition -- calls
// FetchPortalInviteState rather than re-deriving it, so a Client reads
// the same way at both the Clients list and every other caller.
type PortalInviteState struct {
	// Access is portalinvite.Derive's answer: nil when the Client has
	// never been invited.
	Access *portalinvite.Access
	// EmailSuppressed mirrors ListItem.EmailSuppressed: whether the
	// Client's address is currently suppressed (#785, ADR-0029).
	EmailSuppressed bool
	// HasEmail is whether the Client has an email address on file at
	// all -- distinct from EmailSuppressed, which only applies once
	// there is an address to suppress. A Client with no address cannot
	// be invited to the portal at all, whatever her Access says.
	HasEmail bool
}

// FetchPortalInviteState reads clientID's current portal-invite state,
// the single-Client counterpart of the batched query ListHandler runs
// for a whole page, through the same portalinvite.HerAccessJoin and
// portalinvite.Derive. Returns sql.ErrNoRows (wrapped) if clientID does
// not exist -- callers are expected to have already confirmed the Client
// exists (an Engagement's own client_id, resolved by the caller's own
// scoped read), so that case is never the ordinary "never invited" one.
func FetchPortalInviteState(ctx context.Context, tx *sql.Tx, clientID string) (PortalInviteState, error) {
	var stored, delivery sql.NullString
	var expiresAt sql.NullTime
	var emailSuppressed, hasEmail bool
	err := tx.QueryRowContext(ctx,
		`SELECT `+portalinvite.HerAccessColumns+`, `+emailSuppressedExpr+`, c.email IS NOT NULL
		 FROM clients c`+portalinvite.HerAccessJoin+`
		 WHERE c.id = $1`,
		clientID,
	).Scan(&stored, &expiresAt, &delivery, &emailSuppressed, &hasEmail)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return PortalInviteState{}, fmt.Errorf("client: fetch portal invite state: %w", err)
	}
	return PortalInviteState{
		Access:          portalinvite.Derive(stored, expiresAt, delivery, clock.Now(ctx)),
		EmailSuppressed: emailSuppressed,
		HasEmail:        hasEmail,
	}, nil
}
