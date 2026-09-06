-- +goose Up
-- sessionnotice.QueueSessionEvicted's ON CONFLICT arbiter -- the same
-- one-pending-row race guard session_notice_outbox_revoked_one_pending
-- (00036) and session_notice_outbox_mfa_recovery_cleared_one_pending
-- (00063) give their own kinds, restated for 00076's 'session_evicted',
-- since a partial unique index only matches the exact WHERE clause an
-- ON CONFLICT names, not the kind value alone. The race it guards is two
-- confirmed sign-ins landing in the same browser before either's queue
-- insert commits.
--
-- Its own migration, not appended to 00076: see 00076's own comment, and
-- 00063's, for why an ADD VALUE and an index reading it cannot share one
-- goose file.
CREATE UNIQUE INDEX session_notice_outbox_evicted_one_pending
    ON session_notice_outbox (identity_uid)
    WHERE status = 'pending' AND kind = 'session_evicted';

-- +goose Down
-- Dropped first (before 00076's own down migration rebuilds
-- session_notice_kind without 'session_evicted') precisely because this
-- index's WHERE clause depends on that value existing.
DROP INDEX session_notice_outbox_evicted_one_pending;
