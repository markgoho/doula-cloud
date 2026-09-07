// Package contracts holds the Staff-side BFF handlers for a Practice's
// Contract Template -- the legal prose, carrying merge-field placeholders
// (client name, price, engagement dates, scope of service), that a later
// ticket fills in per Engagement. All handlers rely on
// staffauth.Middleware having already resolved the caller's Staff/Practice
// ids and opened a request-scoped *sql.Tx with app.current_practice_id
// set, the same way plans does.
package contracts

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/staffauth"
)

// TemplateResponse is the body of both the GET response and the PUT
// request/response: a Practice's Contract Template prose.
type TemplateResponse struct {
	Prose string `json:"prose"`
}

// GetTemplateHandler lets any Staff member at the current Practice read
// its Contract Template. Must be mounted behind staffauth.Middleware.
func GetTemplateHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}

		prose, found, err := fetchProse(r.Context(), tx, practiceID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !found {
			apierr.WriteError(w, "no contract template found for this practice", http.StatusNotFound)
			return
		}

		apierr.WriteJSON(w, http.StatusOK, TemplateResponse{Prose: prose})
	})
}

// PutTemplateHandler lets a Practice Owner replace its Contract Template's
// prose. Must be mounted behind staffauth.Middleware.
func PutTemplateHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := staffauth.RequireTx(w, r)
		// coverage:ignore reason: staffauth.Middleware always sets a tx before this handler runs
		if !ok {
			return
		}
		reader, has := staffauth.ReaderFrom(r.Context())
		if !has {
			// coverage:ignore reason: staffauth.Middleware always places a Reader on context before this handler runs
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !reader.Has("owner") {
			apierr.WriteError(w, "only a Practice Owner can do that", http.StatusForbidden)
			return
		}

		var req TemplateResponse
		if !apierr.DecodeJSON(w, r, &req) {
			return
		}
		req.Prose = strings.TrimSpace(req.Prose)
		if req.Prose == "" {
			apierr.WriteError(w, "prose is required", http.StatusBadRequest)
			return
		}

		if _, err := tx.ExecContext(r.Context(),
			`INSERT INTO contract_templates (practice_id, prose) VALUES ($1, $2)
			 ON CONFLICT (practice_id) DO UPDATE SET prose = EXCLUDED.prose`,
			practiceID, req.Prose,
		); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		apierr.WriteJSON(w, http.StatusOK, TemplateResponse{Prose: req.Prose})
	})
}

// fetchProse reads the prose stored for practiceID, reporting found=false
// if no row exists (e.g. a Practice created before this feature landed --
// seeding is not backfilled).
func fetchProse(ctx context.Context, tx *sql.Tx, practiceID string) (string, bool, error) {
	var prose string
	err := tx.QueryRowContext(ctx,
		`SELECT prose FROM contract_templates WHERE practice_id = $1`,
		practiceID,
	).Scan(&prose)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return "", false, fmt.Errorf("contracts: fetch prose: %w", err)
	}
	return prose, true, nil
}
