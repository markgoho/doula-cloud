package message

import (
	"fmt"
	"os"
	"testing"

	"doula-cloud/api/internal/testdb"
)

// TestAwaitingReplyQueryPlanAtScale is not part of the coverage gate's
// normal run -- it seeds a 14-doula agency's worth of Engagements and
// Messages, disproportionate to run on every `go test ./...` (docs/testing.md's
// own Podman infra is shared with other worktrees). Run it deliberately:
//
//	RUN_PERF_TEST=1 go test ./internal/message/... -run TestAwaitingReplyQueryPlanAtScale -v
//
// It exists to answer AC4 -- "an explain-plan note or benchmark confirming
// it stays fast at a 14-doula agency's volume, not a fixture's" -- for the
// index this ticket does add (messages_engagement_created_at, 00066).
// Lives in package message itself (not message_test) so it can EXPLAIN
// listAwaitingReplyQuery directly -- the exact SQL AwaitingReplyHandler
// runs -- rather than a hand-copied literal free to drift from it, the
// same reasoning activityfeed.TestPracticeQueryPlanAtScale gives for doing
// the same.
//
// Captured plan (2026-09-04, this repo's own Podman Postgres): 500
// Engagements (a generous multiple of a 14-doula pilot Practice's own
// caseload) each carrying 10 Messages, latest sender alternating so half
// the Engagements are actually awaiting a reply. EXPLAIN (ANALYZE,
// BUFFERS) shows a Bitmap Heap Scan on engagements via
// engagements_practice_idx (00059) for the practice_id seek (500 rows,
// ~0.18ms), a Nested Loop pulling exactly one row per Engagement from
// messages via an Index Scan on messages_engagement_created_at -- the
// LATERAL's own "ORDER BY created_at DESC, id DESC LIMIT 1" needs no sort
// at all, the index is already in that order -- then a top-N heapsort
// over the 250 sender_type = 'client' survivors before LIMIT trims to a
// page. Planning 0.8ms, Execution 1.6ms total. Most of the buffer
// traffic (1,508 of 3,022 shared-buffer hits) is messages' own RLS policy
// (messages_practice_visibility) re-checking practice_id through a
// correlated EXISTS against engagements for each of the 500 per-Engagement
// probes -- a pre-existing cost of reading messages at all, not something
// this query adds -- rather than the index lookup itself, which is a
// single-page seek per call. The per-Engagement lookup is exactly what
// messages_engagement_created_at (00066) exists for: before it, this step
// was a sequential scan of the whole messages table, once per Engagement.
func TestAwaitingReplyQueryPlanAtScale(t *testing.T) {
	if os.Getenv("RUN_PERF_TEST") == "" {
		t.Skip("set RUN_PERF_TEST=1 to run (seeds thousands of rows)")
	}

	db := testdb.New(t)
	practiceID := testdb.SeedPractice(t, db, "Perf Practice")

	const engagementCount = 500
	const messagesPerEngagement = 10
	for i := range engagementCount {
		var clientID, engagementID string
		if err := db.Admin.QueryRowContext(t.Context(),
			`INSERT INTO clients (practice_id, given_name, email) VALUES ($1, $2, $3) RETURNING id`,
			practiceID, fmt.Sprintf("Perf Client %d", i), fmt.Sprintf("perf-%d@example.com", i),
		).Scan(&clientID); err != nil {
			t.Fatalf("seed client: %v", err)
		}
		if err := db.Admin.QueryRowContext(t.Context(),
			`INSERT INTO engagements (client_id, practice_id, status, kind) VALUES ($1, $2, 'active', 'birth') RETURNING id`,
			clientID, practiceID,
		).Scan(&engagementID); err != nil {
			t.Fatalf("seed engagement: %v", err)
		}
		// Alternating final sender: half the Engagements end on the
		// Client's own word (awaiting a reply), half on staff's.
		lastSender := senderTypeClient
		if i%2 == 0 {
			lastSender = senderTypeStaff
		}
		if _, err := db.Admin.ExecContext(t.Context(),
			`INSERT INTO messages (engagement_id, sender_type, sender_id, body, created_at)
			 SELECT $1,
			        CASE WHEN n = $2 THEN $3::actor_type ELSE $4::actor_type END,
			        $5, 'perf message', now() - ((($2 - n) || ' minutes')::interval)
			 FROM generate_series(1, $2) AS n`,
			engagementID, messagesPerEngagement, lastSender, senderTypeStaff, clientID,
		); err != nil {
			t.Fatalf("seed messages: %v", err)
		}
	}

	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(t.Context(), `SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set practice id: %v", err)
	}

	query := fmt.Sprintf(`SELECT e.id, c.given_name, c.preferred_name, m.created_at
		FROM engagements e
		JOIN clients c ON c.id = e.client_id
		%s
		WHERE e.practice_id = $1 AND m.sender_type = $2
		ORDER BY m.created_at DESC, e.id DESC LIMIT %d`, awaitingReplyLateral, awaitingPageSize+1)

	rows, err := tx.QueryContext(t.Context(), "EXPLAIN (ANALYZE, BUFFERS) "+query, practiceID, senderTypeClient)
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	defer func() { _ = rows.Close() }()
	t.Logf("EXPLAIN (ANALYZE, BUFFERS) for awaiting-reply query at %d Engagements x %d Messages:", engagementCount, messagesPerEngagement)
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("scan explain line: %v", err)
		}
		t.Log(line)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate explain lines: %v", err)
	}
}
