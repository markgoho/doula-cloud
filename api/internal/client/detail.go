package client

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clientkey"
	"doula-cloud/api/internal/staffauth"
)

// Event is one activity row for this Client (subject_kind 'client',
// ADR-0022): what changed, when, and who did it -- append-only, so this
// is always the row exactly as written. ActorName is the Staff or Client
// actor's display name, joined in for the history screen -- the
// cross-cutting audit expectation ("who did this") needs a name, not a
// bare id a Doula has no way to resolve herself (/staff is
// Owner/Admin-only). Nil when ActorKind is "system" -- ADR-0022's third
// actor kind displays as "Doula Cloud", a client-side rendering rule
// rather than a name this DTO carries.
type Event struct {
	EventType    string          `json:"eventType"`
	Diff         json.RawMessage `json:"diff"`
	ActorKind    string          `json:"actorKind"`
	ActorStaffID *string         `json:"actorStaffId,omitempty"`
	ActorName    *string         `json:"actorName,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
}

// RequestSummary is one engagement_requests row -- the audit record for
// "this Engagement began because she asked and he agreed" (ADR-0017),
// merged into the same history as her activity rows rather than
// mirrored into one. RequestedByName/DecidedByName are joined in for the
// same reason Event.ActorName is: the pending-request block must name
// who asked, for every Staff role that can read this page, not just the
// Owner/Admin roles that may list /staff.
type RequestSummary struct {
	RequestID       string     `json:"requestId"`
	Kind            string     `json:"kind"`
	State           string     `json:"state"`
	RequestedBy     string     `json:"requestedBy"`
	RequestedByName string     `json:"requestedByName"`
	RequestedAt     time.Time  `json:"requestedAt"`
	DecidedBy       *string    `json:"decidedBy,omitempty"`
	DecidedByName   *string    `json:"decidedByName,omitempty"`
	DecidedAt       *time.Time `json:"decidedAt,omitempty"`
	Reason          *string    `json:"reason,omitempty"`
	EngagementID    *string    `json:"engagementId,omitempty"`
}

// HistoryEntry is one row of a Client's merged history -- her
// activity rows and her engagement_requests rows, interleaved by
// time, newest first. Exactly one of ClientEvent/EngagementRequest is
// set, named by Type.
type HistoryEntry struct {
	Type              string          `json:"type"`
	At                time.Time       `json:"at"`
	ClientEvent       *Event          `json:"clientEvent,omitempty"`
	EngagementRequest *RequestSummary `json:"engagementRequest,omitempty"`
}

// DetailResponse is a Client's full detail read: her record (both
// layers), her Practice-defined values resolved against the current
// Client Field Template, her Engagements past and present, and her
// merged history.
type DetailResponse struct {
	Record
	ResolvedFields []ResolvedField     `json:"resolvedFields"`
	Engagements    []EngagementSummary `json:"engagements"`
	History        []HistoryEntry      `json:"history"`
	// ErasedAt is when this Client's data was erased on her own request
	// (ADR-0027), absent for every Client who has not asked. It is what a
	// screen reads to explain why her record shows a placeholder instead
	// of a name, and why editing her is refused.
	ErasedAt *time.Time `json:"erasedAt,omitempty"`
	// StripeRedactionEligibleAt is the date Stripe will first allow her
	// transactions to be redacted -- 90 days past the newest invoice on
	// whichever of her Stripe Customers is furthest from eligible.
	// #394's acceptance criterion asks for this state to be visible to
	// the Practice rather than implied, so it is present for as long as
	// the redaction has not succeeded -- including after it has failed,
	// where the date may already be in the past. Absent means done: it
	// ran, or she had no Stripe presence to redact.
	StripeRedactionEligibleAt *time.Time `json:"stripeRedactionEligibleAt,omitempty"`
}

// DetailHandler views one Client's full record. Access follows
// Reader.CanAccessClient, the same rule EditHandler uses -- ADR-0017's
// "edit follows read" is stated the other way round here: read is the
// wider of the two acts, so whatever may edit may certainly read. Must be
// mounted behind staffauth.Middleware.
func DetailHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}
		clientID := r.PathValue("clientId")
		if !staffauth.ParseUUID(w, "client", clientID) {
			return
		}

		staffID, _ := staffauth.StaffID(r.Context())
		reader, err := staffauth.ResolveReader(r.Context(), tx, practiceID, staffID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		canAccess, err := reader.CanAccessClient(r.Context(), tx, clientID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !canAccess {
			apierr.WriteError(w, "client not found", http.StatusNotFound)
			return
		}

		rec, err := fetchRecord(r.Context(), tx, practiceID, clientID)
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, "client not found", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		engagements, err := ListEngagementsForClient(r.Context(), tx, clientID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		history, err := mergedHistory(r.Context(), tx, clientID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		resolvedFields, err := resolveFields(r.Context(), tx, practiceID, rec.FieldValues)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		erasedAt, redactionEligibleAt, err := readErasureState(r.Context(), tx, clientID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		out := DetailResponse{
			Record:                    rec,
			ResolvedFields:            resolvedFields,
			Engagements:               engagements,
			History:                   history,
			ErasedAt:                  erasedAt,
			StripeRedactionEligibleAt: redactionEligibleAt,
		}
		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(out); err != nil {
			apierr.WriteError(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}

// mergedHistory reads clientID's activity and engagement_requests
// rows and interleaves them newest-first by time -- a read concern, not
// a storage one (ADR-0017 deliberately keeps no mirrored row on either
// side).
func mergedHistory(ctx context.Context, tx *sql.Tx, clientID string) ([]HistoryEntry, error) {
	events, err := listClientEvents(ctx, tx, clientID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, err
	}
	requests, err := listRequestsForClient(ctx, tx, clientID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, err
	}

	history := make([]HistoryEntry, 0, len(events)+len(requests))
	for i := range events {
		history = append(history, HistoryEntry{Type: "client_event", At: events[i].CreatedAt, ClientEvent: &events[i]})
	}
	for i := range requests {
		history = append(history, HistoryEntry{Type: "engagement_request", At: requests[i].RequestedAt, EngagementRequest: &requests[i]})
	}
	sort.Slice(history, func(i, j int) bool { return history[i].At.After(history[j].At) })
	return history, nil
}

func listClientEvents(ctx context.Context, tx *sql.Tx, clientID string) ([]Event, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT a.action, a.diff, a.actor_kind::text, a.actor_staff_id, s.name,
		        ac.given_name, ac.preferred_name, a.created_at
		 FROM activity a
		 LEFT JOIN staff s ON s.id = a.actor_staff_id
		 LEFT JOIN clients ac ON ac.id = a.actor_client_id
		 WHERE a.subject_kind = 'client' AND a.subject_id = $1 ORDER BY a.created_at`,
		clientID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("client: list client events: %w", err)
	}
	defer func() { _ = rows.Close() }()

	events := []Event{}
	for rows.Next() {
		var e Event
		var actorStaffID, actorName, actorClientGivenName, actorClientPreferredName sql.NullString
		var diff []byte
		if err := rows.Scan(&e.EventType, &diff, &e.ActorKind, &actorStaffID, &actorName,
			&actorClientGivenName, &actorClientPreferredName, &e.CreatedAt); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("client: scan client event: %w", err)
		}
		e.Diff = diff
		if actorStaffID.Valid {
			e.ActorStaffID = &actorStaffID.String
		}
		if actorName.Valid {
			e.ActorName = &actorName.String
		}
		// The Client-authored case ADR-0022's actor_kind='client' names:
		// #619's sign-in-address change is written by
		// clientauth.recordAddressChange with a ClientActor, so this row
		// carries her own name rather than a Staff member's.
		if actorClientGivenName.Valid {
			name := PreferredName(actorClientGivenName.String, actorClientPreferredName.String)
			e.ActorName = &name
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("client: iterate client events: %w", err)
	}
	if err := rows.Close(); err != nil {
		// coverage:ignore reason: row close failure, not exercised by unit tests
		return nil, fmt.Errorf("client: close client events: %w", err)
	}

	// Unsealing runs after the rows are closed, never inside the loop:
	// opening a diff reads her key, and a second query on the same
	// transaction while a result set is still open deadlocks on the
	// connection.
	for i := range events {
		events[i].Diff = openDiff(ctx, tx, clientID, events[i].Diff)
	}
	return events, nil
}

