package staffauth

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"doula-cloud/api/internal/authn"
)

// MsgInternalError is the response body for any failure the caller can't
// act on (a DB error, an encoding error) -- exported so every handler and
// the middleware, in this package and in main, share one literal instead
// of duplicating it per call site.
const MsgInternalError = "internal error"

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
		tx, uid, ok := authn.BeginBootstrap(w, r, verifier, db)
		if !ok {
			return
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()

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

		resp, status, msg := signup(r, tx, uid, req)
		if status != http.StatusCreated {
			http.Error(w, msg, status)
			return
		}

		// Create the session before committing, so a failure rolls the
		// new rows back instead of leaving them committed behind a
		// response that reports failure (#145). uid is the identity
		// authn.Begin already verified.
		cookie, err := authn.MintSession(r.Context(), tx, uid, time.Now())
		if err != nil {
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB commit failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}
		committed = true

		http.SetCookie(w, cookie)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
		}
	})
}

func signup(r *http.Request, tx *sql.Tx, identityUID string, req SignupRequest) (SignupResponse, int, string) {
	ctx := r.Context()

	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_identity_uid', $1, true)`, identityUID); err != nil {
		return SignupResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	var practiceID string
	if err := tx.QueryRowContext(ctx, `INSERT INTO practices (name) VALUES ($1) RETURNING id`, req.PracticeName).Scan(&practiceID); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return SignupResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	// staff_self_visibility (00002) only admits the caller's own row while
	// app.current_practice_id is unset -- see that policy's comment -- so
	// the staff INSERT ... RETURNING below must run before it's set. Only
	// after staff exists is app.current_practice_id set, which then covers
	// the signup-bonus, membership, and plan_templates inserts that need
	// it.
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
		return SignupResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		return SignupResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	// The signup bonus: +3 credits, granted exactly once per Practice, in
	// the same transaction as the Practice row itself so a Practice can
	// never exist without it.
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO credit_ledger (practice_id, origin, quantity) VALUES ($1, 'signup_bonus', 3)`,
		practiceID,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return SignupResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO practice_memberships (practice_id, staff_id, roles) VALUES ($1, $2, '{owner,office_manager,doula}')`,
		practiceID, staffID,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return SignupResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO plan_templates (practice_id, plan_type, fields) VALUES ($1, 'care_plan', $2), ($1, 'birth_plan', $3)`,
		practiceID, defaultCarePlanFields, defaultBirthPlanFields,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return SignupResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO contract_templates (practice_id, prose) VALUES ($1, $2)`,
		practiceID, defaultContractTemplateProse,
	); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return SignupResponse{}, http.StatusInternalServerError, MsgInternalError
	}

	return SignupResponse{StaffID: staffID, PracticeID: practiceID}, http.StatusCreated, ""
}

// defaultCarePlanFields and defaultBirthPlanFields are the field lists a
// new Practice starts with (ADR-0001: seeding applies going forward only,
// no backfill for practices created before this ticket). Each field must
// match the palette and options shape validated server-side by the
// plans package (api/internal/plans/template.go) -- kept as a literal
// here, rather than importing that package, to avoid a staffauth<->plans
// import cycle (plans imports staffauth for its own handlers).
const defaultCarePlanFields = `[
	{"id": "support-people", "type": "section_header", "label": "Support People", "order": 0},
	{"id": "support-people-names", "type": "short_text", "label": "Who will be present for support?", "order": 1},
	{"id": "pain-management", "type": "single_select", "label": "Preferred pain management", "options": ["Unmedicated", "Epidural", "Open to options in the moment"], "order": 2},
	{"id": "special-requests", "type": "long_text", "label": "Special requests or concerns", "order": 3},
	{"id": "share-with-backup", "type": "checkbox", "label": "OK to share this plan with a backup doula", "order": 4}
]`

const defaultBirthPlanFields = `[
	{"id": "birth-setting", "type": "section_header", "label": "Birth Setting", "order": 0},
	{"id": "location", "type": "single_select", "label": "Planned birth location", "options": ["Home", "Birth center", "Hospital"], "order": 1},
	{"id": "notify", "type": "multi_select", "label": "People to notify when labor starts", "options": ["Partner", "Doula", "Midwife", "OB", "Family"], "order": 2},
	{"id": "atmosphere", "type": "long_text", "label": "Preferences for atmosphere (music, lighting, etc.)", "order": 3},
	{"id": "consent-photos", "type": "checkbox", "label": "OK to take photos/video during labor", "order": 4}
]`

// defaultContractTemplateProse is the Contract Template prose a new
// Practice starts with (seeding applies going forward only, no backfill
// for practices created before this ticket). Its merge-field placeholders
// match the tokens the contracts package/settings screen document as
// available -- kept as a literal here, rather than importing the
// contracts package, to avoid a staffauth<->contracts import cycle
// (contracts imports staffauth for its own handlers).
const defaultContractTemplateProse = `This agreement is between {{practice_name}} and {{client_name}} for doula services.

Scope of service: {{scope_of_service}}

Engagement dates: {{engagement_start_date}} through {{engagement_end_date}}

Price: {{price}}

By signing below, both parties agree to the terms described above.`

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
