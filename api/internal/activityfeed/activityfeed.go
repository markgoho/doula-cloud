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

import (
	"encoding/json/jsontext"
	"time"
)

// Entry is one row of the feed: what it happened to (SubjectKind,
// SubjectID), what happened (Action), and who did it -- the DTO #486's
// Key interfaces name: "subject kind, subject id, action, actor kind, the
// actor's name ... and the created-at timestamp." ActorName is always
// populated, never a bare id a reader has to resolve itself, matching
// engagement.ActivityEntry's own reasoning. Relative-versus-absolute date
// rendering is left to the UI, per the same Key interfaces bullet.
type Entry struct {
	SubjectKind string `json:"subjectKind"`
	SubjectID   string `json:"subjectId"`

	// SubjectName is the person the row happened to, already resolved --
	// #1148: a feed spanning many subject kinds has to say whose roster
	// change this was, or "Roles changed" names the actor and nobody
	// else. Populated for a membership row (the staff member) and, since
	// #2006, for an engagement or client row (the Client it is about);
	// empty for a practice row, whose subject is the reader's own
	// Practice. See practice.go's resolveSubjectName.
	SubjectName string `json:"subjectName,omitempty"`

	Action    string    `json:"action"`
	ActorKind string    `json:"actorKind"`
	ActorName string    `json:"actorName"`
	CreatedAt time.Time `json:"createdAt"`

	// VisitScheduledAt and VisitAddedAt say which Visit a visit_reassigned
	// row moved (#2666): the Visit's date when it moved, absent for a Visit
	// with no date, and the instant it was added, which never changes. Both
	// are absent on every other row and on a reassignment written before
	// #2666. They are instants only: the reader formats them, as it does
	// every Visit date.
	VisitScheduledAt *time.Time `json:"visitScheduledAt,omitempty"`
	VisitAddedAt     *time.Time `json:"visitAddedAt,omitempty"`

	// ReassignedFromName and ReassignedToName are the Doula a
	// visit_reassigned row moved the Visit from and to (#2672), resolved
	// the way the Engagement ledger resolves them. Practice-wide feed only:
	// the Client portal's projection never sets them, and excludes the
	// action besides.
	ReassignedFromName string `json:"reassignedFromName,omitempty"`
	ReassignedToName   string `json:"reassignedToName,omitempty"`

	// Diff is carried for three actions only: invoices_exported and
	// clients_exported (#2871), which slice of the book left the product, in
	// no Client's name, and practice_on_call_changed, the on-call rule
	// before and after, so the Staff feed can say what changed. Every other row sends none, which
	// keeps a diff off the Client portal and out of this feed (#1150).
	Diff jsontext.Value `json:"diff,omitempty"`
}

// ListResponse is docs/api-design.md section 4's envelope, the same shape
// engagement.ActivityListResponse already uses.
type ListResponse struct {
	Items      []Entry `json:"items"`
	NextCursor *string `json:"nextCursor,omitempty"`
	HasMore    bool    `json:"hasMore"`
}
