-- +goose Up
-- The half of #892's schema change that can run inside a transaction:
-- the actor-shape CHECK that has to name 00100's new enum value, and the
-- one RLS policy the redaction needs. Split for the reason 00072 states
-- -- Postgres forbids using an ALTER TYPE ... ADD VALUE in the same
-- transaction that added it.

-- 'login_deleted' joins the branch that demands a Staff actor: she runs
-- the act on herself, so actor_staff_id equals staff_id, exactly as
-- 'self_service' already does.
ALTER TABLE staff_auth_events DROP CONSTRAINT staff_auth_events_actor_shape;
ALTER TABLE staff_auth_events ADD CONSTRAINT staff_auth_events_actor_shape CHECK (
    (reason IN ('owner_vouched', 'self_service', 'enrolled', 'removed', 'login_deleted')
        AND actor_staff_id IS NOT NULL AND actor_operator IS NULL)
    OR (reason = 'support' AND actor_operator IS NOT NULL AND actor_operator <> '' AND actor_staff_id IS NULL)
);

-- ---------------------------------------------------------------------
-- The redaction's own UPDATE policy.
--
-- 00044's staff_self_update cannot admit this write and must not be
-- widened to. Its WITH CHECK repeats its USING -- the row on the way out
-- must still carry the caller's own identity_uid -- and the whole point
-- of the redaction is that the row on the way out does not: identity_uid
-- is NOT NULL UNIQUE, so it takes a sentinel derived from the row's own
-- id rather than a null or a shared constant, and that is precisely the
-- shape 00044 refuses.
--
-- So a second policy, not an edit to the first. Postgres ORs permissive
-- policies together, so the work-state self-edit 00044 exists for is
-- untouched, and this one admits exactly one shape of write and no
-- other: the caller's own row, in the pre-Practice window, leaving with
-- deleted_at stamped and identity_uid holding the sentinel this policy
-- names in full. A caller cannot reach another person's row (USING), and
-- cannot use this policy to write anything but the redaction (WITH
-- CHECK) -- the sentinel's exact spelling lives here, at the boundary
-- that can actually enforce it, as well as in the handler.
--
-- The sentinel is 'deleted:' || id, which is unique because id is, and
-- which cannot collide with an Identity Platform uid (those are 28
-- alphanumeric characters with no colon).
-- +goose StatementBegin
CREATE POLICY staff_self_login_deletion ON staff
    FOR UPDATE
    USING (
        NULLIF(current_setting('app.current_practice_id', true), '') IS NULL
        AND identity_uid = NULLIF(current_setting('app.current_identity_uid', true), '')
    )
    WITH CHECK (
        NULLIF(current_setting('app.current_practice_id', true), '') IS NULL
        AND deleted_at IS NOT NULL
        AND identity_uid = 'deleted:' || id::text
    );
-- +goose StatementEnd

-- +goose Down
DROP POLICY staff_self_login_deletion ON staff;

-- The rows go before the constraint that would refuse them and before
-- the enum that spells them: a restored CHECK validates what is already
-- on file, so a surviving 'login_deleted' row would fail both of its
-- branches. Pre-launch, no users, no production data (CLAUDE.md).
DELETE FROM staff_auth_events WHERE reason = 'login_deleted';

ALTER TABLE staff_auth_events DROP CONSTRAINT staff_auth_events_actor_shape;
ALTER TABLE staff_auth_events ADD CONSTRAINT staff_auth_events_actor_shape CHECK (
    (reason IN ('owner_vouched', 'self_service', 'enrolled', 'removed')
        AND actor_staff_id IS NOT NULL AND actor_operator IS NULL)
    OR (reason = 'support' AND actor_operator IS NOT NULL AND actor_operator <> '' AND actor_staff_id IS NULL)
);

-- 00100 cannot drop the enum value it added -- Postgres has no ALTER
-- TYPE ... DROP VALUE -- so the rebuild lives here, in the half that
-- runs inside a transaction.
ALTER TYPE staff_auth_event_reason RENAME TO staff_auth_event_reason_old;
CREATE TYPE staff_auth_event_reason AS ENUM
    ('owner_vouched', 'self_service', 'support', 'enrolled', 'removed');
ALTER TABLE staff_auth_events
    ALTER COLUMN reason TYPE staff_auth_event_reason
    USING reason::text::staff_auth_event_reason;
DROP TYPE staff_auth_event_reason_old;
