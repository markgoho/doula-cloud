-- +goose Up
-- A "Client" is the pregnant/birthing person a Practice serves. It is a
-- global table (one row per person, no practice_id), the same shape as
-- "staff" in 00002_practice_staff_tenancy.sql -- a Client can, in
-- principle, have Engagements at more than one Practice over time.
--
-- An "Engagement" is the relationship between a Client and a Practice,
-- spanning intake through postpartum. It carries both client_id and
-- practice_id, and is what actually ties a Client to a given Practice.

CREATE TYPE engagement_status AS ENUM ('intake', 'active', 'postpartum', 'completed');

CREATE TABLE clients (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL,
    email text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE engagements (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id uuid NOT NULL REFERENCES clients (id),
    practice_id uuid NOT NULL REFERENCES practices (id),
    status engagement_status NOT NULL DEFAULT 'intake',
    created_at timestamptz NOT NULL DEFAULT now()
);

GRANT SELECT, INSERT, UPDATE, DELETE ON clients, engagements TO app_runtime;

ALTER TABLE clients ENABLE ROW LEVEL SECURITY;
ALTER TABLE engagements ENABLE ROW LEVEL SECURITY;

-- clients has no practice_id column (it is a global entity), so its
-- practice-tier visibility is an EXISTS subquery against engagements: a
-- Client row is visible if it has an Engagement at the Practice named by
-- the current session variable -- the same shape as staff_practice_visibility
-- in 00002_practice_staff_tenancy.sql.
--
-- This is a SELECT-only policy, not the usual single unqualified policy,
-- because of a chicken-and-egg problem: creating a Client happens in the
-- same transaction as creating their first Engagement, and at the moment
-- the Client row is inserted no Engagement referencing it exists yet, so
-- an EXISTS-based check would reject the insert. clients_insert below
-- covers that case separately.
CREATE POLICY clients_select ON clients
    FOR SELECT
    USING (
        EXISTS (
            SELECT 1 FROM engagements e
            WHERE e.client_id = clients.id
              AND e.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        )
    );

-- Any caller with a Practice context set (i.e. one who has already passed
-- staffauth.Middleware's membership check) may create a new Client row.
-- The row only becomes visible to them afterwards once an Engagement
-- links it to that Practice, via clients_select above.
CREATE POLICY clients_insert ON clients
    FOR INSERT
    WITH CHECK (NULLIF(current_setting('app.current_practice_id', true), '') IS NOT NULL);

-- engagements carries practice_id directly, so its policy is a plain
-- column comparison, same shape as practice_memberships_practice_visibility
-- in 00002_practice_staff_tenancy.sql. No chicken-and-egg problem here:
-- the practice_id on a new Engagement row is already known (and equal to
-- the session variable) at insert time.
CREATE POLICY engagements_practice_visibility ON engagements
    USING (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid);

-- +goose Down
DROP TABLE engagements;
DROP TABLE clients;
DROP TYPE engagement_status;
