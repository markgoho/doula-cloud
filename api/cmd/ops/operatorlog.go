package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/pagecursor"
)

// operatorLogPageSize is how many rows one page of the Operator log
// carries.
const operatorLogPageSize = 50

const (
	subjectPractice = "practice"
	subjectStaff    = "staff"
)

// logSubject is what a row acted on. Name is null for a Feedback piece,
// which has none, and for a Staff member no longer on file.
type logSubject struct {
	Kind    string  `json:"kind"`
	ID      string  `json:"id"`
	Name    *string `json:"name"`
	Deleted bool    `json:"deleted"`
}

type logOwner struct {
	ID   string  `json:"id"`
	Name *string `json:"name"`
}

// logRow is one operator_acts row as the Operator log shows it. Details
// pass through as recorded; ops says each action in words.
type logRow struct {
	ID          string          `json:"id"`
	At          time.Time       `json:"at"`
	Operator    string          `json:"operator"`
	Action      string          `json:"action"`
	Subject     *logSubject     `json:"subject"`
	Details     json.RawMessage `json:"details"`
	RequestedBy *logOwner       `json:"requestedBy,omitempty"`
}

type logPage struct {
	Items      []logRow `json:"items"`
	NextCursor *string  `json:"nextCursor,omitempty"`
	HasMore    bool     `json:"hasMore"`
}

// operatorLog answers one page of operator_acts, newest first. It shows
// names only, so it writes no read row (ADR-0062).
func operatorLog(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		afterAt, afterID, ok := listCursor(w, r)
		if !ok {
			return
		}
		tx, err := db.BeginTx(r.Context(), &sql.TxOptions{ReadOnly: true})
		if err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		defer func() { _ = tx.Rollback() }()
		page, err := readLogPage(r.Context(), tx, afterAt, afterID)
		if err == nil {
			err = nameSubjects(r.Context(), tx, page.Items)
		}
		if err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, page)
	}
}

func readLogPage(ctx context.Context, tx *sql.Tx, afterAt, afterID any) (logPage, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT a.id, a.created_at, o.name, a.action, a.subject_kind, a.subject_id, a.details
		   FROM operator_acts a
		   JOIN operators o ON o.id = a.operator_id
		  WHERE $1::timestamptz IS NULL OR (a.created_at, a.id) < ($1, $2::uuid)
		  ORDER BY a.created_at DESC, a.id DESC
		  LIMIT $3`, afterAt, afterID, operatorLogPageSize+1)
	if err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return logPage{}, fmt.Errorf("read operator log: %w", err)
	}
	defer func() { _ = rows.Close() }()
	page := logPage{Items: []logRow{}}
	for rows.Next() {
		var row logRow
		var kind, subjectID sql.NullString
		if err := rows.Scan(&row.ID, &row.At, &row.Operator, &row.Action, &kind, &subjectID, &row.Details); err != nil {
			// coverage:ignore reason: scan failure on a well-typed query, not exercised by unit tests
			return logPage{}, fmt.Errorf("scan operator log: %w", err)
		}
		if kind.Valid {
			row.Subject = &logSubject{Kind: kind.String, ID: subjectID.String}
		}
		if row.Action == "refund" {
			var d refundActDetails
			_ = json.Unmarshal(row.Details, &d)
			row.RequestedBy = &logOwner{ID: d.RequestedByStaffID}
		}
		page.Items = append(page.Items, row)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return logPage{}, fmt.Errorf("read operator log: %w", err)
	}
	if len(page.Items) > operatorLogPageSize {
		page.Items = page.Items[:operatorLogPageSize]
		page.HasMore = true
		last := page.Items[operatorLogPageSize-1]
		next := pagecursor.Encode(last.At, last.ID)
		page.NextCursor = &next
	}
	return page, nil
}

type nameKey struct{ kind, id string }

type subjectName struct {
	name    *string
	deleted bool
}

// nameSubjects fills each row's Practice and Staff names, and a refund's
// requesting Owner, through ops_operator_log_names (00152).
func nameSubjects(ctx context.Context, tx *sql.Tx, items []logRow) error {
	practiceIDs, staffIDs := []string{}, []string{}
	for _, row := range items {
		if row.Subject != nil && row.Subject.Kind == subjectPractice {
			practiceIDs = append(practiceIDs, row.Subject.ID)
		}
		if row.Subject != nil && row.Subject.Kind == subjectStaff {
			staffIDs = append(staffIDs, row.Subject.ID)
		}
		if row.RequestedBy != nil {
			staffIDs = append(staffIDs, row.RequestedBy.ID)
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT kind, id, name, deleted FROM ops_operator_log_names($1, $2)`,
		practiceIDs, staffIDs)
	if err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return fmt.Errorf("name operator log subjects: %w", err)
	}
	defer func() { _ = rows.Close() }()
	names := map[nameKey]subjectName{}
	for rows.Next() {
		var key nameKey
		var n subjectName
		if err := rows.Scan(&key.kind, &key.id, &n.name, &n.deleted); err != nil {
			// coverage:ignore reason: scan failure on a well-typed query, not exercised by unit tests
			return fmt.Errorf("scan operator log subject: %w", err)
		}
		names[key] = n
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return fmt.Errorf("name operator log subjects: %w", err)
	}
	for _, row := range items {
		if row.Subject != nil {
			n := names[nameKey{row.Subject.Kind, row.Subject.ID}]
			row.Subject.Name, row.Subject.Deleted = n.name, n.deleted
		}
		if row.RequestedBy != nil {
			row.RequestedBy.Name = names[nameKey{subjectStaff, row.RequestedBy.ID}].name
		}
	}
	return nil
}
