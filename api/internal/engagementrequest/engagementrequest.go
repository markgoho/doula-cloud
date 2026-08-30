// Package engagementrequest is the Staff-side BFF surface for ADR-0017's
// Engagement Request: the four decision endpoints (request, approve,
// refuse, withdraw), the approval screen's own read (detail.go), and the
// act that turns an approval into an Engagement.
// Approval is the only path anywhere in the codebase that inserts an
// engagements row -- see approve() in approve.go, shared by ApproveHandler
// and RequestHandler's solo-Practice collapsed path.
package engagementrequest

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgconn"

	"doula-cloud/api/internal/staffauth"
)

// The engagement_request_state enum values (00042) this package writes
// and compares against in Go.
const (
	statePending  = "pending"
	stateApproved = "approved"
)

// validKinds mirrors the engagement_kind enum (00042).
var validKinds = map[string]bool{"birth": true, "postpartum": true}

// DecisionResponse reports the state a Request ended up in -- the same
// shape for refuse and withdraw, mirroring offer.DecisionResponse.
type DecisionResponse struct {
	RequestID string `json:"requestId"`
	State     string `json:"state"`
}

// isContractorOriginator reports whether reader is barred from
// originating an Engagement Request -- ADR-0017's "a contractor
// originates nothing" -- unless she also holds approval authority
// herself (the solo-Practice case: an Owner or Admin who happens to work
// under a contractor employment type stays admitted).
func isContractorOriginator(reader staffauth.Reader) bool {
	return reader.IsContractor() && !reader.Has("owner") && !reader.Has("admin")
}

// mayApproveDirectly reports whether reader already holds approval
// authority -- ADR-0017's "where the requester already holds approval
// authority, request and approval are one act", independent of
// employment type.
func mayApproveDirectly(reader staffauth.Reader) bool {
	return reader.Has("owner") || reader.Has("admin")
}

// hasLiveEngagement reports whether clientID already holds an Engagement
// that has not completed -- ADR-0017's "a second live Engagement warns,
// never refuses", surfaced at both request time and approval time.
func hasLiveEngagement(ctx context.Context, tx *sql.Tx, clientID string) (bool, error) {
	var has bool
	err := tx.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM engagements WHERE client_id = $1 AND status <> 'completed')`,
		clientID,
	).Scan(&has)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return false, fmt.Errorf("engagementrequest: check live engagement: %w", err)
	}
	return has, nil
}

// isUniqueViolation reports whether err is a Postgres unique-constraint
// violation (SQLSTATE 23505), mirroring staffauth.isUniqueViolation.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// writeJSON encodes v as the response body under status. The header must
// be set before WriteHeader, so this is the one place in the package that
// writes the status line -- callers never call w.WriteHeader themselves.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// coverage:ignore reason: response encoding failure, not exercised by unit tests
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
	}
}
