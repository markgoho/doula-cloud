package contracts

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/clientstanding"
	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/push"
	"doula-cloud/api/internal/staffauth"
)

// msgIncompleteMergeFields is what a Staff member sees when Send is
// refused because the Contract still has a merge field with no value
// (#258): a blank in a legal document is exactly the defect this ticket
// exists for, so every key parsed out of the prose is required -- there
// is no optional merge field. Details carries every missing key at once
// (docs/api-design.md section 7), not only the first, keyed by
// msgMergeFieldValueRequired so a caller can list them without a second
// round trip.
const msgIncompleteMergeFields = "every merge field must have a value before this contract can be sent"

// msgMergeFieldValueRequired is the per-key Details message
// msgIncompleteMergeFields carries for each missing merge field key.
const msgMergeFieldValueRequired = "enter a value for this field"

// msgNoPortalInvite is what a Staff member sees when Send is refused
// because the Engagement's Client has never been sent a portal invite
// (#255): a Contract that moves to 'sent' is reachable only through the
// Client portal, and the portal is reachable only by accepting an
// invite, so sending one to a Client with none would produce a Contract
// nothing could move out of 'sent'. Named as the reason directly, per
// the ticket's "a reason a person can act on" -- the Engagement page's
// own "Send portal invite" action is the way out.
const msgNoPortalInvite = "this client has not been invited to the portal yet"

// msgPortalInviteRefused refuses a send while the Client's accept was
// refused (#2522): she signs in through another record, so nothing sent
// from this one reaches her. It names no other record.
const msgPortalInviteRefused = "This Client signs in through another record, so cannot sign a Contract sent from this one. Combine the two records: save this Client's details, and you will be asked whether the other record is the same person."

// The refusals for an invitation that exists but cannot bring her to
// the Contract: each says what happened to the invite and what to do,
// in the order a person would say it (docs/api-design.md section 7
// rule 4, #1884).
const (
	msgPortalInviteExpired      = "This Client's portal invite has expired, so you cannot send this Contract yet. Send a new portal invite first."
	msgPortalInviteWithdrawn    = "This Client's portal invite was withdrawn, so you cannot send this Contract yet. Send a new portal invite first."
	msgPortalInviteNotDelivered = "This Client's portal invite did not arrive, so you cannot send this Contract yet. Send a new portal invite first."
	msgPortalAccessRemoved      = "This Client no longer has portal access, so you cannot send this Contract."
)

// sendPushPayload is the content-free payload delivered to the Client's
// push subscription(s) on Send -- per the ticket's "no Contract content
// leaking through the notification channel" rule, it carries only the
// Engagement id, the same shape message.pushPayload uses for a Client
// recipient (no practiceId, since the Client-portal thread route needs
// only the Engagement id).
type sendPushPayload struct {
	EngagementID string `json:"engagementId"`
}

// PostSendContractHandler transitions the Contract for :engagementId from
// 'draft' to 'sent' -- the only transition it permits; any other current
// status 409s. It also refuses (409, failed-precondition) once the
// Contract is otherwise sendable unless the Engagement's Client can
// reach the portal (#255, portalinvite.Access.CanReachPortal): accepted,
// or holding an invitation she can still accept whose send did not
// fail. Never invited, refused, expired, withdrawn, unlinked and an
// undelivered invite are each refused in their own words (portalRefusal).
// That check runs before any
// write, so a refused Send leaves the Contract a draft, writes no
// activity entry, and sends no push -- identical to the not-a-draft
// refusal above. On success it notifies the Client's registered push
// subscription(s) with sendPushPayload via pusher, the #61 Pusher
// interface. Declared staffauth.AnyStaff at the mount (#282, #970):
// whoever reaches the Engagement at all -- Owner, Admin, an employed
// Doula, or a contractor on a granted attachment -- may send. Must be
// mounted through idempotency.Router.ExemptGated.
func PostSendContractHandler(pusher push.Pusher) http.Handler {
	return contractHandler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call, ct contractRow) {
		if ok, refusal := TransitionSend.Check(Status(ct.Status)); !ok {
			apierr.WriteError(w, refusal, http.StatusConflict)
			return
		}
		// Ahead of the portal-invite and merge-field refusals below,
		// which an erased record would also meet (its link is removed and
		// its Contracts' merge values are emptied) but for the wrong
		// reason.
		if !clientstanding.ActSendContract.RequireForEngagement(w, r, c.Tx, c.EngagementID) {
			return
		}

		access, err := clientPortalAccess(r.Context(), c.Tx, c.EngagementID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !access.CanReachPortal() {
			apierr.Write(w, http.StatusConflict, apierr.CodeFailedPrecondition, portalRefusal(access), nil)
			return
		}

		mergeFields := extractMergeFields(ct.Prose)
		values := withResolvedPrice(mergeFields, ct.Values, ct.AmountCents)
		if missing := missingMergeFieldKeys(mergeFields, values); len(missing) > 0 {
			details := make(map[string]string, len(missing))
			for _, key := range missing {
				details[key] = msgMergeFieldValueRequired
			}
			apierr.Write(w, http.StatusConflict, apierr.CodeFailedPrecondition, msgIncompleteMergeFields, details)
			return
		}

		if _, err := c.Tx.ExecContext(r.Context(),
			`UPDATE contracts SET status = $1::contract_status WHERE id = $2`,
			string(StatusSent), ct.ID,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if err := recordEngagementActivity(r.Context(), c, activity.ActionContractSent, nil); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		notifyClient(r.Context(), c.Tx, pusher, c.EngagementID)

		out := ContractResponse{
			EngagementID:    c.EngagementID,
			Status:          string(StatusSent),
			Prose:           ct.Prose,
			MergeFields:     mergeFields,
			Values:          values.nonEmpty(),
			AmountChangedAt: ct.AmountChangedAt,
		}
		writeContract(w, r, c, http.StatusOK, out)
	}, staffauth.Engagement())
}