// readErasureState reads the two dates the detail screen needs to
// explain an erased Client: when she was erased, and -- while any part
// of the Stripe redaction has not succeeded -- when Stripe will first
// allow it.
//
// Three things about that second date, each of which was got wrong once:
//
//   - It reads redactable_after, not next_attempt_at. next_attempt_at is
//     scheduling and a retry rewrites it (outbox.MarkFailed), so showing
//     it turns "redactable after March" into a five-minute retry stamp.
//     redactable_after is written once at enqueue time and never moves.
//   - It includes dead-lettered rows, not only pending ones. A redaction
//     that failed is precisely what a Practice must still be able to
//     see; filtering it out makes a failure look like a completion.
//     Stripe's Redaction Jobs API is not enabled on this account
//     (ADR-0027), so today that is the normal path, not the rare one.
//   - It takes the maximum across her Customers, because each has its
//     own eligibility date and the Stripe half is done when the last of
//     them is.
//
// Absent means done: every redaction row succeeded, or she has no Stripe
// presence at all.
func readErasureState(ctx context.Context, tx *sql.Tx, clientID string) (erasedAt, redactionEligibleAt *time.Time, err error) {
	var erased, eligible sql.NullTime
	err = tx.QueryRowContext(ctx,
		`SELECT c.erased_at,
		        (SELECT max(o.redactable_after) FROM client_erasure_outbox o
		          WHERE o.client_id = c.id AND o.redactable_after IS NOT NULL
		            AND o.status <> 'sent')
		 FROM clients c WHERE c.id = $1`,
		clientID,
	).Scan(&erased, &eligible)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests -- the row was already read by fetchRecord above
		return nil, nil, fmt.Errorf("client: read erasure state: %w", err)
	}
	if erased.Valid {
		erasedAt = &erased.Time
	}
	if eligible.Valid {
		redactionEligibleAt = &eligible.Time
	}
	return erasedAt, redactionEligibleAt, nil
}

