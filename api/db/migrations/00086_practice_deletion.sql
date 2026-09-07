-- +goose Up
-- #871: a Practice deletes itself, following ADR-0027's own template --
-- redact in place, never a hard DELETE -- but adapted for what a
-- Practice carries that a Client does not: other people's access
-- (Staff, Clients) and Doula Cloud's own business records. Three
-- pieces: the durable deletion-state facts on practices itself, the
-- credit-forfeiture enum value (00085, a separate migration because
-- ALTER TYPE ... ADD VALUE and this file's own DDL cannot safely share
-- one transaction if this file ever needed to read it back), and the
-- outbox that carries the day-23 reminder and day-30 finalization.

-- =====================================================================
-- practices: deletion state
-- =====================================================================

-- deletion_requested_at is the one fact every gate reads: null means
-- "not pending", set means a 30-day restore window is open. Unlike
-- clients.erased_at (ADR-0027), this is not the terminal state itself --
-- deleted_at is -- because a Practice's deletion has a middle: a window
-- in which it can still be undone. deletion_requested_by and
-- deletion_finalize_at are read together with it by staffauth's own
-- membership query (no new query, one wider SELECT) and by the
-- pending-deletion screen. deletion_finalize_at is the durable
-- day-30 target; the practice_deletion_outbox row below carries its own,
-- separately mutable next_attempt_at, the same split 00065 drew between
-- redactable_after and next_attempt_at, and for the same reason: a
-- retry backoff on the outbox row must never quietly move the deadline
-- a restore screen already showed an Owner.
ALTER TABLE practices ADD COLUMN deletion_requested_at timestamptz;
ALTER TABLE practices ADD COLUMN deletion_requested_by uuid REFERENCES staff (id);
ALTER TABLE practices ADD COLUMN deletion_finalize_at timestamptz;

-- deleted_at is the terminal fact, set only once, at finalization. The
-- row and its id survive untouched (this ticket's decision 1: a
-- Practice is a business entity, not a natural person, so unlike
-- clients.erased_at's redaction of every identifying column, nothing on
-- practices itself is scrubbed) -- every Client under it has already
-- been redacted by cascaded client.Erase calls by the time this is set.
ALTER TABLE practices ADD COLUMN deleted_at timestamptz;

-- =====================================================================
-- practice_deletion_outbox
-- =====================================================================

-- Two acts, enqueued together at initiation: a reminder 7 days before
-- the deadline and the finalization itself, the same row-for-row shape
-- client_erasure_outbox (00064) and every other outbox in this codebase
-- use. Neither row is ever cancelled or deleted when an Owner restores
-- the Practice -- this table carries no DELETE grant, the same design
-- 00064 chose, because Postgres's row-level locking gives no clean way
-- to guarantee a cancel lands before a concurrent claim does. Instead
-- each act rechecks practices.deletion_requested_at/deleted_at live, at
-- send time, before doing anything -- the "skip-at-send recheck" shape
-- offer/outbox.go already uses for a withdrawn Offer, generalized here
-- to a restored Practice: a row whose Practice is no longer pending is
-- marked sent having done nothing.
CREATE TYPE practice_deletion_act AS ENUM ('reminder', 'finalize');

CREATE TYPE practice_deletion_outbox_status AS ENUM ('pending', 'sent', 'dead_lettered');

CREATE TABLE practice_deletion_outbox (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    practice_id     uuid NOT NULL REFERENCES practices (id),
    act             practice_deletion_act NOT NULL,
    status          practice_deletion_outbox_status NOT NULL DEFAULT 'pending',
    attempt_count   int NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    sent_at         timestamptz,
    last_error      text
);

-- At most one pending row per (Practice, act). This is not only a
-- backstop the way 00064's client_erasure_outbox index is: enqueue()
-- upserts on this exact (practice_id, act) WHERE status = 'pending'
-- conflict target, so a restore during the window followed by a fresh
-- initiate moves the existing pending row's deadline forward instead of
-- colliding with it or leaving a stale row for the skip-at-send recheck
-- to fire early against.
CREATE UNIQUE INDEX practice_deletion_outbox_one_pending
    ON practice_deletion_outbox (practice_id, act)
    WHERE status = 'pending';

CREATE INDEX practice_deletion_outbox_claim
    ON practice_deletion_outbox (next_attempt_at)
    WHERE status = 'pending';

GRANT SELECT, INSERT, UPDATE ON practice_deletion_outbox TO app_runtime;

-- No RLS -- platform-level like every other outbox table (00064's own
-- comment applies unchanged): the worker runs with no Practice session
-- context, and practice_id is here so it and a later report can name
-- which Practice an act belonged to without a join.

-- +goose Down
DROP TABLE practice_deletion_outbox;
DROP TYPE practice_deletion_outbox_status;
DROP TYPE practice_deletion_act;

ALTER TABLE practices DROP COLUMN deleted_at;
ALTER TABLE practices DROP COLUMN deletion_finalize_at;
ALTER TABLE practices DROP COLUMN deletion_requested_by;
ALTER TABLE practices DROP COLUMN deletion_requested_at;
