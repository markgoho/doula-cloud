-- +goose Up
-- ADR-0022 (docs/adr/0022-one-activity-log-with-a-subject-and-three-kinds-
-- of-actor.md): one append-only audit log, keyed by subject_kind and
-- subject_id, replacing the three tables that had each independently
-- rediscovered the same shape -- practice_membership_events (00039),
-- client_events (00042) and client_field_template_events (00050). No
-- production data exists (pre-launch, CLAUDE.md), so this drops all
-- three whole rather than migrating rows.
--
-- actor_kind is staff | client | system, not staff | system: a Client's
-- own signature or payment is her act, not the product's, and the ADR
-- rejects folding it into 'system'. actor_staff_id/actor_client_id are
-- both nullable and the CHECK below ties exactly one to its actor_kind,
-- the same belt-and-braces client_events_actor (00042) already used for
-- two kinds.

CREATE TYPE activity_actor_kind AS ENUM ('staff', 'client', 'system');

CREATE TABLE activity (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    practice_id     uuid NOT NULL REFERENCES practices (id),
    subject_kind    text NOT NULL,
    subject_id      uuid NOT NULL,
    action          text NOT NULL,
    diff            jsonb NOT NULL,
    actor_kind      activity_actor_kind NOT NULL,
    actor_staff_id  uuid REFERENCES staff (id),
    actor_client_id uuid REFERENCES clients (id),
    created_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT activity_actor CHECK (
        (actor_kind = 'staff'  AND actor_staff_id  IS NOT NULL AND actor_client_id IS NULL)
        OR (actor_kind = 'client' AND actor_client_id IS NOT NULL AND actor_staff_id IS NULL)
        OR (actor_kind = 'system' AND actor_staff_id  IS NULL     AND actor_client_id IS NULL)
    )
);

-- The one query shape every reader needs: a subject's own rows, newest
-- first, scoped to its Practice (RLS restates the last part, this index
-- serves it).
CREATE INDEX activity_subject ON activity (practice_id, subject_kind, subject_id, created_at);

GRANT SELECT, INSERT ON activity TO app_runtime;   -- no UPDATE, no DELETE -- append-only

ALTER TABLE activity ENABLE ROW LEVEL SECURITY;

-- One policy for the whole table, the shape practice_membership_events
-- and client_events (the majority of the three predecessors) already
-- used: a plain practice_id comparison, admitting SELECT and INSERT
-- alike. client_field_template_events (00050) additionally required an
-- Owner/Admin role and a self-named actor on INSERT; that check is not
-- carried forward here; it is enforced at the Go handler
-- (staffauth.RequireOwnerOrAdmin) instead, and it could not be carried
-- forward unchanged regardless -- accept.go and signup.go write a
-- 'membership' row before app.current_staff_id is ever set (there is no
-- session yet to hold it), so a table-wide self-actor check would break
-- them.
-- +goose StatementBegin
CREATE POLICY activity_practice_visibility ON activity
    USING (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid);
-- +goose StatementEnd

-- ---------------------------------------------------------------------
-- Drop the three tables this replaces.
-- ---------------------------------------------------------------------

DROP POLICY practice_membership_events_practice_visibility ON practice_membership_events;
DROP TABLE practice_membership_events;
DROP TYPE membership_event_type;

DROP POLICY client_events_practice_visibility ON client_events;
DROP TABLE client_events;
DROP TYPE client_event_actor;
DROP TYPE client_event_type;

DROP POLICY client_field_template_events_insert ON client_field_template_events;
DROP POLICY client_field_template_events_select ON client_field_template_events;
DROP TABLE client_field_template_events;

-- +goose Down

CREATE TABLE client_field_template_events (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    practice_id    uuid NOT NULL REFERENCES practices (id),
    diff           jsonb NOT NULL,
    actor_staff_id uuid NOT NULL REFERENCES staff (id),
    created_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX client_field_template_events_practice ON client_field_template_events (practice_id, created_at);

GRANT SELECT, INSERT ON client_field_template_events TO app_runtime;

ALTER TABLE client_field_template_events ENABLE ROW LEVEL SECURITY;

-- +goose StatementBegin
CREATE POLICY client_field_template_events_select ON client_field_template_events
    FOR SELECT
    USING (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE POLICY client_field_template_events_insert ON client_field_template_events
    FOR INSERT
    WITH CHECK (
        practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        AND actor_staff_id = NULLIF(current_setting('app.current_staff_id', true), '')::uuid
        AND EXISTS (
            SELECT 1 FROM practice_memberships pm
            WHERE pm.staff_id = actor_staff_id
              AND pm.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
              AND pm.roles && ARRAY['owner', 'admin']::practice_role[]
        )
    );
-- +goose StatementEnd

CREATE TYPE client_event_type  AS ENUM ('created', 'updated');
CREATE TYPE client_event_actor AS ENUM ('staff', 'system');

CREATE TABLE client_events (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    practice_id    uuid NOT NULL REFERENCES practices (id),
    client_id      uuid NOT NULL REFERENCES clients (id),
    event_type     client_event_type NOT NULL,
    diff           jsonb NOT NULL,
    actor_kind     client_event_actor NOT NULL,
    actor_staff_id uuid REFERENCES staff (id),
    created_at     timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT client_events_actor CHECK (
        (actor_kind = 'staff'  AND actor_staff_id IS NOT NULL)
        OR (actor_kind = 'system' AND actor_staff_id IS NULL)
    )
);

CREATE INDEX client_events_client ON client_events (practice_id, client_id, created_at);
CREATE INDEX client_events_diff   ON client_events USING gin (diff);

GRANT SELECT, INSERT ON client_events TO app_runtime;

ALTER TABLE client_events ENABLE ROW LEVEL SECURITY;

-- +goose StatementBegin
CREATE POLICY client_events_practice_visibility ON client_events
    USING (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid);
-- +goose StatementEnd

CREATE TYPE membership_event_type AS ENUM
    ('joined', 'roles_changed', 'employment_type_changed', 'removed');

CREATE TABLE practice_membership_events (
    id                       uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    practice_id              uuid NOT NULL REFERENCES practices (id),
    staff_id                 uuid NOT NULL REFERENCES staff (id),
    event_type               membership_event_type NOT NULL,
    previous_roles           practice_role[],
    roles                    practice_role[],
    previous_employment_type employment_type,
    employment_type          employment_type,
    actor_staff_id           uuid NOT NULL REFERENCES staff (id),
    created_at               timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX practice_membership_events_membership
    ON practice_membership_events (practice_id, staff_id, created_at);

GRANT SELECT, INSERT ON practice_membership_events TO app_runtime;

ALTER TABLE practice_membership_events ENABLE ROW LEVEL SECURITY;

CREATE POLICY practice_membership_events_practice_visibility ON practice_membership_events
    USING (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid);

DROP POLICY activity_practice_visibility ON activity;
DROP TABLE activity;
DROP TYPE activity_actor_kind;
