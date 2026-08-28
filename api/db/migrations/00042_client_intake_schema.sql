-- +goose Up
-- ADR-0017 (docs/adr/0017-twelve-columns-a-practice-defined-layer-and-an-
-- engagement-that-is-asked-for.md), Concrete schema section, applied as
-- specified there (#396). It overlaps ADR-0015's specified migration and
-- takes only the part intake writes; the remainder -- the status set,
-- birth outcome, ending reason, the immutability trigger, portal_accounts,
-- the accept-time 409 -- stays with that document.
--
-- No production data (pre-launch, CLAUDE.md) and no earlier migration
-- inserts a row into clients or engagements, so every column added below
-- as NOT NULL lands on an empty table -- no backfill UPDATE belongs in
-- this file, the same reasoning 00029 already applied to
-- practices.stripe_connect_*. The "backfill" ADR-0017 and this ticket's
-- thread call for is in the Go test fixtures across the repo (helpers_test.go
-- files), not here.

-- =====================================================================
-- clients: a Practice, twelve structural columns, and a values blob
-- =====================================================================

ALTER TABLE clients ADD COLUMN practice_id uuid NOT NULL REFERENCES practices (id);

ALTER TABLE clients DROP COLUMN name;
ALTER TABLE clients ADD COLUMN given_name          text NOT NULL;
ALTER TABLE clients ADD COLUMN family_name         text;
ALTER TABLE clients ADD COLUMN preferred_name      text;
ALTER TABLE clients ALTER COLUMN email DROP NOT NULL;
ALTER TABLE clients ADD COLUMN phone               text;
ALTER TABLE clients ADD COLUMN address_line1       text;
ALTER TABLE clients ADD COLUMN address_line2       text;
ALTER TABLE clients ADD COLUMN address_locality    text;
ALTER TABLE clients ADD COLUMN address_region      text;
ALTER TABLE clients ADD COLUMN address_postal_code text;
ALTER TABLE clients ADD COLUMN date_of_birth       date;

-- Practice-defined values, keyed on field id. Shaped like
-- plan_instances.answers, but read against the Practice's *live*
-- template rather than a snapshot -- ADR-0017's departure from ADR-0001.
ALTER TABLE clients ADD COLUMN field_values jsonb NOT NULL DEFAULT '{}'::jsonb;

-- clients_select collapses onto practice_id now that clients carries it
-- directly, the same shape as engagements_practice_visibility (00005).
DROP POLICY clients_select ON clients;
-- +goose StatementBegin
CREATE POLICY clients_select ON clients
    FOR SELECT
    USING (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid);
-- +goose StatementEnd

-- clients_insert is replaced, not deleted: the chicken-and-egg reason it
-- existed (00005 -- a Client had no Practice of her own at insert time)
-- is gone now that practice_id rides the insert. The new WITH CHECK
-- verifies the practice_id being written matches the caller's Practice
-- context AND that the caller's own Membership there is not a contractor
-- Doula's (ADR-0017's write table: a contractor originates no Client).
DROP POLICY clients_insert ON clients;
-- +goose StatementBegin
CREATE POLICY clients_insert ON clients
    FOR INSERT
    WITH CHECK (
        practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        AND EXISTS (
            SELECT 1 FROM practice_memberships pm
            WHERE pm.staff_id = NULLIF(current_setting('app.current_staff_id', true), '')::uuid
              AND pm.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
              AND pm.employment_type <> 'contractor'
        )
    );
-- +goose StatementEnd

-- Whoever may read a Client may edit her (ADR-0017: "edit follows read"),
-- so this follows clients_select's shape exactly.
-- +goose StatementBegin
CREATE POLICY clients_update ON clients
    FOR UPDATE
    USING (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid);
-- +goose StatementEnd

-- client_portal_users (00026) reached a Practice through engagements
-- because clients had no practice_id of its own. All three of its
-- policies that did so collapse to a one-hop EXISTS against clients
-- instead -- client_portal_users still has no practice_id column, so
-- this is not a plain comparison, but it no longer needs to reach through
-- engagements to get there.
DROP POLICY client_portal_users_invite_insert ON client_portal_users;
-- +goose StatementBegin
CREATE POLICY client_portal_users_invite_insert ON client_portal_users
    FOR INSERT
    WITH CHECK (
        identity_uid IS NULL
        AND EXISTS (
            SELECT 1 FROM clients c
            WHERE c.id = client_portal_users.client_id
              AND c.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        )
    );
-- +goose StatementEnd

DROP POLICY client_portal_users_practice_visibility ON client_portal_users;
-- +goose StatementBegin
CREATE POLICY client_portal_users_practice_visibility ON client_portal_users
    FOR SELECT
    USING (
        EXISTS (
            SELECT 1 FROM clients c
            WHERE c.id = client_portal_users.client_id
              AND c.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        )
    );
-- +goose StatementEnd

DROP POLICY client_portal_users_invite_update ON client_portal_users;
-- +goose StatementBegin
CREATE POLICY client_portal_users_invite_update ON client_portal_users
    FOR UPDATE
    USING (
        identity_uid IS NULL
        AND EXISTS (
            SELECT 1 FROM clients c
            WHERE c.id = client_portal_users.client_id
              AND c.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        )
    )
    WITH CHECK (identity_uid IS NULL);
