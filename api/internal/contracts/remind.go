package contracts

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/contractnotice"
	"doula-cloud/api/internal/portalinvite"
	"doula-cloud/api/internal/practicetimezone"
	"doula-cloud/api/internal/push"
	"doula-cloud/api/internal/staffauth"
)

// msgRemindedToday is what a Staff member sees when a reminder is refused
// because the Client was already emailed about a Contract today (#2951):
// by a reminder, or by the Contract being sent.
const msgRemindedToday = "This Client was already emailed about a Contract today. You can send a reminder tomorrow."

// The refusals for a Client the reminder could not reach, one for each way
// she cannot be brought to the Contract (portalRefusal's states, said for a
// reminder rather than a send).
const (
	msgRemindNoInvite      = "This Client has not been invited to the portal yet, so a reminder has nowhere to go."
	msgRemindRefused       = "This Client signs in through another record, so a reminder from this one has nowhere to go."
	msgRemindExpired       = "This Client's portal invite has expired, so a reminder has nowhere to go. Send a new portal invite first."
	msgRemindWithdrawn     = "This Client's portal invite was withdrawn, so a reminder has nowhere to go. Send a new portal invite first."
	msgRemindNotDelivered  = "This Client's portal invite did not arrive, so a reminder has nowhere to go. Send a new portal invite first."
	msgRemindAccessRemoved = "This Client no longer has portal access, so a reminder has nowhere to go."
)

// PostRemindContractHandler emails the Client of engagement :engagementId's
// Sent Contract a reminder, and pushes to her subscriptions the way a send
// does (#2951). It leaves the Contract Sent and records contract_reminded
// against the Engagement with the Staff member who asked, the act. Whether
// the mail arrived is the outbox row's, which the Engagement reads back
// (#2978).
//
// A Client is emailed about a Contract at most once in a Practice day (the
// day the list reads, in the Practice's zone): a second reminder, or one on
// the day the Contract was sent, is refused 409 and writes nothing. Mail
// that dead-lettered never reached her and does not hold the day. The day
// is counted across all of her Contracts, and the check and the write sit
// behind one lock per Client, so two Staff pressing at once cannot both
// pass. Refused first, before any write: a Contract that is not Sent (which
// an erased Client's never is, #2990), a Client merged away, and a Client who
// cannot reach the portal.
// Declared staffauth.AnyStaff at the mount, as send is, and mounted
// through idempotency.Router.ReplayableGated, since a replay must not mail
// her again.
func PostRemindContractHandler(pusher push.Pusher) http.Handler {
	return contractHandler(func(w http.ResponseWriter, r *http.Request, c staffauth.Call, ct contractRow) {
		if ok, refusal := TransitionRemind.Check(Status(ct.Status)); !ok {
			apierr.WriteError(w, refusal, http.StatusConflict)
			return
		}
		rule, err := remindRuleFor(r.Context(), c)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !rule.Can {
			apierr.Write(w, http.StatusConflict, apierr.CodeFailedPrecondition, rule.Refusal, nil)
			return
		}

		ctx := r.Context()
		if err := recordEngagementActivity(ctx, c, activity.ActionContractReminded, nil); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if err := contractnotice.Queue(ctx, c.Tx, c.PracticeID, ct.ID, contractnotice.KindReminder, c.StaffID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		notifyClient(ctx, c.Tx, pusher, c.EngagementID)

		mergeFields := extractMergeFields(ct.Prose)
		out := ContractResponse{
			EngagementID:    c.EngagementID,
			Status:          ct.Status,
			Prose:           ct.Prose,
			MergeFields:     mergeFields,
			Values:          withResolvedPrice(mergeFields, ct.Values, ct.AmountCents).nonEmpty(),
			AmountChangedAt: ct.AmountChangedAt,
		}
		writeContract(w, r, c, http.StatusOK, out)
	}, staffauth.Engagement())
}

// remindRefusal is the sentence a reminder is refused with when access
// cannot bring her to the Contract.
func remindRefusal(access *portalinvite.Access) string {
	if access == nil {
		return msgRemindNoInvite
	}
	switch access.State {
	case portalinvite.StateRefused:
		return msgRemindRefused
	case portalinvite.StateExpired:
		return msgRemindExpired
	case portalinvite.StateRevoked:
		return msgRemindWithdrawn
	case portalinvite.StateUnlinked:
		return msgRemindAccessRemoved
	case portalinvite.StateInvited, portalinvite.StateAccepted:
		// An invitation she has not lost: the reminder was refused because
		// it never arrived, which the sentence below says.
	}
	return msgRemindNotDelivered
}

// remindRuleFor is the rule for this Contract's reminder. It first takes a
// transaction-scoped advisory lock keyed on the Client, so a second request
// for her waits for this one to commit and then sees its queued mail.
func remindRuleFor(ctx context.Context, c staffauth.Call) (remindRule, error) {
	zone, err := practicetimezone.Load(ctx, c.Tx, c.PracticeID)
	if err != nil {
		return remindRule{}, fmt.Errorf("contracts: load practice zone: %w", err)
	}
	t, err := sentTarget(ctx, c.Tx, c.EngagementID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return remindRule{}, err
	}
	if _, err := c.Tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "contract-remind:"+t.ClientID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return remindRule{}, fmt.Errorf("contracts: lock client for reminder: %w", err)
	}
	rules, err := remindRules(ctx, c.Tx, []remindTarget{t}, zone)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return remindRule{}, err
	}
	return rules[t.ContractID], nil
}

// startOfDay is midnight of now's calendar day in zone.
func startOfDay(now time.Time, zone *time.Location) time.Time {
	year, month, day := now.In(zone).Date()
	return time.Date(year, month, day, 0, 0, 0, 0, zone)
}
