-- +goose Up
-- A "Practice" is a tenant business (a doula practice). "Staff" are the
-- people who work there. "practice_memberships" links a Staff person to a
-- Practice and records what role(s) they hold there (owner, office
-- manager, doula) -- the same person can have different roles at
-- different Practices. A membership can start with zero roles (see the
-- Staff-invitation ticket) until an Owner assigns some, so there is no
-- database rule requiring at least one role per row.

CREATE TABLE practices (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- staff is a global table (one row per person), not scoped to a single
-- Practice -- a person can work at more than one Practice via separate
-- practice_memberships rows.
CREATE TABLE staff (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    identity_uid text NOT NULL UNIQUE,
    name text NOT NULL,
    email text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TYPE practice_role AS ENUM ('owner', 'office_manager', 'doula');

CREATE TABLE practice_memberships (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    practice_id uuid NOT NULL REFERENCES practices (id),
    staff_id uuid NOT NULL REFERENCES staff (id),
    roles practice_role[] NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (practice_id, staff_id)
);

-- ---------------------------------------------------------------------
-- Row-Level Security (RLS)
--
-- RLS is a second, database-enforced check on top of the application's
-- own filtering. Even if a bug in the Go code forgets to filter by
-- Practice, Postgres itself still refuses to hand back rows that don't
-- belong to the caller's current Practice. The BFF tells Postgres which
-- Practice the current request is acting as by setting a per-request,
-- per-transaction session variable (app.current_practice_id) via
-- set_config(...). If that variable is never set, the policies below have
-- nothing to match against, so they return zero rows -- "fail closed",
-- not "fail open".
--
-- These policies only apply to the low-privilege app_runtime role that
-- the running application connects as. Table owners and superusers (e.g.
-- the role that runs migrations) always bypass RLS -- that is Postgres's
-- own design, not something this migration can turn off. Tests must use
-- an app_runtime-derived connection to actually observe RLS in effect.
-- ---------------------------------------------------------------------

-- Roles are cluster-wide in Postgres, not per-database, so a bare CREATE
-- ROLE would fail if this migration is ever applied a second time against
-- another database in the same Cloud SQL instance. Guard it so re-applying
-- is a no-op instead of a blocked deploy.
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'app_runtime') THEN
        CREATE ROLE app_runtime NOLOGIN;
    END IF;
END
$$;
-- +goose StatementEnd

GRANT SELECT, INSERT, UPDATE, DELETE ON practices, staff, practice_memberships TO app_runtime;

ALTER TABLE staff ENABLE ROW LEVEL SECURITY;
ALTER TABLE practice_memberships ENABLE ROW LEVEL SECURITY;

-- staff has no practice_id column (it is a global entity), so its
-- practice-tier visibility is an EXISTS subquery against
-- practice_memberships: a staff row is visible if that person holds a
-- membership at the Practice named by the current session variable.
CREATE POLICY staff_practice_visibility ON staff
    USING (
        EXISTS (
            SELECT 1 FROM practice_memberships pm
            WHERE pm.staff_id = staff.id
              AND pm.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        )
    );

-- Right after login, before the BFF has worked out which Practice (if
-- any) the caller is acting as, the caller still needs to look themselves
-- up by their verified identity-provider uid. This second, separate
-- policy allows exactly that lookup. Postgres OR's multiple policies on
-- the same table together, so a staff row is visible if either policy
-- matches. It is deliberately restricted to the window before
-- app.current_practice_id is set: once a Practice context is chosen, only
-- staff_practice_visibility governs staff-table access for the rest of
-- the transaction, so this self-lookup can't quietly leak the caller's
-- own row into a different Practice's data.
CREATE POLICY staff_self_visibility ON staff
    FOR SELECT
    USING (
        NULLIF(current_setting('app.current_practice_id', true), '') IS NULL
        AND identity_uid = NULLIF(current_setting('app.current_identity_uid', true), '')
    );

-- practice_memberships already carries practice_id directly, so its
-- policy is a plain column comparison rather than an EXISTS subquery.
CREATE POLICY practice_memberships_practice_visibility ON practice_memberships
    USING (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid);

-- +goose Down
DROP TABLE practice_memberships;
DROP TYPE practice_role;
DROP TABLE staff;
DROP TABLE practices;
DROP ROLE IF EXISTS app_runtime;