-- +goose StatementEnd

-- =====================================================================
-- client_field_templates
-- =====================================================================

CREATE TABLE client_field_templates (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    practice_id uuid NOT NULL REFERENCES practices (id),
    fields      jsonb NOT NULL DEFAULT '[]'::jsonb,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX client_field_templates_one_per_practice
    ON client_field_templates (practice_id);

-- One row per Practice, never deleted -- an Owner or Admin edits fields
-- into and out of the "fields" array (archiving, not row deletion), so
-- no DELETE grant.
GRANT SELECT, INSERT, UPDATE ON client_field_templates TO app_runtime;

ALTER TABLE client_field_templates ENABLE ROW LEVEL SECURITY;

-- +goose StatementBegin
CREATE POLICY client_field_templates_practice_visibility ON client_field_templates
    USING (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid);
-- +goose StatementEnd

-- =====================================================================
-- client_events
-- =====================================================================

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

GRANT SELECT, INSERT ON client_events TO app_runtime;   -- no UPDATE, no DELETE -- append-only

ALTER TABLE client_events ENABLE ROW LEVEL SECURITY;

-- Same single ADR-0008 row as the record it describes: practice-tier,
-- never reachable from a portal session (no client-tier policy here).
-- +goose StatementBegin
CREATE POLICY client_events_practice_visibility ON client_events
    USING (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid);
-- +goose StatementEnd

-- =====================================================================
-- engagement_kind, engagement_requests
-- =====================================================================

-- ADR-0015's enum, created here because this is the effort whose intake
-- screen writes it (used by both engagement_requests below and
-- engagements.kind further down).
CREATE TYPE engagement_kind AS ENUM ('birth', 'postpartum');

CREATE TYPE engagement_request_state AS ENUM
    ('pending', 'approved', 'refused', 'withdrawn');

CREATE TABLE engagement_requests (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    practice_id   uuid NOT NULL REFERENCES practices (id),
    client_id     uuid NOT NULL REFERENCES clients (id),
    kind          engagement_kind NOT NULL,
    due_date      date,
    note          text,
    state         engagement_request_state NOT NULL DEFAULT 'pending',
    requested_by  uuid NOT NULL REFERENCES staff (id),
    requested_at  timestamptz NOT NULL DEFAULT now(),
    decided_by    uuid REFERENCES staff (id),
    decided_at    timestamptz,
    reason        text,
    engagement_id uuid REFERENCES engagements (id),
    CONSTRAINT engagement_requests_decision CHECK (
        (state =  'pending' AND decided_by IS NULL     AND decided_at IS NULL)
     OR (state <> 'pending' AND decided_by IS NOT NULL AND decided_at IS NOT NULL)
    ),
    CONSTRAINT engagement_requests_refusal_reason CHECK (
        state <> 'refused' OR reason IS NOT NULL
    ),
    CONSTRAINT engagement_requests_approval_engagement CHECK (
        (state = 'approved') = (engagement_id IS NOT NULL)
    )
);

-- At most one pending Request per Client *per kind*, so two Doulas
-- cannot both ask for the same woman's birth package and spend two
-- Credits on one piece of work -- while a Client buying a birth package
-- and a postpartum package at intake is still one visit to the screen.
-- Same partial-index idiom as engagement_offer_outbox_one_pending (00041).
CREATE UNIQUE INDEX engagement_requests_one_pending
    ON engagement_requests (client_id, kind)
    WHERE state = 'pending';

GRANT SELECT, INSERT, UPDATE ON engagement_requests TO app_runtime;   -- no DELETE

ALTER TABLE engagement_requests ENABLE ROW LEVEL SECURITY;

-- +goose StatementBegin
CREATE POLICY engagement_requests_practice_visibility ON engagement_requests
    USING (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid);
-- +goose StatementEnd

-- =====================================================================
-- engagement_request_outbox
-- =====================================================================

-- Same shape as staff_invite_outbox (00038) and engagement_offer_outbox
-- (00041), row for row, except the recipient: a Request mails every Owner
-- and every Admin at the Practice (ADR-0010, ADR-0017), not one fixed
-- address, so the row itself names who it's for -- and the partial unique
-- index is keyed on the (request, recipient) pair rather than on the
-- Request alone. No secret column: unlike an Invitation or an Offer, a
-- Request carries no bearer token for the worker to read at send time.
CREATE TYPE engagement_request_outbox_status AS ENUM ('pending', 'sent', 'dead_lettered');

CREATE TABLE engagement_request_outbox (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id      uuid NOT NULL REFERENCES engagement_requests (id),
    staff_id        uuid NOT NULL REFERENCES staff (id),
    status          engagement_request_outbox_status NOT NULL DEFAULT 'pending',
    attempt_count   int NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    created_at      timestamptz NOT NULL DEFAULT now(),
    sent_at         timestamptz,
    last_error      text
);

CREATE UNIQUE INDEX engagement_request_outbox_one_pending_per_recipient
    ON engagement_request_outbox (request_id, staff_id)
    WHERE status = 'pending';

GRANT SELECT, INSERT, UPDATE ON engagement_request_outbox TO app_runtime;

-- No RLS -- platform-level like every other outbox table: the worker runs
-- with no Practice or Client session context.

-- The worker's own trusted-session context has no Practice set, so it
-- needs the same app.notification_worker_trusted door 00038/00041 already
-- opened on other tables, applied here so it can check a Request's
-- current state (skip one already decided or withdrawn before this row
-- was sent) and stamp its own bookkeeping. This does not license the
-- mailed body to say more: Notifications stay content-free per
-- CONTEXT.md -- no kind, due date, or Client name in the message itself,
-- only a pointer back to the dashboard, the same restraint
-- engagement_offers_notification_worker (00041) already observes.
-- +goose StatementBegin
CREATE POLICY engagement_requests_notification_worker ON engagement_requests
    FOR SELECT
    USING (current_setting('app.notification_worker_trusted', true) = 'true');
-- +goose StatementEnd

-- =====================================================================
-- engagements: kind and due_date
-- =====================================================================

-- kind is NOT NULL with no database default, exactly as ADR-0015
-- specifies it -- a default would be a second opinion about what the
-- Practice sold. Nothing else from ADR-0015's engagements section is
-- built here.
ALTER TABLE engagements ADD COLUMN kind     engagement_kind NOT NULL;
ALTER TABLE engagements ADD COLUMN due_date date;

-- +goose Down
ALTER TABLE engagements DROP COLUMN due_date;
ALTER TABLE engagements DROP COLUMN kind;

DROP POLICY engagement_requests_notification_worker ON engagement_requests;
DROP TABLE engagement_request_outbox;
DROP TYPE engagement_request_outbox_status;

DROP POLICY engagement_requests_practice_visibility ON engagement_requests;
DROP TABLE engagement_requests;
DROP TYPE engagement_request_state;
DROP TYPE engagement_kind;

DROP POLICY client_events_practice_visibility ON client_events;
DROP TABLE client_events;
DROP TYPE client_event_actor;
DROP TYPE client_event_type;

DROP POLICY client_field_templates_practice_visibility ON client_field_templates;
DROP TABLE client_field_templates;

DROP POLICY client_portal_users_invite_update ON client_portal_users;
-- +goose StatementBegin
CREATE POLICY client_portal_users_invite_update ON client_portal_users
    FOR UPDATE
    USING (
        identity_uid IS NULL
        AND EXISTS (
            SELECT 1 FROM engagements e
            WHERE e.client_id = client_portal_users.client_id
              AND e.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        )
    )
    WITH CHECK (identity_uid IS NULL);
-- +goose StatementEnd

DROP POLICY client_portal_users_practice_visibility ON client_portal_users;
-- +goose StatementBegin
CREATE POLICY client_portal_users_practice_visibility ON client_portal_users
    FOR SELECT
    USING (
        EXISTS (
            SELECT 1 FROM engagements e
            WHERE e.client_id = client_portal_users.client_id
              AND e.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        )
    );
-- +goose StatementEnd

DROP POLICY client_portal_users_invite_insert ON client_portal_users;
-- +goose StatementBegin
CREATE POLICY client_portal_users_invite_insert ON client_portal_users
    FOR INSERT
    WITH CHECK (
        identity_uid IS NULL
        AND EXISTS (
            SELECT 1 FROM engagements e
            WHERE e.client_id = client_portal_users.client_id
              AND e.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        )
    );
-- +goose StatementEnd

DROP POLICY clients_update ON clients;

DROP POLICY clients_insert ON clients;
-- +goose StatementBegin
CREATE POLICY clients_insert ON clients
    FOR INSERT
    WITH CHECK (NULLIF(current_setting('app.current_practice_id', true), '') IS NOT NULL);
-- +goose StatementEnd

DROP POLICY clients_select ON clients;
-- +goose StatementBegin
CREATE POLICY clients_select ON clients
    FOR SELECT
    USING (
        EXISTS (
            SELECT 1 FROM engagements e
            WHERE e.client_id = clients.id
              AND e.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        )
    );
-- +goose StatementEnd

ALTER TABLE clients DROP COLUMN field_values;
ALTER TABLE clients DROP COLUMN date_of_birth;
ALTER TABLE clients DROP COLUMN address_postal_code;
ALTER TABLE clients DROP COLUMN address_region;
ALTER TABLE clients DROP COLUMN address_locality;
ALTER TABLE clients DROP COLUMN address_line2;
ALTER TABLE clients DROP COLUMN address_line1;
ALTER TABLE clients DROP COLUMN phone;
ALTER TABLE clients ALTER COLUMN email SET NOT NULL;
ALTER TABLE clients DROP COLUMN preferred_name;
ALTER TABLE clients DROP COLUMN family_name;
ALTER TABLE clients DROP COLUMN given_name;
ALTER TABLE clients ADD COLUMN name text NOT NULL;

ALTER TABLE clients DROP COLUMN practice_id;
