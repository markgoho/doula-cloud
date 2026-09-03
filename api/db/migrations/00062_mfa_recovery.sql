-- +goose Up
-- #615 (map #164, #605's resolution): the three MFA-recovery paths --
-- Owner vouching, saved codes for a Practice's sole Owner, and a Doula
-- Cloud support action -- and the person-scoped audit table that records
-- every enrolment removal, whichever path caused it.
--
-- The issued (Owner-vouched) code widens auth_token_purpose (00061)
-- rather than getting a table of its own, exactly as that migration's
-- own comment anticipated: it is a single live token per identity, the
-- same shape staff_email_verification and staff_password_reset already
-- have. Safe to add with ALTER TYPE ... ADD VALUE, unlike 00055's enum
-- rebuild, because nothing in this migration uses the new value in the
-- same transaction.
ALTER TYPE auth_token_purpose ADD VALUE 'staff_mfa_recovery';

-- staff_mfa_recovery_vouches records which Owner vouched for which
-- issued code, keyed 1:1 on the auth_tokens row it belongs to.
-- auth_tokens itself (00061) is deliberately generic -- purpose, an
-- identity, an expiry -- and carries no room for "who authorized this
-- one". The Owner-vouch AC needs that fact to survive from mint (an
-- authenticated, Owner-only act) to spend (unauthenticated, #605's
-- sequence): staff_auth_events' actor_staff_id for the eventual
-- 'owner_vouched' row is read out of here, not out of auth_tokens.
--
-- Append-only, like auth_tokens.Mint's own delete-then-insert: a
-- re-vouch mints a fresh auth_tokens row (Mint deletes the prior unspent
-- one first) which cascades here, so at most one row per staff_id is
-- ever live, but the table itself is never updated.
CREATE TABLE staff_mfa_recovery_vouches (
    token_hash     text PRIMARY KEY REFERENCES auth_tokens (token_hash) ON DELETE CASCADE,
    staff_id       uuid NOT NULL REFERENCES staff (id),
    owner_staff_id uuid NOT NULL REFERENCES staff (id),
    created_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX staff_mfa_recovery_vouches_staff_idx ON staff_mfa_recovery_vouches (staff_id);

GRANT SELECT, INSERT ON staff_mfa_recovery_vouches TO app_runtime;

-- staff_mfa_recovery_codes is the sole Owner's saved-code set (#605,
-- §4.2.1.1): a per-person collection, not a single live token, which is
-- exactly the shape auth_tokens' one-row-per-(identity,purpose) index
-- cannot hold without bending it -- so this ticket gives it its own
-- table rather than widening auth_tokens' purpose for it, as #613's own
-- migration comment said to do when a purpose needs a different shape.
--
-- code_hash is SHA-256 of the plaintext, mirroring auth_tokens.token_hash
-- -- only the digest is ever stored. used_at and revoked_at are
-- deliberately separate: a spent code and a code invalidated because a
-- second Owner arrived are different facts (staffmfarecovery's
-- reconcileSavedCodes reads used_at to decide whether to mint a
-- replacement, and would mis-fire if a revoked-but-unspent code looked
-- the same as a spent one).
CREATE TABLE staff_mfa_recovery_codes (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    staff_id   uuid NOT NULL REFERENCES staff (id),
    code_hash  text NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    used_at    timestamptz,
    revoked_at timestamptz
);

-- staffmfarecovery.reconcileSavedCodes' "does she already hold live
-- codes" check and SpendHandler's per-person lookup both filter on
-- exactly this condition.
CREATE INDEX staff_mfa_recovery_codes_live_idx
    ON staff_mfa_recovery_codes (staff_id)
    WHERE used_at IS NULL AND revoked_at IS NULL;

GRANT SELECT, INSERT, UPDATE ON staff_mfa_recovery_codes TO app_runtime;

