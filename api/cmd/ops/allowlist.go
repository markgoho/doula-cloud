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
	"doula-cloud/api/internal/staffauth"
)

const (
	msgPilotOtherField    = "Send only the address. Who adds it comes from your sign-in."
	msgPilotAddressFormat = "Enter an email address in the correct format, like name@example.com"
	msgPilotKeyReused     = "This Idempotency-Key was already used to add a different address. Nothing has changed."
	msgPilotAlreadyAdded  = "%s is already on the pilot allowlist. It was added on %s, and nothing has changed."
	msgPilotAddRaced      = "%s was added to the pilot allowlist while this add was being made, so nothing has changed."
)

var pilotAddressFields = map[string]bool{"address": true}

type pilotAddressRequest struct {
	Address string `json:"address"`
}

// pilotAddressResult says what changed: the address, as stored, and when.
type pilotAddressResult struct {
	Address string    `json:"address"`
	AddedAt time.Time `json:"addedAt"`
}

// pilotAddressActDetails is the add's operator_acts details: the address
// it added, and the request key a retry is matched on.
type pilotAddressActDetails struct {
	Address    string `json:"address"`
	RequestKey string `json:"requestKey"`
}

// pilotAddressAct adds an address to the pilot allowlist (ADR-0062's act
// table): add only, under an Idempotency-Key that names the request so a
// retry is the same add. It is recorded as an operator_acts row the
// allowlist row keys.
func pilotAddressAct(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestKey, ok := requestKeyOf(w, r, "address")
		if !ok {
			return
		}
		var req pilotAddressRequest
		if !decodeDeclared(w, r, pilotAddressFields, msgPilotOtherField, &req) {
			return
		}
		address := staffauth.NormalizeAddress(req.Address)
		if !staffauth.LooksLikeAddress(address) {
			apierr.Write(w, http.StatusUnprocessableEntity, apierr.CodeInvalidArgument, msgCorrectAnswers,
				map[string]string{"address": msgPilotAddressFormat})
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

		var priorDetails []byte
		var priorAt time.Time
		err = tx.QueryRowContext(r.Context(),
			`SELECT details, created_at FROM operator_acts
			  WHERE operator_id = $1 AND action = 'pilot_address_added' AND details->>'requestKey' = $2`,
			op.ID, requestKey).Scan(&priorDetails, &priorAt)
		if err == nil {
			var prior pilotAddressActDetails
			_ = json.Unmarshal(priorDetails, &prior)
			if prior.Address != address {
				apierr.Write(w, http.StatusUnprocessableEntity, apierr.CodeInvalidArgument, msgPilotKeyReused, nil)
				return
			}
			apierr.WriteJSON(w, http.StatusOK, pilotAddressResult{Address: address, AddedAt: priorAt})
			return
		}
		if !errors.Is(err, sql.ErrNoRows) {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}

		var addedAt time.Time
		err = tx.QueryRowContext(r.Context(), `SELECT added_at FROM ops_pilot_address_added_at($1)`, address).Scan(&addedAt)
		if err == nil {
			apierr.Write(w, http.StatusConflict, apierr.CodeConflict,
				fmt.Sprintf(msgPilotAlreadyAdded, address, addedAt.In(businessZone).Format("2 January 2006")), nil)
			return
		}
		if !errors.Is(err, sql.ErrNoRows) {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}

		now := clock.Now(r.Context())
		actID, err := recordAct(r.Context(), tx, op, actPilotAddressAdded, nil,
			pilotAddressActDetails{Address: address, RequestKey: requestKey}, nil, now)
		if err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		var written bool
		if err := tx.QueryRowContext(r.Context(), `SELECT ops_add_pilot_address($1, $2, $3)`,
			address, actID, now).Scan(&written); err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		if !written {
			// coverage:ignore reason: a second add of the address committed between this request's read and its insert; ON CONFLICT is what refuses it
			apierr.Write(w, http.StatusConflict, apierr.CodeConflict, fmt.Sprintf(msgPilotAddRaced, address), nil)
			return
		}
		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, pilotAddressResult{Address: address, AddedAt: now})
	}
}
