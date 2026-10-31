-- +goose Up
-- #2209: when a Plan Instance last changed, the date its printed page and
-- its PDF carry so a reader can tell whether the copy in hand is current.
-- It is the instant the answers last became different -- the same test
-- PutInstanceHandler already uses to clear the Client's acknowledgment
-- (#301) -- and, before any answer is saved, the instant the plan was
-- started. A save that leaves every answer as it was does not move it.
--
-- Written by the two statements that make a Plan Instance what it is,
-- PostInstanceHandler's INSERT and PutInstanceHandler's UPDATE, from
-- clock.Now(ctx) (#773, #1992). There is no DEFAULT, the same as 00131:
-- a later insert that leaves it out is refused by NOT NULL rather than
-- quietly taking the database's clock. An existing row is backfilled from
-- created_at, the one stored instant it has.
ALTER TABLE plan_instances ADD COLUMN changed_at timestamptz;
UPDATE plan_instances SET changed_at = created_at;
ALTER TABLE plan_instances ALTER COLUMN changed_at SET NOT NULL;

-- +goose Down
ALTER TABLE plan_instances DROP COLUMN changed_at;
