-- +goose NO TRANSACTION
-- +goose Up
-- LV-G8 (#291) asks for two things 00039 did not build: a route that
-- removes a Membership, and a record of who removed it and when. The
-- record is the reason this is an enum value rather than nothing at all
-- -- the practice_memberships row goes away, so the event row is the
-- only place the removal survives.
--
-- ALTER TYPE ... ADD VALUE cannot run inside a transaction block, hence
-- the NO TRANSACTION annotation: goose wraps each migration in one
-- otherwise.
ALTER TYPE membership_event_type ADD VALUE 'removed';

-- +goose Down
-- Postgres has no DROP VALUE. Rebuilding the enum means rewriting every
-- column that uses it, which is a larger and riskier operation than the
-- Up it reverses; a spare enum member is inert. The table it lives on is
-- dropped whole by 00039's own Down.
SELECT 1;
