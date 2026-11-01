package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
)

const (
	msgGateKeyRequired = "Send an Idempotency-Key header with the opening."
	msgGateNoFields    = "Send no fields with the opening. Who opens the gate comes from your sign-in."
	msgGateAlreadyOpen = "The pilot gate is already open. It was opened on %s, and nothing has changed."
	msgGateOpenRaced   = "The pilot gate was opened while this opening was being made, so nothing has changed."
)

// pilotGateState is the gate as the check page reads it, and what the
// opening says it changed: when it was opened, or null while closed.
type pilotGateState struct {
	OpenedAt *time.Time `json:"openedAt"`
}

// pilotGateActDetails is the opening's operator_acts details: the request
// key a retry is matched on.
type pilotGateActDetails struct {
	RequestKey string `json:"requestKey"`
}

// pilotGateRead answers the check page through ops_pilot_gate_opened_at.
func pilotGateRead(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var gate pilotGateState
		if err := db.QueryRowContext(r.Context(), `SELECT ops_pilot_gate_opened_at()`).Scan(&gate.OpenedAt); err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, gate)
	}
}

// pilotGateOpenAct opens the pilot gate for everyone, for good (ADR-0043,
// ADR-0062's act table): behind a fresh code, under an Idempotency-Key so
// a retry is the same opening. It is recorded as an operator_acts row the
// gate keys. Nothing closes it again.
func pilotGateOpenAct(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestKey := r.Header.Get(headerIdempotencyKey)
		if requestKey == "" {
			apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument, msgGateKeyRequired, nil)
			return
		}
		if !decodeDeclared(w, r, map[string]bool{}, msgGateNoFields, &struct{}{}) {
			return
		}

		op := signedIn(r)
		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		defer func() { _ = tx.Rollback() }()

		if err := spendStepUp(r.Context(), tx, op); err != nil {
			// coverage:ignore reason: a second act spending the stamp between requireStepUp's read and this one; stepup_test.go covers spendStepUp's refusal
			writeStepUpRequired(w)
			return
		}

		var priorAt time.Time
		err = tx.QueryRowContext(r.Context(),
			`SELECT created_at FROM operator_acts
			  WHERE operator_id = $1 AND action = 'pilot_gate_opened' AND details->>'requestKey' = $2`,
			op.ID, requestKey).Scan(&priorAt)
		if err == nil {
			if err := tx.Commit(); err != nil {
				// coverage:ignore reason: DB failure, not exercised by unit tests
				apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
				return
			}
			apierr.WriteJSON(w, http.StatusOK, pilotGateState{OpenedAt: &priorAt})
			return
		}
		if !errors.Is(err, sql.ErrNoRows) {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		var openedAt *time.Time
		if err := tx.QueryRowContext(r.Context(), `SELECT ops_pilot_gate_opened_at()`).Scan(&openedAt); err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if openedAt != nil {
			apierr.Write(w, http.StatusConflict, apierr.CodeConflict,
				fmt.Sprintf(msgGateAlreadyOpen, openedAt.In(businessZone).Format("January 2, 2006")), nil)
			return
		}

		now := clock.Now(r.Context())
		details, _ := json.Marshal(pilotGateActDetails{RequestKey: requestKey})
		var actID string
		if err := tx.QueryRowContext(r.Context(),
			`INSERT INTO operator_acts (operator_id, action, details, step_up_at, created_at)
			 VALUES ($1, 'pilot_gate_opened', $2, $3, $4) RETURNING id`,
			op.ID, details, op.StepUpAt, now).Scan(&actID); err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		var opened bool
		if err := tx.QueryRowContext(r.Context(), `SELECT ops_open_pilot_gate($1, $2)`, actID, now).Scan(&opened); err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !opened {
			// coverage:ignore reason: a second opening committed between this request's read and its update; WHERE opened_at IS NULL is what refuses it
			apierr.Write(w, http.StatusConflict, apierr.CodeConflict, msgGateOpenRaced, nil)
			return
		}
		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, pilotGateState{OpenedAt: &now})
	}
}
