-- +goose Up
-- sessionnotice.QueueMFARecoveryCleared's ON CONFLICT arbiter -- the
-- same one-pending-row race guard session_notice_outbox_revoked_one_pending
-- (00036) gives 'session_revoked', restated for 00062's new
-- 'mfa_recovery_cleared' kind, since a partial unique index only matches
-- the exact WHERE clause an ON CONFLICT names, not the kind value alone.
--
-- Its own migration, not appended to 00062: Postgres refuses
-- ALTER TYPE ... ADD VALUE used later in the same transaction that added
-- it, and goose runs one file as one transaction, so a value 00062 adds
-- cannot be read by an index in that same file. 00062's ADD VALUE has
-- already committed by the time this file's own transaction opens.
CREATE UNIQUE INDEX session_notice_outbox_mfa_recovery_cleared_one_pending
    ON session_notice_outbox (identity_uid)
    WHERE status = 'pending' AND kind = 'mfa_recovery_cleared';

-- +goose Down
-- Dropped first (before 00062's own down migration runs and rebuilds
-- session_notice_kind back to two values) precisely because this index's
-- WHERE clause depends on the third value existing.
DROP INDEX session_notice_outbox_mfa_recovery_cleared_one_pending;
