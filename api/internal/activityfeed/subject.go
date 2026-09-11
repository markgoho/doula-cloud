package activityfeed

import (
	"context"
	"database/sql"
	"fmt"

	"doula-cloud/api/internal/activitypage"
	"doula-cloud/api/internal/pagecursor"
)

// entryProjection is the shape of a subject-scoped read that wants
// nothing beyond what every such read already carries: no extra joins,
// no extra columns, and a builder that drops the activity row's own id
// on the floor.
//
// It is the reason #1150's "a Client cannot receive a diff by default"
// holds at the SQL level rather than in a check somewhere. A diff is an
// extra column a caller names in its own projection; this projection
// names none, so the statement portal.ActivityHandler issues has no
// a.diff in it at all. Withholding is not a decision this code makes on
// each read -- it is what the absence of a line does.
var entryProjection = activitypage.Projection[Entry]{
	Row: func() ([]any, func(activitypage.Row) Entry) {
		return nil, func(r activitypage.Row) Entry {
			return Entry{
				SubjectKind: r.SubjectKind,
				SubjectID:   r.SubjectID,
				Action:      r.Action,
				ActorKind:   r.ActorKind,
				ActorName:   r.ActorName,
				CreatedAt:   r.CreatedAt,
			}
		}
	},
}

// ListForSubject reads one page of subjectKind/subjectID's activity,
// scoped to practiceID, newest first, cursor-paginated -- #486's Key
// interfaces: "a record-scoped variant taking a subject kind and subject
// id, following the shape the existing per-Client and per-Engagement
// readers already use".
//
// Since #1150 that shape lives in activitypage, which every
// subject-scoped reader in the codebase now goes through, and this
// function is the projection of it that answers in the generic Entry --
// the one a Client-portal caller reads. The gate reasoning and the
// exclusion contract are activitypage's own doc comments now, stated
// once for every caller rather than here for one: this applies no access
// check of its own, and excludedActions is the list of actions the read
// must not return, bound as parameters rather than written into the
// query.
//
// It applies no ADR-0008 money filter either: that tier is a Staff-role
// concept activitygate.CanSeeAction/RestrictedActions already model, and
// does not apply to every caller of this function
// (portal.ActivityHandler's Client reads her own money in full, per
// CONTEXT.md's Activity entry).
func ListForSubject(ctx context.Context, tx *sql.Tx, practiceID, subjectKind, subjectID string, excludedActions []string, after *pagecursor.Cursor, pageSize int) (ListResponse, error) {
	page, err := activitypage.List(ctx, tx, activitypage.Query{
		PracticeID:      practiceID,
		SubjectKind:     subjectKind,
		SubjectID:       subjectID,
		ExcludedActions: excludedActions,
		After:           after,
		PageSize:        pageSize,
	}, entryProjection)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return ListResponse{}, fmt.Errorf("activityfeed: list subject activity: %w", err)
	}
	return ListResponse{Items: page.Items, NextCursor: page.NextCursor, HasMore: page.HasMore}, nil
}
