-- +goose Up
-- #1537: a Staff member has a first name and a last name.
--
-- Until now staff.name was one string, asked as "Your name". The design
-- brief's Voice rule (rule 2, applied to Staff by #663) says the product
-- uses a person's first name from the moment a screen knows it, and one
-- field cannot supply a first name reliably: "Mary Anne Smith" has no
-- safe split. #1493 decided on two fields, stored in two columns.
--
-- What happens to the old column. `name` stays, as a GENERATED ALWAYS ...
-- STORED column holding the first name, a space and the last name. It is
-- no longer written by anyone; first_name and last_name are the only
-- source. It stays because roughly forty read paths print a Staff
-- member's full name (rosters, Visits, Messages, the Activity ledger,
-- the on-call grid, the Client portal), and every one of them wants
-- exactly "first then last". A generated column keeps all of them
-- correct in one place instead of forty copies of a concatenation, and
-- it cannot drift from the two columns it is built from. Anything that
-- sorts people sorts on last_name, then first_name instead.
--
-- Existing rows (dev, CI and the deployed database; there are no users).
-- The old string is split at its first run of white space: everything
-- before it is the first name, everything after is the last name. That
-- is right for "Maya Okafor" and for "Mary Anne Smith" read as first
-- "Mary", last "Anne Smith" -- the one case a script cannot know; a
-- person corrects hers on /account. A one-word name leaves last_name
-- empty. "Deleted Staff Member" (00100) becomes first "Deleted", last
-- "Staff Member", and prints back as the same sentinel.
--
-- first_name and last_name are NOT NULL with a default of '' so the
-- statement is safe on a populated table (the #1022 guardrail refuses a
-- NOT NULL column with no default). The default is dropped straight
-- after the backfill, so a later INSERT that forgets a name fails
-- instead of quietly storing an empty one. Emptiness is refused at the
-- API boundary ("Enter your first name", "Enter your last name"); there
-- is no CHECK, because a pre-existing one-word name would violate it.
ALTER TABLE staff ADD COLUMN first_name text NOT NULL DEFAULT '';
ALTER TABLE staff ADD COLUMN last_name text NOT NULL DEFAULT '';

UPDATE staff
   SET first_name = COALESCE(substring(btrim(name) from '^\S+'), ''),
       last_name  = COALESCE(btrim(substring(btrim(name) from '^\S+\s+(.*)$')), '');

ALTER TABLE staff ALTER COLUMN first_name DROP DEFAULT;
ALTER TABLE staff ALTER COLUMN last_name DROP DEFAULT;

ALTER TABLE staff DROP COLUMN name;
ALTER TABLE staff ADD COLUMN name text
    GENERATED ALWAYS AS (btrim(first_name || ' ' || last_name)) STORED;

-- ---------------------------------------------------------------------
-- staff_name_events: how the current name came to be. The shape
-- staff_work_state_events (00043, 00044) has, for the same reason -- the
-- audit-trail expectation: a person must be able to answer "who changed
-- this name, and when?". Signup and Invitation acceptance write the
-- first event (kind 'stated'); a correction on /account writes a
-- 'changed' one. actor_staff_id exists so the question stays answerable
-- if a later ticket widens who may write; today it equals staff_id.
--
-- The event deliberately holds no name. A name is the personal data
-- ADR-0033's login deletion redacts, and an event is append-only (no
-- UPDATE grant), so a copy of the name here could never be redacted.
-- What the ledger answers is who changed it and when; what it was is on
-- the row, and a deleted login's row is redacted in one place.
--
-- No UPDATE, no DELETE grant: an event is a fact about the past.
-- ---------------------------------------------------------------------
CREATE TABLE staff_name_events (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    staff_id       uuid NOT NULL REFERENCES staff (id),
    kind           text NOT NULL CHECK (kind IN ('stated', 'changed')),
    actor_staff_id uuid NOT NULL REFERENCES staff (id),
    created_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX staff_name_events_staff
    ON staff_name_events (staff_id, created_at);

GRANT SELECT, INSERT ON staff_name_events TO app_runtime;

ALTER TABLE staff_name_events ENABLE ROW LEVEL SECURITY;

-- Visible to a Practice a person holds a Membership at, like her
-- work-state events; writable in the pre-Practice window of /account.
CREATE POLICY staff_name_events_practice_visibility ON staff_name_events
    USING (
        EXISTS (
            SELECT 1 FROM practice_memberships pm
            WHERE pm.staff_id = staff_name_events.staff_id
              AND pm.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        )
    );

CREATE POLICY staff_name_events_self ON staff_name_events
    USING (
        NULLIF(current_setting('app.current_practice_id', true), '') IS NULL
        AND staff_id = current_staff_id()
    )
    WITH CHECK (
        NULLIF(current_setting('app.current_practice_id', true), '') IS NULL
        AND staff_id = current_staff_id()
        AND actor_staff_id = current_staff_id()
    );

-- +goose Down
DROP TABLE staff_name_events;
ALTER TABLE staff DROP COLUMN name;
ALTER TABLE staff ADD COLUMN name text;
UPDATE staff SET name = btrim(first_name || ' ' || last_name);
ALTER TABLE staff ALTER COLUMN name SET NOT NULL;
ALTER TABLE staff DROP COLUMN last_name;
ALTER TABLE staff DROP COLUMN first_name;
