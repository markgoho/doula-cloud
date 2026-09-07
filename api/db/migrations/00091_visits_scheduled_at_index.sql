-- +goose Up
-- #263: the Practice-wide schedule reads Visits by when they happen, in
-- that order -- `WHERE scheduled_at IS NOT NULL AND scheduled_at >= $from
-- AND scheduled_at < $to ORDER BY scheduled_at, id`
-- (api/internal/visit/practice_list.go). This index is that query written
-- as an index: the partial predicate is the query's own IS NOT NULL
-- predicate, so a Practice's unscheduled Visits are not in the index at
-- all, and the column order is the query's ORDER BY, so the planner
-- range-scans the requested window and the page comes out already sorted
-- with no separate sort step. `id` is in the key rather than only in the
-- heap because the cursor compares the pair (scheduled_at, id), which is
-- what keeps paging stable when two Visits share an instant.
--
-- No practice_id column exists on visits (the Practice filter is the join
-- to engagements, the same reach visits_practice_visibility already
-- uses), so this index cannot lead on one. The bounded window is what
-- keeps the scan small instead.
CREATE INDEX visits_scheduled_at_idx ON visits (scheduled_at, id)
  WHERE scheduled_at IS NOT NULL;

-- +goose Down
DROP INDEX visits_scheduled_at_idx;
