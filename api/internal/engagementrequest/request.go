package engagementrequest

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"doula-cloud/api/internal/pgerr"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/tasknudge"
)

// RequestBody is the body of a new Engagement Request: the Client, the
// kind and due date, and an optional note -- ADR-0017's "the requester
// describes the work; the approver does not amend it".
type RequestBody struct {
	Kind    string `json:"kind"`
	DueDate string `json:"dueDate"`
	Note    string `json:"note"`
}

// RequestResponse reports the Request created. State is "pending" for
// the ordinary path, or "approved" with an EngagementID set when the
// requester already held approval authority herself and ADR-0017's
// solo-Practice collapse fired.
type RequestResponse struct {
	RequestID    string `json:"requestId"`
	State        string `json:"state"`
	EngagementID string `json:"engagementId,omitempty"`
	Warning      string `json:"warning,omitempty"`
}

// RequestHandler asks for a new Engagement to start. Any Staff member but
// a contractor Doula (ADR-0017: "a contractor originates nothing") --
// enforced here and, independently, by engagement_requests_insert's RLS
// policy (00047). When the requester already holds approval authority
// (an Owner or an Admin), the request and its approval collapse into one
// act: one row, created and decided in the same instant, mailing nobody.
// Must be mounted behind staffauth.Middleware; db and enq are needed only
// for that collapsed path's own possible ErrNoCreditsRemaining branch.
func RequestHandler(db *sql.DB, enq tasknudge.Enqueuer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}
		staffID, _ := staffauth.StaffID(r.Context())
		reader, err := staffauth.ResolveReader(r.Context(), tx, practiceID, staffID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if isContractorOriginator(reader) {
			http.Error(w, "a contractor doula does not request an engagement at a practice she contracts for -- work reaches her as an offer", http.StatusForbidden)
			return
		}

		clientID := r.PathValue("clientId")
		if !staffauth.ParseUUID(w, "client", clientID) {
			return
		}
		if !clientExists(r.Context(), tx, clientID) {
			http.Error(w, "client not found", http.StatusNotFound)
			return
		}

		var body RequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		kind, dueDate, ok := parseRequestBody(w, body)
		if !ok {
			return
		}
		note := sql.NullString{String: strings.TrimSpace(body.Note), Valid: strings.TrimSpace(body.Note) != ""}

		var requestID string
		if err := tx.QueryRowContext(r.Context(),
			`INSERT INTO engagement_requests (practice_id, client_id, kind, due_date, note, requested_by)
			 VALUES ($1, $2, $3::engagement_kind, $4, $5, $6) RETURNING id`,
			practiceID, clientID, kind, dueDate, note, staffID,
		).Scan(&requestID); err != nil {
			if pgerr.IsUniqueViolation(err) {
				http.Error(w, "a pending request for this client and kind already exists", http.StatusConflict)
				return
			}
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		if mayApproveDirectly(reader) {
			collapse(w, r, db, enq, tx, practiceID, requestID, staffID)
			return
		}

		warning, err := hasLiveEngagement(r.Context(), tx, clientID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if err := queueOutbox(r.Context(), tx, practiceID, requestID); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}
		tasknudge.Register(r.Context(), tasknudge.Fire(enq, tasknudge.EngagementRequest))

		resp := RequestResponse{RequestID: requestID, State: statePending}
		if warning {
			resp.Warning = liveEngagementWarning
		}
		writeJSON(w, http.StatusCreated, resp)
	})
}

// collapse finishes the pending row RequestHandler just inserted with an
// immediate approval, ADR-0017's solo-Practice rule: "the row is still
// written, created and decided in the same instant by the same person."
// A collapsed request-and-approval mails nobody -- no outbox row is
// queued on this path. On an empty balance the whole act fails and
// nothing survives, including the request row just inserted: a pending
// Request only the same person could ever approve is not a coherent
// state to leave behind.
func collapse(w http.ResponseWriter, r *http.Request, db *sql.DB, enq tasknudge.Enqueuer, tx *sql.Tx, practiceID, requestID, approverStaffID string) {
	engagementID, warning, err := approve(r.Context(), tx, practiceID, requestID, approverStaffID)
	if err != nil {
		writeApproveErr(w, r, db, enq, tx, practiceID, err)
		return
	}
	resp := RequestResponse{RequestID: requestID, State: stateApproved, EngagementID: engagementID}
	if warning {
		resp.Warning = liveEngagementWarning
	}
	writeJSON(w, http.StatusCreated, resp)
}

// clientExists reports whether clientID is visible to the caller's
// Practice -- RLS (clients_select, 00042) already scopes the read.
func clientExists(ctx context.Context, tx *sql.Tx, clientID string) bool {
	var exists bool
	// coverage:ignore reason: DB query failure, not exercised by unit tests -- treated as "not found" either way
	_ = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM clients WHERE id = $1)`, clientID).Scan(&exists)
	return exists
}

// parseRequestBody validates kind and, if present, dueDate. Writes its
// own 400 and returns ok=false on failure.
func parseRequestBody(w http.ResponseWriter, body RequestBody) (kind string, dueDate sql.NullString, ok bool) {
	kind = strings.TrimSpace(body.Kind)
	if !validKinds[kind] {
		http.Error(w, "kind must be 'birth' or 'postpartum'", http.StatusBadRequest)
		return "", sql.NullString{}, false
	}
	due := strings.TrimSpace(body.DueDate)
	if due == "" {
		return kind, sql.NullString{}, true
	}
	if _, err := time.Parse(time.DateOnly, due); err != nil {
		http.Error(w, "dueDate must be YYYY-MM-DD", http.StatusBadRequest)
		return "", sql.NullString{}, false
	}
	return kind, sql.NullString{String: due, Valid: true}, true
}
