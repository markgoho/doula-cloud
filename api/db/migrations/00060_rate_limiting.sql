-- +goose Up
-- #602: the BFF had no rate limiting of any kind. Two tables, both BFF
-- infrastructure with no tenant -- the same reasoning 00028_sessions.sql
-- gives for skipping RLS on `sessions`, which this restates rather than
-- cites alone because a reviewer reading this file in isolation should
-- not have to open another migration to see why RLS is absent here too.
--
-- rate_limit_buckets is the counter a request checks and increments in
-- one atomic UPSERT (see ratelimit.Wrap): one row per (endpoint,
-- dimension, key) triple, holding the current window's count and when
-- that window started. No reaper: a bucket nobody re-touches is a few
-- bytes of dead weight, and the same UPSERT that would have incremented
-- it resets it to a fresh window the moment anyone does -- idempotency_
-- keys (00027) already accepted the same trade for the same reason.
CREATE TABLE rate_limit_buckets (
    key          text PRIMARY KEY,
    window_start timestamptz NOT NULL,
    count        integer NOT NULL
);

GRANT SELECT, INSERT, UPDATE ON rate_limit_buckets TO app_runtime;

-- rate_limit_refusals is the audit trail CLAUDE.md's cross-cutting
-- expectation asks for -- "who did it and when" -- for the one action
-- here worth recording after the fact: a refusal. It cannot be an
-- `activity` row (ADR-0022): every endpoint this ticket limits runs
-- before any Practice exists or is known (staff signup, staff/portal
-- invitation acceptance, login), and activity.practice_id is NOT NULL
-- with an INSERT policy gated on it. This is the same shape ADR-0022
-- itself names for `staff_work_state_events` (00043) and 00055 names
-- again for credit_ledger.granted_by: "where activity cannot hold the
-- [fact], the record lives on the table that owns the fact." Global,
-- like the buckets above -- a refusal belongs to no Practice, so there
-- is nothing for a practice-scoped policy to check.
CREATE TABLE rate_limit_refusals (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    endpoint   text NOT NULL,
    dimension  text NOT NULL,
    key_value  text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- The one query shape this table serves: every refusal against one
-- address/IP, newest first, to answer "is someone hammering this".
CREATE INDEX rate_limit_refusals_lookup ON rate_limit_refusals (dimension, key_value, created_at);

GRANT SELECT, INSERT ON rate_limit_refusals TO app_runtime; -- no UPDATE, no DELETE -- append-only

-- +goose Down
DROP TABLE rate_limit_refusals;
DROP TABLE rate_limit_buckets;
