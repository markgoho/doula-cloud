// Package activity is the one audit log ADR-0022 asks for
// (docs/adr/0022-one-activity-log-with-a-subject-and-three-kinds-of-
// actor.md): a row names what it happened to -- SubjectKind and
// SubjectID -- rather than living in a table named after that thing.
// staffauth, client and clientfieldtemplate all record their history
// through Record rather than each holding its own INSERT.
package activity

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// ActorKind is the ADR's three kinds of actor -- staff, client and
// system -- never two: a Client's own signature or payment is her act,
// not the product's, and the product acting on its own behalf is a
// distinct third case, never confused with either.
type ActorKind string

// The three ActorKind values ADR-0022 names.
const (
	ActorStaff  ActorKind = "staff"
	ActorClient ActorKind = "client"
	ActorSystem ActorKind = "system"
)

// Actor is who did it. Exactly one of StaffID/ClientID is set, matching
// Kind -- StaffActor, ClientActor and SystemActor are the only way to
// build one, so a caller cannot reach Record with a mismatched pair.
type Actor struct {
	Kind     ActorKind
	StaffID  string
	ClientID string
}

// StaffActor is a Staff member acting -- most of the product's history.
func StaffActor(staffID string) Actor { return Actor{Kind: ActorStaff, StaffID: staffID} }

// ClientActor is the Client herself acting -- signing, paying -- never
// folded into ActorSystem (ADR-0022's "Considered and rejected").
func ClientActor(clientID string) Actor { return Actor{Kind: ActorClient, ClientID: clientID} }

// SystemActor is Doula Cloud acting with nobody asking. Displays as
// "Doula Cloud", never "System" (ADR-0022).
func SystemActor() Actor { return Actor{Kind: ActorSystem} }

// Entry is one row: what it happened to (SubjectKind, SubjectID), what
// happened (Action), the diff, and the Actor. Diff is already-marshaled
// JSON -- each caller shapes its own diff (a per-field change map, a
// whole-array before/after, a typed field's before/after), and Record
// has no reason to know any of those shapes.
type Entry struct {
	PracticeID  string
	SubjectKind string
	SubjectID   string
	Action      string
	Diff        json.RawMessage
	Actor       Actor
}

// RecordOption configures a Record call. ScopedTo is the only one.
type RecordOption func(*recordConfig)

type recordConfig struct {
	widenToPractice string
}

// ScopedTo widens app.current_practice_id to practiceID inside Record
// itself, as its last statement before the INSERT -- for a write site
// that runs outside staffauth.Middleware (a Client-portal or
// unauthenticated-token path) and so needs the widening to pass
// activity's single RLS policy (activity_practice_visibility,
// 00051_activity_log.sql), which compares only against
// app.current_practice_id. ADR-0022's "fourth event table" section names
// exactly this trap. Folding the set_config into Record, after e's
// arguments are already assembled and immediately before the insert,
// means no statement can land between the widening and the write it
// exists for -- there is no before-ordering for a caller to get wrong or
// have to comment about. set_config's third argument is still
// transaction-scoped, though: the widening outlives this call, so a
// scoped Record must still be the last RLS-gated statement its caller
// runs on tx before Commit. Mirrors payments.resolveInvoiceForEvent's
// own set_config, which stays separate because it deliberately scopes
// more than one statement on that webhook path.
func ScopedTo(practiceID string) RecordOption {
	return func(c *recordConfig) { c.widenToPractice = practiceID }
}

// Record writes one row of e's history, in the caller's own transaction
// so the record and the change it describes either both land or neither
// does -- CLAUDE.md's audit-trail expectation, answered where the change
// happens. Pass ScopedTo(practiceID) when tx has no per-request
// app.current_practice_id already set (see ScopedTo's own doc comment);
// every other caller passes no options.
func Record(ctx context.Context, tx *sql.Tx, e Entry, opts ...RecordOption) error {
	var cfg recordConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	var staffID, clientID sql.NullString
	switch e.Actor.Kind {
	case ActorStaff:
		staffID = sql.NullString{String: e.Actor.StaffID, Valid: true}
	case ActorClient:
		clientID = sql.NullString{String: e.Actor.ClientID, Valid: true}
	case ActorSystem:
	}

	diff := e.Diff
	if diff == nil {
		diff = json.RawMessage("{}")
	}

	if cfg.widenToPractice != "" {
		if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_practice_id', $1, true)`, cfg.widenToPractice); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			return fmt.Errorf("activity: scope to practice: %w", err)
		}
	}

	_, err := tx.ExecContext(ctx,
		`INSERT INTO activity
		     (practice_id, subject_kind, subject_id, action, diff, actor_kind, actor_staff_id, actor_client_id)
		 VALUES ($1, $2, $3, $4, $5, $6::activity_actor_kind, $7, $8)`,
		e.PracticeID, e.SubjectKind, e.SubjectID, e.Action, []byte(diff), string(e.Actor.Kind), staffID, clientID,
	)
	// coverage:ignore reason: DB query failure, not exercised by unit tests
	if err != nil {
		return fmt.Errorf("activity: record: %w", err)
	}
	return nil
}
