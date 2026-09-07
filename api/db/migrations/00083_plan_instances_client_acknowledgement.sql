-- +goose Up
-- #301's v1 decision: a Client may acknowledge that she has read her
-- Birth Plan -- she cannot edit a field or suggest a change. Nullable and
-- unset by default: "never acknowledged" and "not applicable" (a Care
-- Plan row, staff-only, never reaches a Client-portal write) both read as
-- NULL here, distinguished by plan_type rather than by this column
-- alone. A Staff edit that changes the stored `answers` clears this back
-- to NULL (plans.PutInstanceHandler), so a non-NULL value always means
-- "read since the plan last changed," never just "read once, ever."
ALTER TABLE plan_instances ADD COLUMN client_acknowledged_at timestamptz;

-- +goose Down
ALTER TABLE plan_instances DROP COLUMN client_acknowledged_at;
