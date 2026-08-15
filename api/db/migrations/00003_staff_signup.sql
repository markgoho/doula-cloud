-- +goose Up
-- Two things Practice signup and Staff login need that 00002 didn't
-- provide yet.
--
-- 1. Signup has to INSERT a new staff row before any
--    practice_memberships row exists for that person (the membership
--    needs the new staff row's id). The existing staff policy from
--    00002 is USING-only, so Postgres reuses it as the INSERT check too
--    -- which requires a membership that can't exist yet. This adds a
--    second, narrower INSERT policy: a caller may only ever insert a
--    staff row for their own verified identity.
-- 2. Right after login, before the caller has chosen a Practice, the
--    BFF needs to list which Practices they belong to (to decide
--    auto-redirect vs. a picker). The existing practice_memberships
--    policy only allows rows for the *chosen* Practice, which isn't
--    known yet -- so that lookup would always come back empty. This
--    mirrors the staff_self_visibility idea from 00002: memberships are
--    visible by the caller's own identity while no Practice has been
--    chosen for the request yet.
--
--    Naively writing that second policy as "EXISTS (SELECT 1 FROM staff
--    WHERE ...)" creates a cycle: staff's own policy already does an
--    EXISTS subquery into practice_memberships (00002), so Postgres ends
--    up expanding staff's policy -> practice_memberships' policy ->
--    staff's policy -> ... forever ("infinite recursion detected in
--    policy"). A SECURITY DEFINER function breaks the cycle: it runs
--    with the privileges of the migration role that owns the staff
--    table, which -- like any table owner -- bypasses that table's own
--    RLS, so looking up "my staff id" from inside a policy no longer
--    re-triggers staff's policies.

ALTER TABLE staff ADD COLUMN last_practice_id uuid REFERENCES practices (id);

CREATE POLICY staff_self_insert ON staff
    FOR INSERT
    WITH CHECK (identity_uid = NULLIF(current_setting('app.current_identity_uid', true), ''));

CREATE FUNCTION current_staff_id() RETURNS uuid
    LANGUAGE sql SECURITY DEFINER STABLE
    SET search_path = public, pg_temp
    AS $$
        SELECT id FROM staff WHERE identity_uid = NULLIF(current_setting('app.current_identity_uid', true), '')
    $$;

CREATE POLICY practice_memberships_self_visibility ON practice_memberships
    FOR SELECT
    USING (
        NULLIF(current_setting('app.current_practice_id', true), '') IS NULL
        AND staff_id = current_staff_id()
    );

-- +goose Down
DROP POLICY practice_memberships_self_visibility ON practice_memberships;
DROP FUNCTION current_staff_id();
DROP POLICY staff_self_insert ON staff;
ALTER TABLE staff DROP COLUMN last_practice_id;
