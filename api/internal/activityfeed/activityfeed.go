// Package activityfeed is #486's cross-subject-kind reader, built on
// #485's activitygate: a Practice-wide feed spanning every registered
// subject kind, and a record-scoped reader parameterized by subject kind
// and subject id for a caller that has already established access to
// that one subject itself (a Client-portal ownership check, say) rather
// than needing the gate's own per-row decision.
//
// It does not replace engagement.ListActivityHandler's own per-Engagement
// reader, which keeps its SQL-level money exclusion for pagination
// correctness on a single subject (see that file's own doc comment) --
// this package's job is the two shapes that reader cannot express: many
// subject kinds in one ordered, cursor-stable list, and a record-scoped
// read for a population (the Client portal) the gate has no reader for
// at all.
package activityfeed

import "time"

// Entry is one row of the feed: what it happened to (SubjectKind,
// SubjectID), what happened (Action), and who did it -- the DTO #486's
// Key interfaces name: "subject kind, subject id, action, actor kind, the
// actor's name ... and the created-at timestamp." ActorName is always
// populated, never a bare id a reader has to resolve itself, matching
// engagement.ActivityEntry's own reasoning. Relative-versus-absolute date
// rendering is left to the UI, per the same Key interfaces bullet.
type Entry struct {
	SubjectKind string    `json:"subjectKind"`
	SubjectID   string    `json:"subjectId"`
	Action      string    `json:"action"`
	ActorKind   string    `json:"actorKind"`
	ActorName   string    `json:"actorName"`
	CreatedAt   time.Time `json:"createdAt"`
}

// ListResponse is docs/api-design.md section 4's envelope, the same shape
// engagement.ActivityListResponse already uses.
type ListResponse struct {
	Items      []Entry `json:"items"`
	NextCursor *string `json:"nextCursor,omitempty"`
	HasMore    bool    `json:"hasMore"`
}