-- staff_mfa_recovery_outbox delivers a vouched code to the *Owner's* own
-- address (#605: "her own address, not Priya's, which is what makes it
-- a second channel"). A separate table from staff_token_mail_outbox
-- (00061), not a third kind on it: that table's worker resolves its
-- recipient as the *identity_uid the token belongs to*, which for this
-- code is the locked-out person, not the Owner who must actually receive
-- the mail -- the two are different identities by design, so the
-- recipient has to be its own column rather than reusing the token's
-- own identity_uid. subject_staff_id is carried only for the mail copy
-- (naming who the code is for, so an Owner vouching for two people
-- close together isn't left guessing); it names no Practice or Client,
-- so ADR-0009's content restriction is unaffected.
CREATE TYPE staff_mfa_recovery_outbox_status AS ENUM ('pending', 'sent', 'dead_lettered');

CREATE TABLE staff_mfa_recovery_outbox (
    id                     uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    recipient_identity_uid text NOT NULL,
    subject_staff_id       uuid NOT NULL REFERENCES staff (id),
    token                  text,
    status                 staff_mfa_recovery_outbox_status NOT NULL DEFAULT 'pending',
    attempt_count          int NOT NULL DEFAULT 0,
    next_attempt_at        timestamptz NOT NULL DEFAULT now(),
    created_at             timestamptz NOT NULL DEFAULT now(),
    sent_at                timestamptz,
    last_error             text
);

GRANT SELECT, INSERT, UPDATE ON staff_mfa_recovery_outbox TO app_runtime;

-- No RLS on any of the three tables above, matching auth_tokens itself
-- and every other outbox table: staff_mfa_recovery_vouches and
-- staff_mfa_recovery_codes are read and written from the unauthenticated
-- spend endpoint, before any session or Practice context exists to scope
-- a policy against, and staff_mfa_recovery_outbox's worker runs the same
-- way every other outbox worker does -- with no Practice or Client
-- session.

-- ---------------------------------------------------------------------
-- staff_auth_events: the append-only, person-scoped audit table #615's
-- AC asks for. Not ADR-0022's `activity` -- restated from #605's
-- resolution and 00043's own precedent: activity.practice_id is NOT
-- NULL and its INSERT policy needs app.current_practice_id, but an
-- enrolment is a fact about a person, and two of the three paths here
-- (self-service and support) have no Practice in scope when they fire.
--
-- Exactly one of actor_staff_id/actor_operator is set, the same shape
-- credit_ledger.granted_by (00055) uses for a human who is none of
-- ADR-0022's three actor kinds: an Owner vouching or a sole Owner
-- spending her own saved code both name a Staff row (actor_staff_id --
-- for self-service this equals staff_id, she is her own actor); the
-- support path names the Doula Cloud operator who ran it, a person with
-- no staff row at all, so a free-text column is the only place that
-- name can live, mirroring granted_by exactly.
CREATE TYPE staff_auth_event_reason AS ENUM ('owner_vouched', 'self_service', 'support');

CREATE TABLE staff_auth_events (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    staff_id        uuid NOT NULL REFERENCES staff (id),
    actor_staff_id  uuid REFERENCES staff (id),
    actor_operator  text,
    reason          staff_auth_event_reason NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT staff_auth_events_actor_shape CHECK (
        (reason IN ('owner_vouched', 'self_service') AND actor_staff_id IS NOT NULL AND actor_operator IS NULL)
        OR (reason = 'support' AND actor_operator IS NOT NULL AND actor_operator <> '' AND actor_staff_id IS NULL)
    )
);

CREATE INDEX staff_auth_events_staff_idx ON staff_auth_events (staff_id, created_at);

GRANT SELECT, INSERT ON staff_auth_events TO app_runtime;

-- No RLS -- and this is not staff_work_state_events' (00043) shape
-- despite looking like its sibling. That table's writer runs in a
-- pre-Practice window that still has *an identity* (00044's
-- staff_self_update policy keys on app.current_identity_uid). Every
-- writer here has neither: the unauthenticated spend endpoint
-- (SpendMFARecoveryHandler) runs before any session exists at all --
-- the code itself is the only credential -- and the internal support
-- action (SupportClearHandler) is authenticated by X-Internal-Secret,
-- not a session or a Practice. A policy keyed on either session variable
-- would run before that variable ever has a value, the same reasoning
-- auth_tokens (00061) and sessions (00028) give for carrying none. The
-- append-only guarantee is the GRANT above (no UPDATE, no DELETE), not a
-- policy -- there is nobody's row it needs to hide from anybody today,
-- since this ticket builds no reader.

-- A third session_notice_outbox (00036) kind: the affected person is
-- notified whenever her enrolment is cleared, whoever cleared it (#615's
-- AC), the same recipient shape "new sign-in"/"session revoked" already
-- have. Safe to add here, in the same transaction as everything else in
-- this file, the same reasoning auth_token_purpose's own ADD VALUE above
-- rests on: nothing in *this* file's transaction reads the new value.
-- 00063's own partial index does read it, in the next migration's own
-- transaction, once this one has committed -- see 00063's comment for
-- why that one could not just be appended here instead.
ALTER TYPE session_notice_kind ADD VALUE 'mfa_recovery_cleared';

-- +goose Down
-- 00063's own down runs first (higher version, reversed order) and
-- drops the partial index that reads 'mfa_recovery_cleared', which is
-- what makes rebuilding the enum back to two values safe here.
ALTER TYPE session_notice_kind RENAME TO session_notice_kind_old;
CREATE TYPE session_notice_kind AS ENUM ('new_signin', 'session_revoked');
ALTER TABLE session_notice_outbox
    ALTER COLUMN kind TYPE session_notice_kind
    USING kind::text::session_notice_kind;
DROP TYPE session_notice_kind_old;

DROP TABLE staff_auth_events;
DROP TYPE staff_auth_event_reason;
DROP TABLE staff_mfa_recovery_outbox;
DROP TYPE staff_mfa_recovery_outbox_status;
DROP TABLE staff_mfa_recovery_codes;
DROP TABLE staff_mfa_recovery_vouches;

-- Postgres has no ALTER TYPE ... DROP VALUE, so reversing the widened
-- purpose requires rebuilding the enum, the same shape 00021's own down
-- migration uses. Safe only because down-migrations run against a
-- disposable test/dev database (CLAUDE.md: no production data) -- a real
-- staff_mfa_recovery row would fail the USING cast below.
ALTER TYPE auth_token_purpose RENAME TO auth_token_purpose_old;
CREATE TYPE auth_token_purpose AS ENUM ('staff_email_verification', 'staff_password_reset');
ALTER TABLE auth_tokens
    ALTER COLUMN purpose TYPE auth_token_purpose
    USING purpose::text::auth_token_purpose;
DROP TYPE auth_token_purpose_old;
