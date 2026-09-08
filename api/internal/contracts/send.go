package contracts

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/client"
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
// Contract is otherwise sendable if the Engagement's Client has never
// been sent a portal invite (#255) -- pending or accepted both satisfy
// it, since a pending Client can still accept and reach the Contract;
// only "never invited at all" is refused. That check runs before any
// write, so a refused Send leaves the Contract a draft, writes no
// activity entry, and sends no push -- identical to the not-a-draft
// refusal above. On success it notifies the Client's registered push
// subscription(s) with sendPushPayload via pusher, the #61 Pusher
// interface. Declared staffauth.AnyStaff at the mount (#282, #970):
// whoever reaches the Engagement at all -- Owner, Admin, an employed
// Doula, or a contractor on a granted attachment -- may send. Must be
// mounted through idempotency.Router.ExemptGated.
func PostSendContractHandler(pusher push.Pusher) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, engagementID, ok := resolveContractRequest(w, r)
		if !ok {
			return
		}

		id, prose, status, values, amountCents, err := fetchContract(r.Context(), tx, engagementID)
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, "no contract found for this engagement", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if ok, refusal := TransitionSend.Check(Status(status)); !ok {
			apierr.WriteError(w, refusal, http.StatusConflict)
			return
		}

		invited, err := clientHasPortalInvite(r.Context(), tx, engagementID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !invited {
			apierr.Write(w, http.StatusConflict, apierr.CodeFailedPrecondition, msgNoPortalInvite, nil)
			return
		}

		mergeFields := extractMergeFields(prose)
		values = withResolvedPrice(mergeFields, values, amountCents)
		if missing := missingMergeFieldKeys(mergeFields, values); len(missing) > 0 {
			details := make(map[string]string, len(missing))
			for _, key := range missing {
				details[key] = msgMergeFieldValueRequired
			}
			apierr.Write(w, http.StatusConflict, apierr.CodeFailedPrecondition, msgIncompleteMergeFields, details)
			return
		}

		if _, err := tx.ExecContext(r.Context(),
			`UPDATE contracts SET status = $1::contract_status WHERE id = $2`,
			string(StatusSent), id,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		practiceID, _ := staffauth.PracticeID(r.Context())
		staffID, _ := staffauth.StaffID(r.Context())
		if err := activity.Record(r.Context(), tx, activity.Entry{
			PracticeID:  practiceID,
			SubjectKind: activity.SubjectEngagement,
			SubjectID:   engagementID,
			Action:      string(activity.ActionContractSent),
			Actor:       activity.StaffActor(staffID),
		}); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		notifyClient(r.Context(), tx, pusher, engagementID)

		out := ContractResponse{
			EngagementID: engagementID,
			Status:       string(StatusSent),
			Prose:        prose,
			MergeFields:  mergeFields,
			Values:       values.nonEmpty(),
		}
		apierr.WriteJSON(w, http.StatusOK, out)
	})
}

// clientHasPortalInvite reports whether engagementID's Client has ever
// been sent a portal invite -- client.FetchPortalInviteState's Status
// being non-nil, the same "pending or accepted, never invited is the
// only refusal" rule the shared derivation already encodes (#255). Using
// that function rather than a bare EXISTS keeps this check and the
// Engagement page's own "never invited" readout answering from the one
// place, per the ticket's "do not fork it".
func clientHasPortalInvite(ctx context.Context, tx *sql.Tx, engagementID string) (bool, error) {
	var clientID string
	if err := tx.QueryRowContext(ctx,
		`SELECT client_id FROM engagements WHERE id = $1`,
		engagementID,
	).Scan(&clientID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests -- resolveContractRequest already proved the Engagement exists
		return false, fmt.Errorf("contracts: resolve engagement client: %w", err)
	}
	state, err := client.FetchPortalInviteState(ctx, tx, clientID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return false, fmt.Errorf("contracts: fetch client portal invite state: %w", err)
	}
	return state.Status != nil, nil
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
	body, err := json.Marshal(sendPushPayload{EngagementID: engagementID})
	// coverage:ignore reason: sendPushPayload always marshals cleanly, not exercised by unit tests
	if err != nil {
		// coverage:ignore reason: sendPushPayload always marshals cleanly, not exercised by unit tests
		log.Printf("contracts: notify client: marshal payload: %v", err)
		// coverage:ignore reason: sendPushPayload always marshals cleanly, not exercised by unit tests
		return
	}

	rows, err := tx.QueryContext(ctx,
		`SELECT endpoint, p256dh_key, auth_key FROM push_subscriptions_for_message_recipient($1, 'client')`,
		engagementID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		log.Printf("contracts: notify client: query subscriptions: %v", err)
		return
	}
	defer func() { _ = rows.Close() }()

	var subs []push.Subscription
	for rows.Next() {
		var s push.Subscription
		if err := rows.Scan(&s.Endpoint, &s.P256dhKey, &s.AuthKey); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			log.Printf("contracts: notify client: scan subscription row: %v", err)
			return
		}
		subs = append(subs, s)
	}
	// coverage:ignore reason: row iteration failure, not exercised by unit tests
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		log.Printf("contracts: notify client: iterate subscription rows: %v", err)
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return
	}

	for _, sub := range subs {
		if err := pusher.Send(ctx, sub, body); err != nil {
			log.Printf("contracts: notify client: send push: %v", err)
		}
	}
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
