package activitypage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Stamp answers "who did this, and when?" for one fact a screen shows
// beside the thing it happened to (#2104): a Contract's send, a Plan's
// last edit. By is the actor's name as the ledger itself prints it
// (ActorName), so a departed Staff member reads the same here as on the
// Activity section below it.
type Stamp struct {
	At time.Time `json:"at"`
	By string    `json:"by"`
}

// LatestQuery names one subject's newest row of one action.
type LatestQuery struct {
	PracticeID  string
	SubjectKind string
	SubjectID   string
	Action      string
	// NotBefore, when set, ignores a row written before it. A Contract's
	// send is recorded against its Engagement, not the Contract, so a
	// later Contract on the same Engagement passes its own created_at
	// here to leave the voided one's send behind.
	NotBefore *time.Time
}

// Latest reads q's newest row and names its actor, or reports nil when
// there is none. Like List it applies no access gate, and it runs on the
// caller's own transaction, so activity_practice_visibility (00051) is
// still in force beneath the practice_id predicate it writes.
func Latest(ctx context.Context, tx *sql.Tx, q LatestQuery) (*Stamp, error) {
	var (
		actorKind                                                           string
		staffName, clientGiven, clientPreferred, clientFamily, operatorName sql.NullString
		at                                                                  time.Time
	)
	err := tx.QueryRowContext(ctx,
		`SELECT a.actor_kind::text, `+SharedActorJoin.Columns+`, a.created_at
		 FROM activity a
		 `+SharedActorJoin.Joins+`
		 WHERE a.practice_id = $1 AND a.subject_kind = $2 AND a.subject_id = $3 AND a.action = $4
		   AND ($5::timestamptz IS NULL OR a.created_at >= $5::timestamptz)
		 ORDER BY a.created_at DESC, a.id DESC
		 LIMIT 1`,
		q.PracticeID, q.SubjectKind, q.SubjectID, q.Action, q.NotBefore,
	).Scan(&actorKind, &staffName, &clientGiven, &clientPreferred, &clientFamily, &operatorName, &at)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil //nolint:nilnil // nothing happened yet is an answer, not a failure
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return nil, fmt.Errorf("activitypage: latest %s: %w", q.Action, err)
	}
	return &Stamp{
		At: at,
		By: ActorName(actorKind, staffName, clientGiven, clientPreferred, clientFamily, operatorName, ActorFirstName),
	}, nil
}
