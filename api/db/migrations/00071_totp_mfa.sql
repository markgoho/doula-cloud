-- +goose Up
-- #606: TOTP MFA, required for Owners, optional otherwise, raisable to
-- required for everyone at a Practice by a switch its Owner throws.
-- Enforcement lives at the Practice-scoped boundary (staffauth.Middleware),
-- not at session mint -- #167's amendment: a role is held by a Membership,
-- not by a person, so "is MFA required" cannot be answered until a
-- Practice is named.

-- The ID token's `firebase.sign_in_second_factor` claim describes the
-- *sign-in event*, and the session cookie outlives the token that proved
-- it -- so the fact has to be carried on the session row, read at mint,
-- not re-derived from a token that is long gone by the time the boundary
-- needs it (decision 3). Defaulting false covers every row minted before
-- this column existed and every population (Client sessions too) that
-- never sets it.
ALTER TABLE sessions ADD COLUMN second_factor boolean NOT NULL DEFAULT false;

-- A Practice's own switch, off by default -- decision in the brief: no
-- grace period, no due date, no countdown. Writable only by an Owner
-- (enforced in Go, RequireOwner); there is no role-shaped thing for a
-- CHECK constraint to enforce here.
ALTER TABLE practices ADD COLUMN require_mfa_for_all_staff boolean NOT NULL DEFAULT false;

-- staff_auth_events (00062) gains two reasons of its own kind: a person
-- enrolling or removing her own TOTP factor, outside any of #615's three
-- recovery paths. Both are self-caused -- actor_staff_id = staff_id, the
-- same shape 'self_service' already uses for "she is her own actor".
-- The per-Practice switch throw/clear is deliberately NOT one of these
-- reasons: it has no staff_id subject (it is a fact about a Practice,
-- not a person), so it belongs only in ADR-0022's activity log, which
-- already has a practice_id to key on.
--
-- Added here, used nowhere in this same migration -- Postgres forbids an
-- ALTER TYPE ... ADD VALUE from being used in the same transaction that
-- adds it (00062's and 00063's own split for exactly this reason). The
-- CHECK constraint that references these two values is 00072, one
-- migration later.
ALTER TYPE staff_auth_event_reason ADD VALUE 'enrolled';
ALTER TYPE staff_auth_event_reason ADD VALUE 'removed';

-- +goose Down
ALTER TABLE practices DROP COLUMN require_mfa_for_all_staff;
ALTER TABLE sessions DROP COLUMN second_factor;

-- 00072's own down runs first (higher version, reversed order) and drops
-- the CHECK constraint that reads 'enrolled'/'removed', which is what
-- makes rebuilding the enum back to three values safe here. Postgres has
-- no ALTER TYPE ... DROP VALUE (00062's own down explains this); safe
-- only against a disposable test/dev database (CLAUDE.md: no production
-- data) -- a real 'enrolled' or 'removed' row would fail the USING cast.
ALTER TYPE staff_auth_event_reason RENAME TO staff_auth_event_reason_old;
CREATE TYPE staff_auth_event_reason AS ENUM ('owner_vouched', 'self_service', 'support');
ALTER TABLE staff_auth_events
    ALTER COLUMN reason TYPE staff_auth_event_reason
    USING reason::text::staff_auth_event_reason;
DROP TYPE staff_auth_event_reason_old;
