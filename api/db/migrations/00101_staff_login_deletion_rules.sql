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
-- untouched, and *this* policy admits exactly one shape of write: the
-- caller's own row (USING), in the pre-Practice window, leaving with
-- deleted_at stamped and identity_uid holding the sentinel spelled out
-- below (WITH CHECK).
--
-- What that does and does not buy, said plainly rather than left to be
-- discovered. It buys the sentinel: 00044 refuses any write that walks
-- the row away from the caller's own identity_uid, so before this policy
-- the redaction was impossible, and the sentinel's exact spelling now
-- lives at a boundary that can enforce it. It does *not* make the
-- redaction the only write she can make to her own row -- 00044 is
-- row-level by its own stated design ("this permits her to update any
-- column of her own row"), so stamping deleted_at while keeping her
-- identity_uid is still admitted, by that policy rather than this one.
-- Narrowing 00044 to close that would take the work-state self-edit down
-- with it, for a write no route exposes. The pairing of the stamp and
-- the sentinel is therefore the handler's guarantee -- one statement,
-- checked to have affected exactly one row -- and this policy's job is
-- to make that one statement possible at all.
-- One more thing this policy does not do on its own, worth knowing here
-- because the code that depends on it is two packages away. Postgres
-- checks a table's SELECT policies against the *new* row of an UPDATE,
-- and the new row's identity_uid is the sentinel, which
-- staff_self_visibility (00006) does not match. So the redaction also
-- needs a SELECT policy that admits the row it is about to become, and
-- the only one that does is staff_notification_worker (00033) --
-- DeleteLoginHandler sets app.notification_worker_trusted for what looks
-- like an unrelated reason (reading across every Practice she belongs
-- to) and this is the second thing that flag buys.
--
-- staffauth's rls_test.go pins every case above, including that one.
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
