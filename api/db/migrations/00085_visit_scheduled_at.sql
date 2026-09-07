-- +goose Up
-- #250: a Visit's own scheduled instant, distinct from created_at (when
-- the row was typed). Nullable -- a Doula logging a meeting that already
-- happened is not forced to schedule it, and every Visit created before
-- this migration stays valid with no value. No RLS change: the existing
-- visits_practice_visibility policy (00007_visit.sql) is column-agnostic
-- and already covers SELECT/INSERT/UPDATE/DELETE on this column the same
-- way it covers staff_id.
ALTER TABLE visits ADD COLUMN scheduled_at timestamptz;

-- +goose Down
ALTER TABLE visits DROP COLUMN scheduled_at;
