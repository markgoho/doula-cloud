-- +goose NO TRANSACTION
-- +goose Up
-- #2480, ADR-0022's amendment of 2026-10-31: the fourth Activity actor
-- kind. ADD VALUE stands alone because 00142's CHECK names the value, and
-- a value added in a transaction cannot be used before it commits;
-- 00142 stays transactional so its functions' REVOKE lands with them.
ALTER TYPE activity_actor_kind ADD VALUE IF NOT EXISTS 'operator';

-- +goose Down
-- Postgres has no DROP VALUE; 00142's Down removes every use of it.
SELECT 1;
