package staffauth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"doula-cloud/api/internal/pagecursor"
)

// workStateHistoryPageSize is the fixed number of changes one page
// carries. Deliberately small: a person's work state moves when she
// moves house or office, so a page of twenty is several careers for
// almost everyone, and the cursor exists for the case it is not
// (docs/api-design.md section 4).
const workStateHistoryPageSize = 20

// WorkStateChange is one entry in a Staff member's work state history:
// what she said, what it replaced, and when.
//
// PreviousWorkState is empty for the assertion made at onboarding --
// 00043 leaves previous_work_state NULL there, because there was no
// earlier value. The screen prints that as "Reported New York" and a
// real move as "Changed from New York to New Jersey"; they are different
// sentences and the empty string is what tells them apart.
//
// There is no actor field. Only the person herself may write a work
// state -- enforced by the shape of PUT /api/staff/work-state, which
// carries no staff id, and by 00044's policies -- so every row's actor
// is its subject and printing it would add a name, not an answer. A row
// where actor_staff_id differs from staff_id is the signal 00043 kept
// the column for; if a future ticket ever widens the write, this DTO
// grows an actor and the screen stops saying "self-reported".
type WorkStateChange struct {
	EventID           string    `json:"eventId"`
	PreviousWorkState string    `json:"previousWorkState,omitempty"`
	WorkState         string    `json:"workState"`
	CreatedAt         time.Time `json:"createdAt"`
}

// WorkStateHistory is docs/api-design.md section 4's envelope, plus the
// one fact the screen needs that is not an item.
//
// MemberSince is when this person's Membership at the current Practice
// was created. It is here because a contractor doula who asserted her
// work state at another Practice first carries those rows into this one
// -- 00043's table has no practice_id, and joining a second Practice
// writes no event -- so without it the screen would show assertions made
// elsewhere with nothing to say they were made elsewhere. With it, the
// screen marks every entry older than the Membership as made before she
// joined, which is the honest reading and the thing #459 asked for.
type WorkStateHistory struct {
	MemberSince time.Time         `json:"memberSince"`
	Items       []WorkStateChange `json:"items"`
	NextCursor  *string           `json:"nextCursor,omitempty"`
	HasMore     bool              `json:"hasMore"`
}

// ListWorkStateHistoryHandler serves the history behind one roster row's
// "Works from" value (#459). The roster shows the current value and the
// day it was asserted, which is the whole answer to "how did this get
// set?" only for a value that never moved; once it moves, the earlier
// assertion -- the one every Credit purchase before that date was
// apportioned on -- was nowhere on screen.
//
// Owner and Admin, matching the roster itself (ADR-0008's read table)
// and enforced by the role declaration on this route's GatedRouter
// mount, not inside this handler. No RLS policy is widened to serve it:
// staff_work_state_events_practice_visibility (00043) already admits a
// row whose subject holds a Membership at the current Practice, which is
// exactly this Practice's roster. Must be mounted behind Middleware.
func ListWorkStateHistoryHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, practiceID, ok := RequireTx(w, r)
		// coverage:ignore reason: Middleware always sets a tx before this handler runs
		if !ok {
			return
		}

		staffID := r.PathValue("staffId")
		if !ParseUUID(w, "staff", staffID) {
			return
		}

		// The Membership read is also the existence check: RLS scopes it
		// to the current Practice, so a staff id belonging to somebody
		// else's Practice is a 404 rather than an empty history that
		// would confirm the person exists.
		memberSince, err := membershipCreatedAt(r.Context(), tx, practiceID, staffID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				http.Error(w, "staff member not found", http.StatusNotFound)
				return
			}
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		var after *pagecursor.Cursor
		if raw := r.URL.Query().Get("cursor"); raw != "" {
			c, err := pagecursor.Decode(raw)
			if err != nil {
				http.Error(w, "invalid cursor", http.StatusBadRequest)
				return
			}
			after = &c
		}

		items, hasMore, err := listWorkStateChanges(r.Context(), tx, staffID, after)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
			return
		}

		resp := WorkStateHistory{MemberSince: memberSince, Items: items, HasMore: hasMore}
		if hasMore {
			next := pagecursor.Encode(items[len(items)-1].CreatedAt, items[len(items)-1].EventID)
			resp.NextCursor = &next
		}

		w.Header().Set("Content-Type", "application/json")
		// coverage:ignore reason: response encoding failure, not exercised by unit tests
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, MsgInternalError, http.StatusInternalServerError)
		}
	})
}

