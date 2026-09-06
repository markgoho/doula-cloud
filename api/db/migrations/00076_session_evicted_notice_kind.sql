-- +goose Up
-- A fourth session_notice_outbox (00036) kind, for #610's cross-population
-- eviction: signing into one population evicts the browser's live session
-- in the other, and the evicted Staff member is told it happened.
--
-- Not 'session_revoked' reused: that notice's words are "all of your
-- sessions were signed out ... on every device", which authn.EndAllSessions
-- makes true and an eviction does not -- an eviction deletes exactly one
-- row, the one this browser held, and the same person's session on her
-- phone is untouched. One kind per true sentence, the same reason
-- 'mfa_recovery_cleared' (00062) is queued alongside 'session_revoked'
-- rather than folded into it.
--
-- Its own migration rather than an append to any earlier file: Postgres
-- refuses ALTER TYPE ... ADD VALUE used later in the transaction that
-- added it, and goose runs one file as one transaction -- so 00077's
-- partial index, which reads this value, could not live here. The same
-- split 00062/00063 already made, for the same reason.
ALTER TYPE session_notice_kind ADD VALUE 'session_evicted';

-- +goose Down
-- 00077's own down runs first (higher version, reversed order) and drops
-- the partial index that reads 'session_evicted', which is what makes
-- rebuilding the enum without it safe here.
ALTER TYPE session_notice_kind RENAME TO session_notice_kind_old;
CREATE TYPE session_notice_kind AS ENUM ('new_signin', 'session_revoked', 'mfa_recovery_cleared');
ALTER TABLE session_notice_outbox
    ALTER COLUMN kind TYPE session_notice_kind
    USING kind::text::session_notice_kind;
DROP TYPE session_notice_kind_old;
