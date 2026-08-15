-- +goose Up
-- Database foundation for staff-to-client Messaging (#57, part of #56):
-- an immutable, append-only per-Engagement thread with an optional
-- single attachment, plus a population-agnostic push_subscriptions table
-- so a later notification type can reuse it without a schema change.
-- Not user-facing on its own -- no handlers are added here, only the
-- schema and RLS a future ticket's handlers will run against.

CREATE TYPE actor_type AS ENUM ('staff', 'client');

-- A Message always belongs to exactly one Engagement. sender_id has no
-- FK -- it points at staff.id when sender_type = 'staff' or clients.id
-- when sender_type = 'client', and Postgres has no polymorphic FK.
-- Attachment fields live directly on the row rather than a separate
-- table: a Message is capped at one attachment, so a join would only add
-- complexity, not flexibility. No updated_at/soft-delete column --
-- immutability comes from never exposing an update or delete endpoint,
-- not from the schema.
CREATE TABLE messages (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    engagement_id uuid NOT NULL REFERENCES engagements (id),
    sender_type actor_type NOT NULL,
    sender_id uuid NOT NULL,
    body text,
    attachment_object_path text,
    attachment_content_type text,
    attachment_byte_size bigint,
    attachment_filename text,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT messages_has_content
        CHECK (body IS NOT NULL OR attachment_object_path IS NOT NULL),
    CONSTRAINT messages_attachment_all_or_nothing
        CHECK (
            (attachment_object_path IS NULL) = (attachment_content_type IS NULL)
            AND (attachment_object_path IS NULL) = (attachment_byte_size IS NULL)
            AND (attachment_object_path IS NULL) = (attachment_filename IS NULL)
        )
);

-- Links a Staff or Client identity to a Web Push subscription for one
-- device/browser. Deliberately carries no Message-specific columns so a
-- later notification type (Invoice-due, Contract-ready) can reuse it
-- without a migration. owner_id, like messages.sender_id, has no FK for
-- the same polymorphic reason.
CREATE TABLE push_subscriptions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_type actor_type NOT NULL,
    owner_id uuid NOT NULL,
    endpoint text NOT NULL UNIQUE,
    p256dh_key text NOT NULL,
    auth_key text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

GRANT SELECT, INSERT, UPDATE, DELETE ON messages, push_subscriptions TO app_runtime;

ALTER TABLE messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE push_subscriptions ENABLE ROW LEVEL SECURITY;

-- messages has no practice_id column, so its practice-tier visibility is
-- an EXISTS subquery against engagements, the same shape as
-- visits_practice_visibility in 00007_visit.sql. No chicken-and-egg
-- problem: a Message is always created under an Engagement that already
-- exists, so a single ALL-command policy covers SELECT/INSERT (UPDATE
-- and DELETE are never exercised -- no such endpoint exists -- but the
-- policy permits them at the RLS layer, same as engagements_practice_visibility
-- does for engagements).
CREATE POLICY messages_practice_visibility ON messages
    USING (
        EXISTS (
            SELECT 1 FROM engagements e
            WHERE e.id = messages.engagement_id
              AND e.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        )
    );

-- Client-tier visibility, the second of the two tiers the ticket asks
-- for: messages carries no client_id column directly, so unlike
-- engagements_client_visibility (which compares client_id directly) this
-- is an EXISTS subquery through engagements to client_id, the same shape
-- as messages_practice_visibility above. An ALL-command policy, not
-- SELECT-only: a Client sends messages too, unlike a Client-portal
-- caller creating an Engagement.
CREATE POLICY messages_client_visibility ON messages
    USING (
        EXISTS (
            SELECT 1 FROM engagements e
            WHERE e.id = messages.engagement_id
              AND e.client_id = NULLIF(current_setting('app.current_client_id', true), '')::uuid
        )
    );

-- push_subscriptions is scoped to the owning identity, not to a
-- Practice/Engagement -- a Staff member's own subscriptions regardless of
-- which Practice is currently selected. current_staff_id() (00003)
-- resolves from app.current_identity_uid alone, independent of
-- app.current_practice_id, so unlike messages_practice_visibility above
-- this needs an explicit guard against app.current_client_id being set --
-- mirroring the 00006 widening of staff_self_visibility -- or a person
-- sharing one identity_uid across both a staff row and a
-- client_portal_users row would have their Staff push_subscriptions row
-- leak into a Client-portal-scoped request.
CREATE POLICY push_subscriptions_staff_visibility ON push_subscriptions
    USING (
        NULLIF(current_setting('app.current_client_id', true), '') IS NULL
        AND owner_type = 'staff'
        AND owner_id = current_staff_id()
    );

-- Mirrors push_subscriptions_staff_visibility for the Client-portal
-- population. The guard against app.current_practice_id being set is
-- belt-and-suspenders here: unlike current_staff_id(), a direct
-- app.current_client_id comparison already evaluates to NULL (so matches
-- nothing) during a Staff request where it's never set.
CREATE POLICY push_subscriptions_client_visibility ON push_subscriptions
    USING (
        NULLIF(current_setting('app.current_practice_id', true), '') IS NULL
        AND owner_type = 'client'
        AND owner_id = NULLIF(current_setting('app.current_client_id', true), '')::uuid
    );

-- +goose Down
DROP TABLE push_subscriptions;
DROP TABLE messages;
DROP TYPE actor_type;
