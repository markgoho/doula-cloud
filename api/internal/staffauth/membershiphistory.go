package staffauth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"doula-cloud/api/internal/activity"
	"doula-cloud/api/internal/activitypage"
	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/pagecursor"
)

// membershipHistoryPageSize is the fixed number of entries one page
// carries, the same twenty ListWorkStateHistoryHandler uses and for the
// same reason (docs/api-design.md section 4): a Membership's roles and
// employment type move rarely, so twenty is most people's whole time at
// a Practice, and the cursor exists for the Practice where it is not.
const membershipHistoryPageSize = 20

// MembershipChange is one entry in a Membership's history: what
// happened, who did it, when, and -- for the two actions that move a
// fact -- what that fact was before and after.
//
// The values are the stored ones (`owner`, `contractor`), never display
// words. The one place a `practice_role` or an `employment_type` is
// given the word a person reads is the app's own roles.ts (#262), gated
// by roles.usage.spec.ts; a label map here would be exactly the second
// copy that ticket closed, and it would put the Go side in the business
// of the product's vocabulary, which ADR-0005 keeps in one place.
//
// Every before/after field is omitted when empty, and empty means "this
// fact did not move on this event", never "this fact became blank" --
// see membershipDiff. A `joined` event carries Roles and EmploymentType
// with no previous, a `removed` event carries the previous with no
// after, a `roles_changed` carries both role fields and neither
// employment one, and `sessions_ended` (#473) carries none of the four:
// it names an act performed against the Membership rather than a change
// to what the Membership is.
type MembershipChange struct {
	EventID   string `json:"eventId"`
	Action    string `json:"action"`
	ActorName string `json:"actorName"`

	PreviousRoles          []string `json:"previousRoles,omitempty"`
	Roles                  []string `json:"roles,omitempty"`
	PreviousEmploymentType string   `json:"previousEmploymentType,omitempty"`
	EmploymentType         string   `json:"employmentType,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
}

// MembershipHistory is docs/api-design.md section 4's envelope, and only
// that.
//
// No MemberSince, which WorkStateHistory carries: a work state event has
// no practice_id, so that reader has to date the Membership to mark the
// assertions made before this Practice ever met her. Every row here is
// scoped to one Practice by activity.practice_id, so nothing in this
// history was recorded anywhere else and there is nothing to mark.
type MembershipHistory struct {
	Items      []MembershipChange `json:"items"`
	NextCursor *string            `json:"nextCursor,omitempty"`
	HasMore    bool               `json:"hasMore"`
}

// ListMembershipHistoryHandler serves the history behind one roster row
// (#872). Every path that creates or changes a Membership has recorded
// itself since the beginning -- signup, an accepted Invitation, an
// Owner's edit, a removal, a login deletion, an ended session -- and
// until this handler nothing read a single one of those rows back, so
// "when did she become an Admin, and who made her one?" had an answer
// in the database and nowhere a person could reach. The roster shows
// what somebody is today; this is what is behind it.
//
// Owner and Admin, matching the roster itself (ADR-0008's read table)
// and enforced by the role declaration on this route's GatedRouter
// mount, not inside this handler -- the same boundary
// ListWorkStateHistoryHandler stands behind. No RLS policy is widened to
// serve it: activity_practice_visibility (00051) already admits a row
// whose practice_id is the current Practice, and the query filters to
// this Practice again in the app layer, so a bug in either alone cannot
// reach another Practice's rows. Must be mounted behind Middleware.
func ListMembershipHistoryHandler() http.Handler {
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

		// The Membership read is the existence check, exactly as it is
		// for the work state history: RLS scopes it to the current
		// Practice, so a staff id belonging to somebody else's Practice
		// is a 404 rather than an empty history that would confirm the
		// person exists.
		if _, err := membershipCreatedAt(r.Context(), tx, practiceID, staffID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				apierr.WriteError(w, "staff member not found", http.StatusNotFound)
				return
			}
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		var after *pagecursor.Cursor
		if raw := r.URL.Query().Get("cursor"); raw != "" {
			c, err := pagecursor.Decode(raw)
			if err != nil {
				apierr.WriteError(w, "invalid cursor", http.StatusBadRequest)
				return
			}
			after = &c
		}

		page, err := listMembershipChanges(r.Context(), tx, practiceID, staffID, after)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		apierr.WriteJSON(w, http.StatusOK, MembershipHistory{
			Items:      page.Items,
			NextCursor: page.NextCursor,
			HasMore:    page.HasMore,
		})
	})
}

// membershipProjection is this package's own half of a subject-scoped
// activity row (#1150): the diff, and nothing else. activitypage owns
// the Practice and subject scoping, the newest-first (created_at, id)
// cursor, the sentinel row that answers "is there more?" without a
// second query, the page trim, and the actor's resolved name.
//
// There is no changes-only filter, which the work state history needs
// and this one does not. A Membership event is only ever written when
// something actually moved: UpdateMembershipHandler records one event
// per axis that changed and nothing at all for a no-op edit, and the
// other four actions each name an act that happened once. Every row is
// already a change, so a filter could only ever drop a real one.
//
// EventID is the activity row's own id, which is why the shared builder
// is handed the row rather than only the fields a generic feed would
// show: this DTO names the event a reader can come back to.
//
// What this gives up, said rather than left to be discovered: the shared
// reader always joins clients for the actor's name, and a Membership has
// no Client writer and can have none -- a Membership is a person's
// standing at a Practice, and nothing a Client does touches one. So this
// read now carries one LEFT JOIN that never matches. That is the trade
// #1150 made deliberately: this package used to avoid the join by
// writing its own two-branch actor rule, and a second spelling of that
// rule is exactly how the Engagement ledger came to render a departed
// colleague as an empty cell while this one named her.
var membershipProjection = activitypage.Projection[MembershipChange]{
	Columns: []string{"a.diff"},
	Row: func() ([]any, func(activitypage.Row) MembershipChange) {
		var diff []byte
		return []any{&diff}, func(r activitypage.Row) MembershipChange {
			c := MembershipChange{
				EventID:   r.ID,
				Action:    r.Action,
				ActorName: r.ActorName,
				CreatedAt: r.CreatedAt,
			}
			applyMembershipDiff(&c, diff)
			return c
		}
	},
}

// listMembershipChanges reads one page, newest first, and reports
// whether another follows.
func listMembershipChanges(ctx context.Context, tx *sql.Tx, practiceID, staffID string, after *pagecursor.Cursor) (activitypage.Page[MembershipChange], error) {
	return activitypage.List(ctx, tx, activitypage.Query{
		PracticeID:  practiceID,
		SubjectKind: activity.SubjectMembership,
		SubjectID:   staffID,
		After:       after,
		PageSize:    membershipHistoryPageSize,
	}, membershipProjection)
}

// applyMembershipDiff unpacks the diff column into the entry's four
// before/after fields, through the same membershipDiff type
// RecordMembershipEvent marshals -- so a key can never be renamed on one
// side alone.
//
// The CSV the diff stores is split into a real array here rather than
// handed to the app as a string: the app's job is to give each stored
// role its word (roles.ts's rolesLabel), and asking it to parse a
// Postgres-shaped string first would put a storage detail on a screen's
// side of the wire, which docs/api-design.md section 2 rules out. An
// empty side stays nil rather than becoming a one-element array holding
// the empty string, which is what strings.Split would make of it.
//
// It reports no error. The diff column is jsonb, written only by
// RecordMembershipEvent's own json.Marshal of this very type, so there
// is no row a decode can fail on -- and if a decode somehow did fail,
// the entry still carries the three facts a reader most needs (what
// happened, who did it, when), so refusing the whole page over one
// unreadable diff would answer a smaller question with a bigger
// failure. The fields simply stay empty, which the DTO already treats
// as "this fact did not move".
func applyMembershipDiff(c *MembershipChange, diff []byte) {
	var d membershipDiff
	if err := json.Unmarshal(diff, &d); err != nil {
		return
	}
	if d.Roles.From != "" {
		c.PreviousRoles = splitRoles(d.Roles.From)
	}
	if d.Roles.To != "" {
		c.Roles = splitRoles(d.Roles.To)
	}
	c.PreviousEmploymentType = d.EmploymentType.From
	c.EmploymentType = d.EmploymentType.To
}