// unreadableDiff is what a sealed diff renders as once the Client's key
// has been destroyed -- ADR-0027's crypto-shredding, seen from the read
// side. It is deliberately a value the screen can render rather than an
// error the request fails on: the entry is still a true record that
// something happened, on that date, by that person, and only *what*
// changed is gone.
const unreadableDiff = `{"erased":true}`

// openDiff turns whatever is stored in activity.diff into what a reader
// should see. Three cases, in the order they are met:
//
//   - a plaintext diff, written before #394 -- activity is append-only,
//     so those rows were never converted and are returned as they are;
//   - a sealed diff whose key still exists -- opened;
//   - a sealed diff whose key is gone -- unreadableDiff.
//
// Any other failure to open (a corrupted envelope, a truncated
// ciphertext) lands in the same place rather than failing the read: the
// screen's honest answer to "what changed here?" in that case is the
// same as after erasure -- it cannot be read.
func openDiff(ctx context.Context, tx *sql.Tx, clientID string, stored []byte) json.RawMessage {
	if !clientkey.IsSealed(stored) {
		return stored
	}
	opened, err := clientkey.Open(ctx, tx, clientID, stored)
	if err != nil {
		return json.RawMessage(unreadableDiff)
	}
	return opened
}

func listRequestsForClient(ctx context.Context, tx *sql.Tx, clientID string) ([]RequestSummary, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT r.id, r.kind::text, r.state::text, r.requested_by, rs.name, r.requested_at,
		        r.decided_by, ds.name, r.decided_at, r.reason, r.engagement_id
		 FROM engagement_requests r
		 JOIN staff rs ON rs.id = r.requested_by
		 LEFT JOIN staff ds ON ds.id = r.decided_by
		 WHERE r.client_id = $1 ORDER BY r.requested_at`,
		clientID,
	)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("client: list requests for client: %w", err)
	}
	defer func() { _ = rows.Close() }()

	requests := []RequestSummary{}
	for rows.Next() {
		var req RequestSummary
		var decidedBy, decidedByName, reason, engagementID sql.NullString
		var decidedAt sql.NullTime
		if err := rows.Scan(&req.RequestID, &req.Kind, &req.State, &req.RequestedBy, &req.RequestedByName, &req.RequestedAt,
			&decidedBy, &decidedByName, &decidedAt, &reason, &engagementID); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("client: scan request: %w", err)
		}
		if decidedBy.Valid {
			req.DecidedBy = &decidedBy.String
		}
		if decidedByName.Valid {
			req.DecidedByName = &decidedByName.String
		}
		if decidedAt.Valid {
			req.DecidedAt = &decidedAt.Time
		}
		if reason.Valid {
			req.Reason = &reason.String
		}
		if engagementID.Valid {
			req.EngagementID = &engagementID.String
		}
		requests = append(requests, req)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("client: iterate requests: %w", err)
	}
	return requests, nil
}
