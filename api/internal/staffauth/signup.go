package staffauth

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"

	"doula-cloud/api/internal/authn"
)

// msgInternalError is the response body for any failure the caller can't
// act on (a DB error, an encoding error) -- shared across signup and
// session handlers so it isn't duplicated per call site.
const msgInternalError = "internal error"

// SignupRequest is the body of a Practice-signup request: a new Practice,
// created together with the Staff row for the person creating it.
type SignupRequest struct {
	PracticeName string `json:"practiceName"`
	StaffName    string `json:"staffName"`
	StaffEmail   string `json:"staffEmail"`
}

// SignupResponse identifies the Practice and Staff row signup created.
type SignupResponse struct {
	StaffID    string `json:"staffId"`
	PracticeID string `json:"practiceId"`
}

// SignupHandler creates a new Practice, a Staff row for the verified
// caller, and a practice_memberships row holding all three roles (Owner,
// Office Manager, Doula), in one transaction. It runs before any Practice
// is known, so -- unlike Middleware -- it never sets
// app.current_practice_id until the new Practice's id exists to set it
// to.
func SignupHandler(verifier authn.Verifier, db *sql.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idToken, ok := bearerToken(r)
		if !ok {
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return
		}

		verified, err := verifier.VerifyIDToken(r.Context(), idToken)
		if err != nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		var req SignupRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		req.PracticeName = strings.TrimSpace(req.PracticeName)
		req.StaffName = strings.TrimSpace(req.StaffName)
		req.StaffEmail = strings.TrimSpace(req.StaffEmail)
		if req.PracticeName == "" || req.StaffName == "" || req.StaffEmail == "" {
			http.Error(w, "practiceName, staffName, and staffEmail are required", http.StatusBadRequest)
			return
		}

		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			// coverage:ignore reason: DB connection failure, not exercised by unit tests
			http.Error(w, msgInternalError, http.StatusInternalServerError)
			return
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()

		resp, status, msg := signup(r, tx, verified.UID, req)
		if status != http.StatusCreated {
			http.Error(w, msg, status)
			return
		}

		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB commit failure, not exercised by unit tests
			http.Error(w, msgInternalError, http.StatusInternalServerError)
			return
		}
		committed = true

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, msgInternalError, http.StatusInternalServerError)
		}
	})
}

func signup(r *http.Request, tx *sql.Tx, identityUID string, req SignupRequest) (SignupResponse, int, string) {
	ctx := r.Context()

	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_identity_uid', $1, true)`, identityUID); err != nil {
		return SignupResponse{}, http.StatusInternalServerError, msgInternalError
	}

	var practiceID string
	if err := tx.QueryRowContext(ctx, `INSERT INTO practices (name) VALUES ($1) RETURNING id`, req.PracticeName).Scan(&practiceID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return SignupResponse{}, http.StatusInternalServerError, msgInternalError
	}

	var staffID string
	err := tx.QueryRowContext(ctx,
		`INSERT INTO staff (identity_uid, name, email) VALUES ($1, $2, $3) RETURNING id`,
		identityUID, req.StaffName, req.StaffEmail,
	).Scan(&staffID)
	if isUniqueViolation(err) {
		return SignupResponse{}, http.StatusConflict, "a staff account already exists for this identity"
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return SignupResponse{}, http.StatusInternalServerError, msgInternalError
	}

	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		return SignupResponse{}, http.StatusInternalServerError, msgInternalError
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO practice_memberships (practice_id, staff_id, roles) VALUES ($1, $2, '{owner,office_manager,doula}')`,
		practiceID, staffID,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return SignupResponse{}, http.StatusInternalServerError, msgInternalError
	}

	return SignupResponse{StaffID: staffID, PracticeID: practiceID}, http.StatusCreated, ""
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
