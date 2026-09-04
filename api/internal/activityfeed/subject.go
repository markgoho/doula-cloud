package activityfeed

import (
	"context"
	"database/sql"
	"fmt"

	"doula-cloud/api/internal/pagecursor"
)

// ListForSubject reads one page of subjectKind/subjectID's activity,
// scoped to practiceID, newest first, cursor-paginated -- #486's Key
// interfaces: "a record-scoped variant taking a subject kind and subject
// id, following the shape the existing per-Client and per-Engagement
// readers already use" (client.mergedHistory and
// engagement.listEngagementActivity).
//
// It applies no access gate of its own: the caller must already know the
// reader may see subjectID at all before calling this -- staffauth's
// activitygate.CanAccessSubject, or a Client-portal ownership check like
// clientauth.Middleware's own -- the same division of labor
// engagement.ListActivityHandler already draws between its own 404 gate
// and its SQL money filter. It applies no ADR-0008 money filter either:
// that tier is a Staff-role concept activitygate.CanSeeAction/
// RestrictedActions already model, and does not apply to every caller of
// this function (portal.ActivityHandler's Client reads her own money in
// full, per CONTEXT.md's Activity entry).
//
// excludedActionsNotIn is an optional SQL fragment of quoted action
// literals (e.g. "'offer_sent', 'offer_accepted'"), built once by the
// caller from its own internal action constants -- never request input,
// the same shape engagement.moneyActionsNotIn already is -- or the empty
// string to exclude nothing.
func ListForSubject(ctx context.Context, tx *sql.Tx, practiceID, subjectKind, subjectID, excludedActionsNotIn string, after *pagecursor.Cursor, pageSize int) (ListResponse, error) {
	exclusion := "TRUE"
	if excludedActionsNotIn != "" {
		exclusion = fmt.Sprintf("a.action NOT IN (%s)", excludedActionsNotIn) //nolint:gosec // exclusion is built only from the caller's own internal action constants, per this function's own doc comment, never request input
	}

	var rows *sql.Rows
	var err error
	if after == nil {
		query := fmt.Sprintf(listForSubjectQueryTemplate, exclusion) //nolint:gosec // exclusion is a package-internal string built above, not request input
		rows, err = tx.QueryContext(ctx, query, practiceID, subjectKind, subjectID, pageSize+1)
	} else {
		query := fmt.Sprintf(listForSubjectAfterQueryTemplate, exclusion) //nolint:gosec // exclusion is a package-internal string built above, not request input
		rows, err = tx.QueryContext(ctx, query, practiceID, subjectKind, subjectID, after.At, after.ID, pageSize+1)
	}
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return ListResponse{}, fmt.Errorf("activityfeed: query subject activity: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := []rawEntry{}
	for rows.Next() {
		var row rawEntry
		var actorKind string
		var staffName, clientGivenName, clientPreferredName sql.NullString
		if err := rows.Scan(&row.cursorID, &row.SubjectKind, &row.SubjectID, &row.Action, &actorKind,
			&staffName, &clientGivenName, &clientPreferredName, &row.CreatedAt); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return ListResponse{}, fmt.Errorf("activityfeed: scan subject activity row: %w", err)
		}
		row.ActorKind = actorKind
		row.ActorName = resolveActorName(actorKind, staffName, clientGivenName, clientPreferredName)
		items = append(items, row)
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return ListResponse{}, fmt.Errorf("activityfeed: iterate subject activity rows: %w", err)
	}

	hasMore := len(items) > pageSize
	if hasMore {
		items = items[:pageSize]
	}
	resp := ListResponse{Items: make([]Entry, len(items)), HasMore: hasMore}
	for i, row := range items {
		resp.Items[i] = row.Entry
	}
	if hasMore {
		last := items[len(items)-1]
		next := pagecursor.Encode(last.CreatedAt, last.cursorID)
		resp.NextCursor = &next
	}
	return resp, nil
}

// listForSubjectQueryTemplate and its "after" counterpart are the same
// (created_at, id) cursor shape docs/api-design.md section 4 asks for,
// generalized to any subject_kind/subject_id pair rather than
// engagement.listEngagementActivityQueryTemplate's engagement-only one.
// %s is exclusion (see ListForSubject's own doc comment for why the
// fmt.Sprintf that fills it carries no injection risk).
const listForSubjectQueryTemplate = `SELECT a.id, a.subject_kind, a.subject_id, a.action, a.actor_kind::text,
	       s.name, c.given_name, c.preferred_name, a.created_at
	FROM activity a
	LEFT JOIN staff s ON s.id = a.actor_staff_id
	LEFT JOIN clients c ON c.id = a.actor_client_id
	WHERE a.practice_id = $1 AND a.subject_kind = $2 AND a.subject_id = $3 AND %s
	ORDER BY a.created_at DESC, a.id DESC LIMIT $4`

const listForSubjectAfterQueryTemplate = `SELECT a.id, a.subject_kind, a.subject_id, a.action, a.actor_kind::text,
	       s.name, c.given_name, c.preferred_name, a.created_at
	FROM activity a
	LEFT JOIN staff s ON s.id = a.actor_staff_id
	LEFT JOIN clients c ON c.id = a.actor_client_id
	WHERE a.practice_id = $1 AND a.subject_kind = $2 AND a.subject_id = $3 AND %s
	  AND (a.created_at, a.id) < ($4, $5)
	ORDER BY a.created_at DESC, a.id DESC LIMIT $6`