// clientPortalAccess reads engagementID's Client's portal access through
// client.FetchPortalInviteState -- the derivation the Engagement page's
// own readout uses, so the send and the screen answer from one place
// (#255's "do not fork it"). nil means she was never invited.
func clientPortalAccess(ctx context.Context, tx *sql.Tx, engagementID string) (*portalinvite.Access, error) {
	var clientID string
	if err := tx.QueryRowContext(ctx,
		`SELECT client_id FROM engagements WHERE id = $1`,
		engagementID,
	).Scan(&clientID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests -- staffauth.Engagement already proved the Engagement exists
		return nil, fmt.Errorf("contracts: resolve engagement client: %w", err)
	}
	state, err := client.FetchPortalInviteState(ctx, tx, clientID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("contracts: fetch client portal invite state: %w", err)
	}
	return state.Access, nil
}

// portalRefusal is the sentence a send is refused with when access
// cannot bring her to the Contract (portalinvite.Access.CanReachPortal
// false). A Contract that moves to 'sent' is reachable only through the
// portal, so an invitation that has expired, was withdrawn, was refused
// or never arrived would leave it sent to nobody.
func portalRefusal(access *portalinvite.Access) string {
	if access == nil {
		return msgNoPortalInvite
	}
	switch access.State {
	case portalinvite.StateRefused:
		return msgPortalInviteRefused
	case portalinvite.StateExpired:
		return msgPortalInviteExpired
	case portalinvite.StateRevoked:
		return msgPortalInviteWithdrawn
	case portalinvite.StateUnlinked:
		return msgPortalAccessRemoved
	}
	return msgPortalInviteNotDelivered
}

// notifyClient sends sendPushPayload to every push subscription
// registered by engagementID's Client, resolved via
// push_subscriptions_for_message_recipient (00010_push_subscriptions_message_recipient.sql)
// the same way message.notifyRecipient does -- Send's recipient is always
// the Client (the Staff member sending it already knows), so this needs
// neither a senderType parameter nor the Practice-id branch
// message.pushPayload carries for a Staff recipient. Failures are logged
// and swallowed: the Contract's status has already been written to tx by
// the time this runs, and push delivery is a best-effort notification
// layer on top of it, not part of Send's own success criteria.
func notifyClient(ctx context.Context, tx *sql.Tx, pusher push.Pusher, engagementID string) {
	push.NotifyMessageRecipient(ctx, tx, pusher, "contracts: notify client", engagementID, "client", sendPushPayload{EngagementID: engagementID})
}

// missingMergeFieldKeys reports every key in mergeFields whose entry in
// values is absent, empty, or whitespace-only, in prose order -- #258's
// Send precondition. Every parsed key is required; there is no optional
// merge field, so a key with no entry at all in values counts as missing
// the same as one mapped to "" or "   ".
func missingMergeFieldKeys(mergeFields []string, values MergeFieldValues) []string {
	var missing []string
	for _, key := range mergeFields {
		if strings.TrimSpace(values[key]) == "" {
			missing = append(missing, key)
		}
	}
	return missing
}
