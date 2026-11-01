package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clock"
	"doula-cloud/api/internal/feedback"
	"doula-cloud/api/internal/pagecursor"
)

// feedbackPageSize is how many pieces one page of the Feedback list
// carries.
const feedbackPageSize = 50

const (
	msgFeedbackNotFound = "There is no piece of Feedback at this address. It may have been erased, or destroyed 24 months after it was sent."
	msgBadIssueFilter   = "The list can be narrowed only to Feedback whose issue did not open."
)

// feedbackSummary is one row of the Feedback list. It names nobody: the
// list is for finding a piece, and opening one is the recorded read.
type feedbackSummary struct {
	ID      string               `json:"id"`
	Kind    string               `json:"kind"`
	RouteID string               `json:"routeId"`
	SentAt  time.Time            `json:"sentAt"`
	Issue   feedback.IssueStatus `json:"issue"`
}

type feedbackPage struct {
	Items      []feedbackSummary `json:"items"`
	NextCursor *string           `json:"nextCursor,omitempty"`
	HasMore    bool              `json:"hasMore"`
}

type feedbackSender struct {
	Kind  string `json:"kind"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// feedbackPiece is every stored field of one piece but client_id, with
// its sender named (ADR-0062's Feedback exception).
type feedbackPiece struct {
	ID           string               `json:"id"`
	Kind         string               `json:"kind"`
	Text         string               `json:"text"`
	PageURL      string               `json:"pageUrl"`
	RouteID      string               `json:"routeId"`
	AppBuild     string               `json:"appBuild"`
	ScreenWidth  int                  `json:"screenWidth"`
	Browser      string               `json:"browser"`
	SentAt       time.Time            `json:"sentAt"`
	Role         string               `json:"role"`
	PracticeName string               `json:"practiceName,omitempty"`
	Sender       feedbackSender       `json:"sender"`
	Issue        feedback.IssueStatus `json:"issue"`
}

// feedbackList answers one page of every piece, or with ?issue=unopened
// of the pieces whose issue open has failed. It shows nobody's name, so
// it writes no read row.
func feedbackList(db *sql.DB, repo string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filter := r.URL.Query().Get("issue")
		if filter != "" && filter != "unopened" {
			apierr.Write(w, http.StatusBadRequest, apierr.CodeInvalidArgument, msgBadIssueFilter, nil)
			return
		}
		afterAt, afterID, ok := listCursor(w, r)
		if !ok {
			return
		}
		page, err := readFeedbackPage(r, db, repo, filter == "unopened", afterAt, afterID)
		if err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, page)
	}
}

func readFeedbackPage(r *http.Request, db *sql.DB, repo string, unopened bool, afterAt, afterID any) (feedbackPage, error) {
	rows, err := db.QueryContext(r.Context(),
		`SELECT id, kind, route_id, sent_at, issue_number, job_status, attempt_count, last_error, dead_letter_reason
		   FROM ops_feedback_list($1, $2, $3, $4)`, unopened, afterAt, afterID, feedbackPageSize+1)
	if err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return feedbackPage{}, fmt.Errorf("read feedback list: %w", err)
	}
	defer func() { _ = rows.Close() }()
	page := feedbackPage{Items: []feedbackSummary{}}
	for rows.Next() {
		var s feedbackSummary
		var number sql.NullInt64
		var job feedback.OpenJob
		if err := rows.Scan(&s.ID, &s.Kind, &s.RouteID, &s.SentAt, &number,
			&job.Status, &job.Attempts, &job.LastError, &job.DeadLetterReason); err != nil {
			// coverage:ignore reason: scan failure on a well-typed query, not exercised by unit tests
			return feedbackPage{}, fmt.Errorf("scan feedback: %w", err)
		}
		s.Issue = feedback.IssueOf(repo, number, job)
		page.Items = append(page.Items, s)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: DB failure, not exercised by unit tests
		return feedbackPage{}, fmt.Errorf("read feedback list: %w", err)
	}
	if len(page.Items) > feedbackPageSize {
		page.Items = page.Items[:feedbackPageSize]
		page.HasMore = true
		last := page.Items[feedbackPageSize-1]
		next := pagecursor.Encode(last.SentAt, last.ID)
		page.NextCursor = &next
	}
	return page, nil
}

// feedbackRead answers one piece and records the read as a feedback_read
// row in the same transaction, so a piece is never shown without its
// record. The row holds the piece's id and the time, never its content.
func feedbackRead(db *sql.DB, repo string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("feedbackId")
		if uuid.Validate(id) != nil {
			apierr.Write(w, http.StatusNotFound, apierr.CodeNotFound, msgFeedbackNotFound, nil)
			return
		}
		tx, err := db.BeginTx(r.Context(), nil)
		if err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		defer func() { _ = tx.Rollback() }()

		piece, err := readFeedbackPiece(r, tx, repo, id)
		if errors.Is(err, sql.ErrNoRows) {
			apierr.Write(w, http.StatusNotFound, apierr.CodeNotFound, msgFeedbackNotFound, nil)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		if _, err := recordAct(r.Context(), tx, signedIn(r), actFeedbackRead, feedbackSubject(piece.ID), nil, nil, clock.Now(r.Context())); err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		if err := tx.Commit(); err != nil {
			// coverage:ignore reason: DB failure, not exercised by unit tests
			writeInternal(w)
			return
		}
		apierr.WriteJSON(w, http.StatusOK, piece)
	}
}

func readFeedbackPiece(r *http.Request, tx *sql.Tx, repo, id string) (feedbackPiece, error) {
	var p feedbackPiece
	var roles, name, email, practice sql.NullString
	var portal bool
	var number sql.NullInt64
	var job feedback.OpenJob
	err := tx.QueryRowContext(r.Context(),
		`SELECT id, kind, text, page_url, route_id, app_build, screen_width, browser, sent_at, roles, from_portal,
		        issue_number, sender_name, sender_email, practice_name, job_status, attempt_count, last_error, dead_letter_reason
		   FROM ops_feedback_piece($1)`, id,
	).Scan(&p.ID, &p.Kind, &p.Text, &p.PageURL, &p.RouteID, &p.AppBuild, &p.ScreenWidth, &p.Browser, &p.SentAt,
		&roles, &portal, &number, &name, &email, &practice,
		&job.Status, &job.Attempts, &job.LastError, &job.DeadLetterReason)
	if err != nil {
		return p, fmt.Errorf("read feedback piece: %w", err)
	}
	p.Role = feedback.RoleText(portal, roles)
	p.PracticeName = practice.String
	p.Sender = feedbackSender{Kind: "staff", Name: name.String, Email: email.String}
	if portal {
		p.Sender.Kind = "client"
	}
	p.Issue = feedback.IssueOf(repo, number, job)
	return p, nil
}
