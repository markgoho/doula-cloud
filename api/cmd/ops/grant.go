package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
)

const (
	msgNoGrantFields  = "Send no fields with the founding grant. Its size comes from the Practice's Staff, and who gives it from your sign-in."
	msgAlreadyGranted = "This Practice already holds its founding grant of %d Credits. A founding grant is given once."
	msgNoStaff        = "This Practice has no Staff, so there is nothing to size a founding grant from."
	msgRosterChanged  = "This Practice's Staff changed while the grant was being given, so nothing has changed. Check the grant again."
)

type foundingGrantCheck struct {
	PracticeName   string `json:"practiceName"`
	StaffCount     int    `json:"staffCount"`
	Credits        int    `json:"credits"`
	GrantedCredits *int   `json:"grantedCredits"`
}

type foundingGrantResult struct {
	PracticeName string `json:"practiceName"`
	StaffCount   int    `json:"staffCount"`
	Credits      int    `json:"credits"`
}

type foundingGrantActDetails struct {
	StaffCount int    `json:"staffCount"`
	Credits    int    `json:"credits"`
	RequestKey string `json:"requestKey"`
}

type foundingGrantDiff struct {
	StaffCount int `json:"staffCount"`
	Credits    int `json:"credits"`
}

type grantPractice struct {
	foundingGrantCheck
	grantedActID sql.NullString
}

func readGrantPractice(ctx context.Context, tx *sql.Tx, practiceID string) (grantPractice, error) {
	var p grantPractice
	var granted sql.NullInt64
	err := tx.QueryRowContext(ctx,
		`SELECT practice_name, staff_count, credits, granted_credits, granted_act_id FROM ops_founding_grant_practice($1)`, practiceID,
	).Scan(&p.PracticeName, &p.StaffCount, &p.Credits, &granted, &p.grantedActID)
	if errors.Is(err, sql.ErrNoRows) {
		return p, errNoPractice
	}
	if err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return p, fmt.Errorf("read practice: %w", err)
	}
	if granted.Valid {
		n := int(granted.Int64)
		p.GrantedCredits = &n
	}
	return p, nil
}

func beginGrantRead(w http.ResponseWriter, r *http.Request, db *sql.DB, practiceID string) (*sql.Tx, grantPractice, bool) {
	tx, err := db.BeginTx(r.Context(), nil)
	if err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		writeInternal(w)
		return nil, grantPractice{}, false
	}
	p, err := readGrantPractice(r.Context(), tx, practiceID)
	if errors.Is(err, errNoPractice) {
		_ = tx.Rollback()
		apierr.Write(w, http.StatusNotFound, apierr.CodeNotFound, msgPracticeNotFound, nil)
		return nil, p, false
	}
	if err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		_ = tx.Rollback()
		writeInternal(w)
		return nil, p, false
	}
	return tx, p, true
}

func foundingGrantCheckRead(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		practiceID, ok := practiceIDFrom(w, r)
		if !ok {
			return
		}
		tx, p, ok := beginGrantRead(w, r, db, practiceID)
		if !ok {
			return
		}
		_ = tx.Rollback()
		apierr.WriteJSON(w, http.StatusOK, p.foundingGrantCheck)
	}
}

func grantedUnder(ctx context.Context, tx *sql.Tx, actID, requestKey string) (foundingGrantActDetails, bool, error) {
	var raw []byte
	if err := tx.QueryRowContext(ctx, `SELECT details FROM operator_acts WHERE id = $1`, actID).Scan(&raw); err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return foundingGrantActDetails{}, false, fmt.Errorf("read grant act: %w", err)
	}
	var details foundingGrantActDetails
	_ = json.Unmarshal(raw, &details)
	return details, details.RequestKey == requestKey, nil
}

func foundingGrantAct(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		practiceID, ok := practiceIDFrom(w, r)
		if !ok {
			return
		}
		requestKey, ok := requestKeyOf(w, r, "founding grant")
		if !ok {
			return
		}
		if !decodeDeclared(w, r, map[string]bool{}, msgNoGrantFields, &struct{}{}) {
			return
		}

		tx, p, ok := beginGrantRead(w, r, db, practiceID)
		if !ok {
			return
		}
		defer func() { _ = tx.Rollback() }()

		if p.GrantedCredits != nil {
			if p.grantedActID.Valid {
				details, same, err := grantedUnder(r.Context(), tx, p.grantedActID.String, requestKey)
				if err != nil {
					// coverage:ignore reason: DB failure, not exercised by unit tests
					writeInternal(w)
					return
				}
				if same {
					apierr.WriteJSON(w, http.StatusOK, foundingGrantResult{PracticeName: p.PracticeName, StaffCount: details.StaffCount, Credits: details.Credits})
					return
				}
			}
			apierr.Write(w, http.StatusConflict, apierr.CodeConflict, fmt.Sprintf(msgAlreadyGranted, *p.GrantedCredits), nil)
			return
		}
		if p.StaffCount == 0 {
			apierr.Write(w, http.StatusConflict, apierr.CodeConflict, msgNoStaff, nil)
			return
		}

		op := signedIn(r)
		now := clock.Now(r.Context())
		actID, err := recordAct(r.Context(), tx, op, actFoundingGrant, practiceSubject(practiceID),
			foundingGrantActDetails{StaffCount: p.StaffCount, Credits: p.Credits, RequestKey: requestKey}, nil, now)
		if err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		var written sql.NullInt64
		if err := tx.QueryRowContext(r.Context(), `SELECT ops_record_founding_grant($1, $2, $3, $4)`,
			practiceID, actID, p.Credits, now).Scan(&written); err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		if !written.Valid {
			// coverage:ignore reason: a second grant or a roster change committed between this request's read and its lock; the function's WHERE is what refuses it
			apierr.Write(w, http.StatusConflict, apierr.CodeConflict, msgRosterChanged, nil)
			return
		}

		diff, _ := json.Marshal(foundingGrantDiff{StaffCount: p.StaffCount, Credits: p.Credits})
		if err := activity.Record(r.Context(), tx, activity.Entry{
			PracticeID:  practiceID,
			SubjectKind: activity.SubjectPractice,
			SubjectID:   practiceID,
			Action:      string(activity.ActionFoundingGrantIssued),
			Diff:        diff,
			Actor:       activity.OperatorActor(actID),
			CreatedAt:   now,
		}); err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, foundingGrantResult{PracticeName: p.PracticeName, StaffCount: p.StaffCount, Credits: p.Credits})
	}
}
