package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/google/uuid"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
)

const (
	msgStaffNotFound   = "There is no Staff member at this address."
	msgNoClearFields   = "Send no fields with the reset. Who it is for comes from the address, and who does it from your sign-in."
	msgIdentityRefused = "Identity Platform did not reset two-factor authentication, so nothing has changed. Try again."
)

type secondFactorStaff struct {
	StaffName     string   `json:"staffName"`
	PracticeNames []string `json:"practiceNames"`
	practiceIDs   []string
}

type secondFactorActDetails struct {
	RequestKey string `json:"requestKey"`
}

var errNoStaff = errors.New("no such staff member")

func staffIDFrom(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("staffId")
	if _, err := uuid.Parse(id); err != nil {
		apierr.Write(w, http.StatusNotFound, apierr.CodeNotFound, msgStaffNotFound, nil)
		return "", false
	}
	return id, true
}

func readSecondFactorStaff(ctx context.Context, tx *sql.Tx, staffID string) (secondFactorStaff, error) {
	staff := secondFactorStaff{PracticeNames: []string{}}
	rows, err := tx.QueryContext(ctx, `SELECT staff_name, practice_id, practice_name FROM ops_second_factor_staff($1)`, staffID)
	if err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return staff, fmt.Errorf("read staff: %w", err)
	}
	defer func() { _ = rows.Close() }()
	found := false
	for rows.Next() {
		var practiceID, practiceName sql.NullString
		if err := rows.Scan(&staff.StaffName, &practiceID, &practiceName); err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			return staff, fmt.Errorf("scan staff: %w", err)
		}
		found = true
		if practiceID.Valid {
			staff.practiceIDs = append(staff.practiceIDs, practiceID.String)
			staff.PracticeNames = append(staff.PracticeNames, practiceName.String)
		}
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return staff, fmt.Errorf("read staff: %w", err)
	}
	if !found {
		return staff, errNoStaff
	}
	return staff, nil
}

func writeStaffRead(w http.ResponseWriter, err error) {
	if errors.Is(err, errNoStaff) {
		apierr.Write(w, http.StatusNotFound, apierr.CodeNotFound, msgStaffNotFound, nil)
		return
	}
	// coverage:ignore reason: DB failure, not exercised by unit tests
	writeInternal(w)
}

func secondFactorCheckRead(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		staffID, ok := staffIDFrom(w, r)
		if !ok {
			return
		}
		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		defer func() { _ = tx.Rollback() }()
		staff, err := readSecondFactorStaff(r.Context(), tx, staffID)
		if err != nil {
			writeStaffRead(w, err)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, staff)
	}
}

func clearedUnder(ctx context.Context, tx *sql.Tx, staffID, requestKey string) (bool, error) {
	var n int
	if err := tx.QueryRowContext(ctx,
		`SELECT count(*) FROM operator_acts
		  WHERE action = 'second_factor_cleared' AND subject_kind = 'staff' AND subject_id = $1 AND details->>'requestKey' = $2`,
		staffID, requestKey).Scan(&n); err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return false, fmt.Errorf("read earlier clear: %w", err)
	}
	return n > 0, nil
}

func secondFactorClearAct(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		staffID, ok := staffIDFrom(w, r)
		if !ok {
			return
		}
		requestKey, ok := requestKeyOf(w, r, "reset")
		if !ok {
			return
		}
		if !decodeDeclared(w, r, map[string]bool{}, msgNoClearFields, &struct{}{}) {
			return
		}

		op := signedIn(r)
		tx, err := d.DB.BeginTx(r.Context(), nil)
		if err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		defer func() { _ = tx.Rollback() }()

		if err := spendUnlessReplay(r.Context(), tx, op, actSecondFactorCleared, requestKey); err != nil {
			// coverage:ignore reason: a DB failure, or a second act spending the stamp between requireStepUp's read and this one; stepup_test.go covers spendStepUp's refusal
			writeStepUpRequired(w)
			return
		}
		staff, err := readSecondFactorStaff(r.Context(), tx, staffID)
		if err != nil {
			writeStaffRead(w, err)
			return
		}
		done, err := clearedUnder(r.Context(), tx, staffID, requestKey)
		if err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		if !done && !clearSecondFactor(w, r, d, tx, op, staffID, requestKey, staff) {
			return
		}
		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, staff)
	}
}

func clearSecondFactor(w http.ResponseWriter, r *http.Request, d Deps, tx *sql.Tx, op operator, staffID, requestKey string, staff secondFactorStaff) bool {
	ctx := r.Context()
	now := clock.Now(ctx)
	actID, err := recordAct(ctx, tx, op, actSecondFactorCleared, staffSubject(staffID), secondFactorActDetails{RequestKey: requestKey}, op.StepUpAt, now)
	if err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		writeInternal(w)
		return false
	}
	var identityUID sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT ops_record_second_factor_cleared($1, $2, $3)`,
		staffID, actID, now).Scan(&identityUID); err != nil || !identityUID.Valid {
		// coverage:ignore reason: DB failure, or a login deleted between this request's read and its write
		writeInternal(w)
		return false
	}
	if err := d.Accounts.ClearSecondFactors(ctx, identityUID.String); err != nil {
		log.Printf("doula-ops: reset two-factor authentication: %v", err)
		apierr.Write(w, http.StatusBadGateway, apierr.CodeInternal, msgIdentityRefused, nil)
		return false
	}
	for _, practiceID := range staff.practiceIDs {
		if err := activity.Record(ctx, tx, activity.Entry{
			PracticeID:  practiceID,
			SubjectKind: activity.SubjectMembership,
			SubjectID:   staffID,
			Action:      string(activity.ActionSecondFactorCleared),
			Actor:       activity.OperatorActor(actID),
			CreatedAt:   now,
		}); err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return false
		}
	}
	return true
}
