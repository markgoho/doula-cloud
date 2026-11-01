package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/mail"
	"strings"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
)

const (
	msgLookupEmail     = "Enter the email address the person signs in with"
	msgLookupEmailForm = "Enter an email address in the correct format, like name@example.com"
	msgCorrectLookup   = "Correct the answer marked below."
)

type foundStaff struct {
	StaffID       string   `json:"staffId"`
	Name          string   `json:"name"`
	Email         string   `json:"email"`
	PracticeNames []string `json:"practiceNames"`
}

type staffLookupResult struct {
	Staff []foundStaff `json:"staff"`
}

type staffLookupRead struct {
	Email    string   `json:"email"`
	StaffIDs []string `json:"staffIds"`
}

func findStaff(ctx context.Context, tx *sql.Tx, email string) ([]foundStaff, error) {
	rows, err := tx.QueryContext(ctx, `SELECT staff_id, staff_name, staff_email, practice_name FROM ops_find_staff($1)`, email)
	if err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return nil, fmt.Errorf("find staff: %w", err)
	}
	defer func() { _ = rows.Close() }()
	found := []foundStaff{}
	for rows.Next() {
		var s foundStaff
		var practiceName sql.NullString
		if err := rows.Scan(&s.StaffID, &s.Name, &s.Email, &practiceName); err != nil {
			// coverage:ignore reason: scan failure on a well-typed query, not exercised by unit tests
			return nil, fmt.Errorf("scan staff: %w", err)
		}
		if n := len(found); n == 0 || found[n-1].StaffID != s.StaffID {
			s.PracticeNames = []string{}
			found = append(found, s)
		}
		if practiceName.Valid {
			last := &found[len(found)-1]
			last.PracticeNames = append(last.PracticeNames, practiceName.String)
		}
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return nil, fmt.Errorf("find staff: %w", err)
	}
	return found, nil
}

// It shows her contact, so it is recorded as a staff_lookup row in the same transaction.
func staffLookup(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		email := strings.TrimSpace(r.URL.Query().Get("email"))
		if email == "" {
			apierr.Write(w, http.StatusUnprocessableEntity, apierr.CodeInvalidArgument, msgCorrectLookup, map[string]string{"email": msgLookupEmail})
			return
		}
		if _, err := mail.ParseAddress(email); err != nil || strings.ContainsAny(email, " <>") {
			apierr.Write(w, http.StatusUnprocessableEntity, apierr.CodeInvalidArgument, msgCorrectLookup, map[string]string{"email": msgLookupEmailForm})
			return
		}

		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		defer func() { _ = tx.Rollback() }()

		found, err := findStaff(r.Context(), tx, email)
		if err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		if len(found) > 0 {
			read := staffLookupRead{Email: email, StaffIDs: []string{}}
			for _, s := range found {
				read.StaffIDs = append(read.StaffIDs, s.StaffID)
			}
			if _, err := recordAct(r.Context(), tx, signedIn(r), actStaffLookup, nil, read, nil, clock.Now(r.Context())); err != nil {
				// coverage:ignore reason: DB failure, not exercised by unit tests
				writeInternal(w)
				return
			}
		}
		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, staffLookupResult{Staff: found})
	}
}
