-- +goose Up
-- #251: a Visit's own free-text notes, written and re-written by Staff.
-- Nullable, not NOT NULL DEFAULT '' -- "never written" and "cleared to
-- empty" are different facts, and NotesHandler (api/internal/visit) only
-- ever writes a non-null string once a Staff member submits the notes
-- write at all, so a Visit that has never had that write reach it stays
-- NULL. No RLS change: the existing visits_practice_visibility policy
-- (00007_visit.sql) is column-agnostic and already covers
-- SELECT/INSERT/UPDATE/DELETE on this column the same way it covers
-- staff_id and scheduled_at (00085).
ALTER TABLE visits ADD COLUMN notes text;

-- +goose Down
ALTER TABLE visits DROP COLUMN notes;
