package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/pagecursor"
)

// practicePageSize is how many Practices one page of either list carries.
const practicePageSize = 50

const msgBadCursor = "This page of the list cannot be found. Start from the first page."

type practiceOwner struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type practiceConnect struct {
	Connected       bool       `json:"connected"`
	CardPayments    string     `json:"cardPayments"`
	Payouts         string     `json:"payouts"`
	RequirementsDue []string   `json:"requirementsDue"`
	UpdatedAt       *time.Time `json:"updatedAt"`
}

// practiceGrant names its giver only when ops gave it; a grant given
// before ops named its giver in a column that is gone.
type practiceGrant struct {
	Credits int        `json:"credits"`
	GivenBy *string    `json:"givenBy"`
	GivenAt *time.Time `json:"givenAt"`
}

type practiceDeletion struct {
	RequestedAt *time.Time `json:"requestedAt"`
	FinalizesAt *time.Time `json:"finalizesAt"`
	DeletedAt   *time.Time `json:"deletedAt"`
}

// practiceRow is one Practice as either list prints it (ADR-0062). Source
// is always null until #1393 records one.
type practiceRow struct {
	ID                 string            `json:"id"`
	Name               string            `json:"name"`
	SignedUpAt         time.Time         `json:"signedUpAt"`
	Owners             []practiceOwner   `json:"owners"`
	StaffCount         int               `json:"staffCount"`
	StripeConnect      practiceConnect   `json:"stripeConnect"`
	CreditsHeld        int               `json:"creditsHeld"`
	EngagementsStarted int               `json:"engagementsStarted"`
	FoundingGrant      *practiceGrant    `json:"foundingGrant"`
	Deletion           *practiceDeletion `json:"deletion"`
	Source             *string           `json:"source"`
	OpenFeedback       int               `json:"openFeedback"`
}

type practicePage struct {
	Items      []practiceRow `json:"items"`
	NextCursor *string       `json:"nextCursor,omitempty"`
	HasMore    bool          `json:"hasMore"`
}

// ownerContactRead is the details of the read row a list page writes:
// which list, and the Practices whose Owners it showed.
type ownerContactRead struct {
	List        string   `json:"list"`
	PracticeIDs []string `json:"practiceIds"`
}

func scanPractice(rows *sql.Rows) (practiceRow, error) {
	var p practiceRow
	var owners []byte
	var requirements []byte
	var grantCredits sql.NullInt64
	var grantBy sql.NullString
	var grantAt sql.NullTime
	var d practiceDeletion
	err := rows.Scan(&p.ID, &p.Name, &p.SignedUpAt, &owners, &p.StaffCount,
		&p.StripeConnect.Connected, &p.StripeConnect.CardPayments, &p.StripeConnect.Payouts,
		&requirements, &p.StripeConnect.UpdatedAt,
		&p.CreditsHeld, &p.EngagementsStarted, &grantCredits, &grantBy, &grantAt,
		&d.RequestedAt, &d.FinalizesAt, &d.DeletedAt, &p.OpenFeedback)
	if err != nil {
		// coverage:ignore reason: scan failure on a well-typed query, not exercised by unit tests
		return p, fmt.Errorf("scan practice: %w", err)
	}
	_ = json.Unmarshal(owners, &p.Owners)
	_ = json.Unmarshal(requirements, &p.StripeConnect.RequirementsDue)
	if grantCredits.Valid {
		p.FoundingGrant = &practiceGrant{Credits: int(grantCredits.Int64), GivenAt: &grantAt.Time}
		if grantBy.Valid {
			p.FoundingGrant.GivenBy = &grantBy.String
		}
	}
	if d.RequestedAt != nil || d.DeletedAt != nil {
		p.Deletion = &d
	}
	return p, nil
}

// practiceList answers one page of the live list or the Deleted list,
// newest signup first. Both show Owners' contacts, so the page is
// recorded as an owner_contact_read row in the same transaction.
func practiceList(db *sql.DB, deleted bool) http.HandlerFunc {
	list := "practices"
	if deleted {
		list = "deleted"
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var afterAt, afterID any
		if raw := r.URL.Query().Get("cursor"); raw != "" {
			c, err := pagecursor.Decode(raw)
			if err == nil {
				_, err = uuid.Parse(c.ID)
			}
			if err != nil {
				apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument, msgBadCursor, nil)
				return
			}
			afterAt, afterID = c.At, c.ID
		}

		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		defer func() { _ = tx.Rollback() }()

		page, err := readPracticePage(r, tx, deleted, afterAt, afterID)
		if err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		read := ownerContactRead{List: list, PracticeIDs: []string{}}
		for _, p := range page.Items {
			read.PracticeIDs = append(read.PracticeIDs, p.ID)
		}
		details, _ := json.Marshal(read)
		if _, err := tx.ExecContext(r.Context(),
			`INSERT INTO operator_acts (operator_id, action, details, created_at) VALUES ($1, 'owner_contact_read', $2, $3)`,
			signedIn(r).ID, details, clock.Now(r.Context())); err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, page)
	}
}

func readPracticePage(r *http.Request, tx *sql.Tx, deleted bool, afterAt, afterID any) (practicePage, error) {
	rows, err := tx.QueryContext(r.Context(),
		`SELECT id, name, signed_up_at, owners, staff_count, connected, card_payments_status, payouts_status,
		        requirements_due, connect_updated_at, credits_held, engagements_started, grant_credits, grant_given_by,
		        grant_given_at, deletion_requested_at, deletion_finalize_at, deleted_at, open_feedback
		   FROM ops_practice_list($1, $2, $3, $4)`, deleted, afterAt, afterID, practicePageSize+1)
	if err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return practicePage{}, fmt.Errorf("read practice list: %w", err)
	}
	defer func() { _ = rows.Close() }()
	page := practicePage{Items: []practiceRow{}}
	for rows.Next() {
		p, err := scanPractice(rows)
		if err != nil {
			// coverage:ignore reason: scan failure on a well-typed query, not exercised by unit tests
			return practicePage{}, err
		}
		page.Items = append(page.Items, p)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return practicePage{}, fmt.Errorf("read practice list: %w", err)
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
