package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/pagecursor"
)

// dormancyNoticeYears is how long without contact makes a Practice's
// unspent balance worth writing to her about.
//
// Two years, not three: APL 1315(1-b) escheats the balance at three
// years' dormancy, and the due-diligence mailing has to reach the owner
// before that, not after. A year's warning is what turns an escheat into
// a Practice spending her own Credits.
const dormancyNoticeYears = 2

// dormantPractice is a Practice holding unspent Credits that nobody has
// been seen using: the row the annual balance notice and the
// December/January due-diligence mailings are addressed from.
type dormantPractice struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	SignedUpAt    time.Time  `json:"signedUpAt"`
	Balance       int        `json:"balance"`
	LastContactAt *time.Time `json:"lastContactAt"`
}

type dormantPage struct {
	Items      []dormantPractice `json:"items"`
	NextCursor *string           `json:"nextCursor,omitempty"`
	HasMore    bool              `json:"hasMore"`
}

// dormantList answers one page of the Practices holding Credits whose
// Staff nobody has seen for dormancyNoticeYears, newest signup first. A
// Practice nobody has ever seen counts. Read only: nothing writes a
// balance off. It shows nobody's contact, so it writes no read row.
func dormantList(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		afterAt, afterID, ok := listCursor(w, r)
		if !ok {
			return
		}
		now := clock.Now(r.Context()).In(businessZone)
		notSeenSince := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, businessZone).AddDate(-dormancyNoticeYears, 0, 0)

		tx, err := db.BeginTx(r.Context(), &sql.TxOptions{ReadOnly: true})
		if err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		defer func() { _ = tx.Rollback() }()

		page, err := readDormantPage(r, tx, notSeenSince, afterAt, afterID)
		if err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, page)
	}
}

func readDormantPage(r *http.Request, tx *sql.Tx, notSeenSince time.Time, afterAt, afterID any) (dormantPage, error) {
	rows, err := tx.QueryContext(r.Context(),
		`SELECT id, name, signed_up_at, balance, last_contact_at FROM ops_dormant_practices($1, $2, $3, $4)`,
		notSeenSince, afterAt, afterID, practicePageSize+1)
	if err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return dormantPage{}, fmt.Errorf("read dormant practices: %w", err)
	}
	defer func() { _ = rows.Close() }()
	page := dormantPage{Items: []dormantPractice{}}
	for rows.Next() {
		var p dormantPractice
		if err := rows.Scan(&p.ID, &p.Name, &p.SignedUpAt, &p.Balance, &p.LastContactAt); err != nil {
			// coverage:ignore reason: scan failure on a well-typed query, not exercised by unit tests
			return dormantPage{}, fmt.Errorf("scan dormant practice: %w", err)
		}
		page.Items = append(page.Items, p)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return dormantPage{}, fmt.Errorf("read dormant practices: %w", err)
	}
	if len(page.Items) > practicePageSize {
		page.Items = page.Items[:practicePageSize]
		page.HasMore = true
		last := page.Items[practicePageSize-1]
		next := pagecursor.Encode(last.SignedUpAt, last.ID)
		page.NextCursor = &next
	}
	return page, nil
}
