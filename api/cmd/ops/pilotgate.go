package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
)

const (
	msgGateNoFields    = "Send no fields with the opening. Who opens the gate comes from your sign-in."
	msgGateAlreadyOpen = "The pilot gate is already open. It was opened on %s, and nothing has changed."
	msgGateOpenRaced   = "The pilot gate was opened while this opening was being made, so nothing has changed."
)

type pilotGateState struct {
	OpenedAt *time.Time `json:"openedAt"`
}

type pilotGateActDetails struct {
	RequestKey string `json:"requestKey"`
}

func pilotGateRead(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var gate pilotGateState
		if err := db.QueryRowContext(r.Context(), `SELECT ops_pilot_gate_opened_at()`).Scan(&gate.OpenedAt); err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, gate)
	}
}

func pilotGateOpenAct(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestKey, ok := requestKeyOf(w, r, "opening")
		if !ok {
			return
		}
		if !decodeDeclared(w, r, map[string]bool{}, msgGateNoFields, &struct{}{}) {
			return
		}

		op := signedIn(r)
		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		defer func() { _ = tx.Rollback() }()

		if err := spendUnlessReplay(r.Context(), tx, op, actPilotGateOpened, requestKey); err != nil {
			// coverage:ignore reason: a DB failure, or a second act spending the stamp between requireStepUp's read and this one; stepup_test.go covers spendStepUp's refusal
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
				writeInternal(w)
				return
			}
			apierr.WriteJSON(w, http.StatusOK, pilotGateState{OpenedAt: &priorAt})
			return
		}
		if !errors.Is(err, sql.ErrNoRows) {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}

		var openedAt *time.Time
		if err := tx.QueryRowContext(r.Context(), `SELECT ops_pilot_gate_opened_at()`).Scan(&openedAt); err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		if openedAt != nil {
			apierr.Write(w, http.StatusConflict, apierr.CodeConflict,
				fmt.Sprintf(msgGateAlreadyOpen, businessDay(*openedAt)), nil)
			return
		}

		now := clock.Now(r.Context())
		actID, err := recordAct(r.Context(), tx, op, actPilotGateOpened, nil, pilotGateActDetails{RequestKey: requestKey}, op.StepUpAt, now)
		if err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		var opened bool
		if err := tx.QueryRowContext(r.Context(), `SELECT ops_open_pilot_gate($1, $2)`, actID, now).Scan(&opened); err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		if !opened {
			// coverage:ignore reason: a second opening committed between this request's read and its update; WHERE opened_at IS NULL is what refuses it
			apierr.Write(w, http.StatusConflict, apierr.CodeConflict, msgGateOpenRaced, nil)
			return
		}
		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, pilotGateState{OpenedAt: &now})
	}
}
