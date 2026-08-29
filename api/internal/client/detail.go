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

	"doula-cloud/api/internal/staffauth"
)

// Event is one client_events row: what changed, when, and who did
// it -- append-only, so this is always the row exactly as written.
type Event struct {
	EventType    string          `json:"eventType"`
	Diff         json.RawMessage `json:"diff"`
	ActorKind    string          `json:"actorKind"`
	ActorStaffID *string         `json:"actorStaffId,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
}

// RequestSummary is one engagement_requests row -- the audit record for
// "this Engagement began because she asked and he agreed" (ADR-0017),
// merged into the same history as her client_events rows rather than
// mirrored into one.
type RequestSummary struct {
	RequestID    string     `json:"requestId"`
	Kind         string     `json:"kind"`
	State        string     `json:"state"`
	RequestedBy  string     `json:"requestedBy"`
	RequestedAt  time.Time  `json:"requestedAt"`
	DecidedBy    *string    `json:"decidedBy,omitempty"`
	DecidedAt    *time.Time `json:"decidedAt,omitempty"`
	Reason       *string    `json:"reason,omitempty"`
	EngagementID *string    `json:"engagementId,omitempty"`
}

// HistoryEntry is one row of a Client's merged history -- her
// client_events rows and her engagement_requests rows, interleaved by
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
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		canAccess, err := reader.CanAccessClient(r.Context(), tx, clientID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !canAccess {
			http.Error(w, "client not found", http.StatusNotFound)
			return
		}

		rec, err := fetchRecord(r.Context(), tx, practiceID, clientID)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "client not found", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		engagements, err := listEngagementsForClient(r.Context(), tx, clientID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		history, err := mergedHistory(r.Context(), tx, clientID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		resolvedFields, err := resolveFields(r.Context(), tx, practiceID, rec.FieldValues)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		out := DetailResponse{Record: rec, ResolvedFields: resolvedFields, Engagements: engagements, History: history}
		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(out); err != nil {
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		}
	})
}

// mergedHistory reads clientID's client_events and engagement_requests
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
		`SELECT event_type::text, diff, actor_kind::text, actor_staff_id, created_at
		 FROM client_events WHERE client_id = $1 ORDER BY created_at`,
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
		var actorStaffID sql.NullString
		var diff []byte
		if err := rows.Scan(&e.EventType, &diff, &e.ActorKind, &actorStaffID, &e.CreatedAt); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("client: scan client event: %w", err)
		}
		e.Diff = diff
		if actorStaffID.Valid {
			e.ActorStaffID = &actorStaffID.String
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return nil, fmt.Errorf("client: iterate client events: %w", err)
	}
	return events, nil
}

func listRequestsForClient(ctx context.Context, tx *sql.Tx, clientID string) ([]RequestSummary, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT id, kind::text, state::text, requested_by, requested_at, decided_by, decided_at, reason, engagement_id
		 FROM engagement_requests WHERE client_id = $1 ORDER BY requested_at`,
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
		var decidedBy, reason, engagementID sql.NullString
		var decidedAt sql.NullTime
		if err := rows.Scan(&req.RequestID, &req.Kind, &req.State, &req.RequestedBy, &req.RequestedAt,
			&decidedBy, &decidedAt, &reason, &engagementID); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return nil, fmt.Errorf("client: scan request: %w", err)
		}
		if decidedBy.Valid {
			req.DecidedBy = &decidedBy.String
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