// membershipCreatedAt reads when this person joined the current Practice.
// practice_id is passed explicitly as well as being enforced by RLS: the
// query says what it means, and RLS is the second check that makes a
// mistake here return nothing rather than somebody else's row.
func membershipCreatedAt(ctx context.Context, tx *sql.Tx, practiceID, staffID string) (time.Time, error) {
	var createdAt time.Time
	err := tx.QueryRowContext(ctx,
		`SELECT created_at FROM practice_memberships
		 WHERE practice_id = $1 AND staff_id = $2`,
		practiceID, staffID,
	).Scan(&createdAt)
	if err != nil {
		return time.Time{}, fmt.Errorf("staffauth: read membership date: %w", err)
	}
	return createdAt, nil
}

// listWorkStateChangesQuery and listWorkStateChangesAfterQuery differ only
// in the cursor's WHERE clause and the LIMIT placeholder, so two static
// queries are simpler and safer than one built at runtime -- the shape
// message.listMessages already uses.
//
// previous_work_state IS DISTINCT FROM work_state is the whole
// changes-only filter, and it is one predicate rather than two because
// IS DISTINCT FROM treats the NULL a first assertion carries as a value:
// NULL vs 'NY' is distinct, so onboarding survives the filter, and
// 'NY' vs 'NY' is not, so a re-assertion does not.
//
// Re-assertions are excluded on purpose. Saving an unchanged value is a
// real act (#437 -- work_state_reported_at is the only staleness signal
// the design has), but it is already shown on the roster as the date
// beside the current value, and a Practice substantiating an ST-100
// needs what was true on the purchase date, which the changes alone
// answer completely. Printing every re-assertion would bury the moves
// that actually shifted the apportionment. Nothing is deleted: the rows
// stay in the table, and this is a decision about a screen.
const listWorkStateChangesQuery = `SELECT id, previous_work_state, work_state, created_at
	FROM staff_work_state_events
	WHERE staff_id = $1 AND previous_work_state IS DISTINCT FROM work_state
	ORDER BY created_at DESC, id DESC
	LIMIT $2`

const listWorkStateChangesAfterQuery = `SELECT id, previous_work_state, work_state, created_at
	FROM staff_work_state_events
	WHERE staff_id = $1 AND previous_work_state IS DISTINCT FROM work_state
	  AND (created_at, id) < ($2, $3)
	ORDER BY created_at DESC, id DESC
	LIMIT $4`

// listWorkStateChanges reads one page, newest first, and reports whether
// another follows. It asks for one row more than the page holds and drops
// it, so "is there more?" costs no second query.
func listWorkStateChanges(ctx context.Context, tx *sql.Tx, staffID string, after *pagecursor.Cursor) ([]WorkStateChange, bool, error) {
	limit := workStateHistoryPageSize + 1

	var rows *sql.Rows
	var err error
	if after == nil {
		rows, err = tx.QueryContext(ctx, listWorkStateChangesQuery, staffID, limit)
	} else {
		rows, err = tx.QueryContext(ctx, listWorkStateChangesAfterQuery, staffID, after.At, after.ID, limit)
	}
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return nil, false, fmt.Errorf("staffauth: query work state history: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := []WorkStateChange{}
	for rows.Next() {
		var c WorkStateChange
		var previous sql.NullString
		if err := rows.Scan(&c.EventID, &previous, &c.WorkState, &c.CreatedAt); err != nil {
			// coverage:ignore reason: scan failure on a well-typed query, not exercised by unit tests
			return nil, false, fmt.Errorf("staffauth: scan work state event: %w", err)
		}
		c.PreviousWorkState = previous.String
		items = append(items, c)
	}
	// coverage:ignore reason: row iteration failure, not exercised by unit tests
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("staffauth: iterate work state history: %w", err)
	}

	hasMore := len(items) > workStateHistoryPageSize
	if hasMore {
		items = items[:workStateHistoryPageSize]
	}
	return items, hasMore, nil
}
