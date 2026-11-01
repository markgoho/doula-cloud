package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	// The image carries no zoneinfo, and the read's months are New York's.
	_ "time/tzdata"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
)

const monthLayout = "2006-01"

const msgMonthNotClosed = "Choose a month that has ended, written as YYYY-MM."

type practiceMonth struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	CreditsBought int    `json:"creditsBought"`
	CreditsSpent  int    `json:"creditsSpent"`
	BoughtBefore  bool   `json:"boughtBefore"`
}

type monthTotals struct {
	CreditsBought        int `json:"creditsBought"`
	CreditsSpent         int `json:"creditsSpent"`
	PracticesBoughtAgain int `json:"practicesBoughtAgain"`
}

type monthlyCreditsRead struct {
	Month     string          `json:"month"`
	Months    []string        `json:"months"`
	Practices []practiceMonth `json:"practices"`
	Totals    monthTotals     `json:"totals"`
}

func monthOf(t time.Time) time.Time {
	t = t.In(businessZone)
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, businessZone)
}

// The current month is refused: no figure for it is shown anywhere.
func monthlyCredits(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lastClosed := monthOf(clock.Now(r.Context())).AddDate(0, -1, 0)
		month := lastClosed
		if raw := r.URL.Query().Get("month"); raw != "" {
			parsed, err := time.ParseInLocation(monthLayout, raw, businessZone)
			if err != nil || parsed.After(lastClosed) {
				apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument, msgMonthNotClosed, map[string]string{"month": msgMonthNotClosed})
				return
			}
			month = parsed
		}

		tx, err := db.BeginTx(r.Context(), &sql.TxOptions{ReadOnly: true})
		if err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		defer func() { _ = tx.Rollback() }()

		read, err := readMonthlyCredits(r, tx, month, lastClosed)
		if err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, read)
	}
}

func readMonthlyCredits(r *http.Request, tx *sql.Tx, month, lastClosed time.Time) (monthlyCreditsRead, error) {
	read := monthlyCreditsRead{Month: month.Format(monthLayout), Practices: []practiceMonth{}}

	var firstSignup sql.NullTime
	if err := tx.QueryRowContext(r.Context(), `SELECT ops_first_signup_at()`).Scan(&firstSignup); err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return read, fmt.Errorf("read first signup: %w", err)
	}
	oldest := lastClosed
	if firstSignup.Valid && monthOf(firstSignup.Time).Before(lastClosed) {
		oldest = monthOf(firstSignup.Time)
	}
	for m := lastClosed; !m.Before(oldest); m = m.AddDate(0, -1, 0) {
		read.Months = append(read.Months, m.Format(monthLayout))
	}

	rows, err := tx.QueryContext(r.Context(),
		`SELECT id, name, credits_bought, credits_spent, bought_before FROM ops_monthly_credits($1, $2)`,
		month, month.AddDate(0, 1, 0))
	if err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return read, fmt.Errorf("read monthly credits: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var p practiceMonth
		if err := rows.Scan(&p.ID, &p.Name, &p.CreditsBought, &p.CreditsSpent, &p.BoughtBefore); err != nil {
			// coverage:ignore reason: scan failure on a well-typed query, not exercised by unit tests
			return read, fmt.Errorf("scan monthly credits: %w", err)
		}
		read.Practices = append(read.Practices, p)
		read.Totals.CreditsBought += p.CreditsBought
		read.Totals.CreditsSpent += p.CreditsSpent
		if p.BoughtBefore && p.CreditsBought > 0 {
			read.Totals.PracticesBoughtAgain++
		}
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return read, fmt.Errorf("read monthly credits: %w", err)
	}
	return read, nil
}
