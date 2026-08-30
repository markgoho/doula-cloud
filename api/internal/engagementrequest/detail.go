package engagementrequest

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/billing"
	"doula-cloud/api/internal/client"
	"doula-cloud/api/internal/staffauth"
)

// creditCost is what one Engagement costs, always: ADR-0015's "a Credit
// locks when an Engagement is created", and approve() spends exactly one.
// It is carried in the response rather than hardcoded in the screen so
// the price lives in one place the day it stops being one.
const creditCost = 1

// ClientIdentity is who the Request is about, and whether the Practice
// has met her before. Only the three name columns travel: the approval
// screen names her and links to her record, and the rest of ADR-0017's
// twelve columns are one click away on a read that already exists.
// IsNewToPractice is false as soon as she holds any Engagement here or
// any Engagement Request but this one -- the "new or already known"
// split #401's map asks the approver to see at a glance.
type ClientIdentity struct {
	ClientID        string `json:"clientId"`
	GivenName       string `json:"givenName"`
	FamilyName      string `json:"familyName"`
	PreferredName   string `json:"preferredName"`
	IsNewToPractice bool   `json:"isNewToPractice"`
}

// DetailResponse is everything the approval screen shows for one pending
// Request, in one read: the ask exactly as it was made (kind, due date,
// note -- ADR-0017's "the approver approves or refuses exactly what was
// described"), who made it and when, who it is about, what it costs and
// what the balance is left at, her Engagements past and present, and the
// second-live-Engagement warning at the approver's seat.
//
// Balance travels alongside BalanceAfter because a screen that must offer
// Buy Credits before the approver spends a Credit she does not have needs
// the balance itself, not only the arithmetic. BalanceAfter is honestly
// negative on an empty balance rather than clamped at zero: it is the
// balance this approval would leave, and an approval into an empty
// balance does not happen.
type DetailResponse struct {
	RequestID       string                     `json:"requestId"`
	State           string                     `json:"state"`
	Kind            string                     `json:"kind"`
	DueDate         *string                    `json:"dueDate,omitempty"`
	Note            *string                    `json:"note,omitempty"`
	RequestedBy     string                     `json:"requestedBy"`
	RequestedByName string                     `json:"requestedByName"`
	RequestedAt     time.Time                  `json:"requestedAt"`
	Client          ClientIdentity             `json:"client"`
	CreditCost      int                        `json:"creditCost"`
	Balance         int                        `json:"balance"`
	BalanceAfter    int                        `json:"balanceAfter"`
	Engagements     []client.EngagementSummary `json:"engagements"`
	Warning         string                     `json:"warning,omitempty"`
}

// DetailHandler reads one pending Engagement Request for the approval
// screen. Owner or Admin only, the same seat that may decide it -- a
// Doula reading a decision she cannot make is a screen nobody asked for,
// and the balance it carries is Owner/Admin-only on its own
// (billing.GetBalanceHandler, ADR-0008). Must be mounted behind
// staffauth.Middleware.
//
// A Request that has already been decided is a 409, not a 200 with a
// disabled screen: the same 404-vs-409 split approve and refuse draw, so
// a stale link says which of the two happened.
func DetailHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireOwnerOrAdmin(w, r)
		if !ok {
			return
		}

		requestID := r.PathValue("requestId")
		if !staffauth.ParseUUID(w, "request", requestID) {
			return
		}

		resp, err := loadDetail(r.Context(), tx, practiceID, requestID)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			http.Error(w, "engagement request not found", http.StatusNotFound)
			return
		case errors.Is(err, errNotPending):
			writeRequestNotDecidable(w, r, tx, requestID)
			return
		case err != nil:
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, resp)
	})
}

// loadDetail assembles the approval screen's read. It returns
// sql.ErrNoRows when requestID is not visible at this Practice (RLS
// scopes the row) and errNotPending when it exists but has been decided.
func loadDetail(ctx context.Context, tx *sql.Tx, practiceID, requestID string) (DetailResponse, error) {
	resp, clientID, err := scanRequest(ctx, tx, requestID)
	if err != nil {
		return DetailResponse{}, err
	}

	engagements, err := client.ListEngagementsForClient(ctx, tx, clientID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return DetailResponse{}, fmt.Errorf("engagementrequest: list engagements: %w", err)
	}
	resp.Engagements = engagements

	warning, err := hasLiveEngagement(ctx, tx, clientID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return DetailResponse{}, err
	}
	if warning {
		resp.Warning = liveEngagementWarning
	}

	otherRequests, err := hasOtherRequests(ctx, tx, clientID, requestID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return DetailResponse{}, err
	}
	resp.Client.IsNewToPractice = len(engagements) == 0 && !otherRequests

	balance, err := billing.Balance(ctx, tx, practiceID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return DetailResponse{}, fmt.Errorf("engagementrequest: read balance: %w", err)
	}
	resp.CreditCost = creditCost
	resp.Balance = balance
	resp.BalanceAfter = balance - creditCost

	return resp, nil
}

// scanRequest reads the Request row itself with its requester's name and
// the Client's three name columns joined in, and refuses anything but a
// pending Request.
func scanRequest(ctx context.Context, tx *sql.Tx, requestID string) (resp DetailResponse, clientID string, err error) {
	var dueDate, note sql.NullString
	err = tx.QueryRowContext(ctx,
		`SELECT r.id, r.state::text, r.kind::text, r.due_date::text, r.note,
		        r.requested_by, rs.name, r.requested_at,
		        c.id, c.given_name, coalesce(c.family_name, ''), coalesce(c.preferred_name, '')
		   FROM engagement_requests r
		   JOIN staff rs ON rs.id = r.requested_by
		   JOIN clients c ON c.id = r.client_id
		  WHERE r.id = $1`,
		requestID,
	).Scan(&resp.RequestID, &resp.State, &resp.Kind, &dueDate, &note,
		&resp.RequestedBy, &resp.RequestedByName, &resp.RequestedAt,
		&resp.Client.ClientID, &resp.Client.GivenName, &resp.Client.FamilyName, &resp.Client.PreferredName)
	if errors.Is(err, sql.ErrNoRows) {
		return DetailResponse{}, "", err //nolint:wrapcheck // sql.ErrNoRows is the sentinel DetailHandler errors.Is against, per loadDetail's doc comment
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return DetailResponse{}, "", fmt.Errorf("engagementrequest: read request: %w", err)
	}
	if resp.State != statePending {
		return DetailResponse{}, "", errNotPending
	}
	if dueDate.Valid {
		resp.DueDate = &dueDate.String
	}
	if note.Valid {
		resp.Note = &note.String
	}
	return resp, resp.Client.ClientID, nil
}

// hasOtherRequests reports whether clientID has been asked about before,
// ignoring the Request being decided -- the second half of the
// new-or-already-known split, so a Client whose only history is a refused
// or withdrawn ask still reads as known.
func hasOtherRequests(ctx context.Context, tx *sql.Tx, clientID, requestID string) (bool, error) {
	var has bool
	err := tx.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM engagement_requests WHERE client_id = $1 AND id <> $2)`,
		clientID, requestID,
	).Scan(&has)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return false, fmt.Errorf("engagementrequest: check other requests: %w", err)
	}
	return has, nil
}
