-- +goose Up
-- Second RLS policy on plan_instances (00012_plan_instances.sql), giving
-- the Client-portal population narrowly-scoped read access: SELECT-only,
-- and restricted to plan_type = 'birth_plan' -- Care Plan (see CONTEXT.md)
-- is staff-only and must stay unreachable from a Client-portal session no
-- matter what a future bug in the Go handler layer does. Mirrors
-- plan_instances_practice_visibility's EXISTS-against-engagements shape,
-- but keyed on app.current_client_id -- the Client-portal session variable
-- #53 establishes (see clientauth.Middleware) -- rather than
-- app.current_practice_id.
--
-- The EXISTS subquery below is itself subject to RLS on engagements:
-- it resolves through engagements_client_visibility (00006), the
-- Client-tier SELECT policy already scoping engagements to
-- app.current_client_id, the same way 00009's Staff-visible-to-Client
-- policies rely on other tables' policies resolving inside their own
-- EXISTS subqueries. Proven directly by this migration's RLS tests, not
-- just asserted here.
--
-- Postgres OR's multiple permissive policies on the same table together,
-- so this adds to plan_instances_practice_visibility rather than
-- replacing it: a Staff request (app.current_practice_id set) matches the
-- practice-tier policy, a Client-portal request (app.current_client_id
-- set) matches this one, and neither leaks into the other because only
-- one of the two session variables is ever set on a given transaction
-- (see clientauth.Middleware / staffauth.Middleware).
CREATE POLICY plan_instances_client_birth_plan_visibility ON plan_instances
    FOR SELECT
    USING (
        plan_type = 'birth_plan'
        AND EXISTS (
            SELECT 1 FROM engagements e
            WHERE e.id = plan_instances.engagement_id
              AND e.client_id = NULLIF(current_setting('app.current_client_id', true), '')::uuid
        )
    );

-- +goose Down
DROP POLICY plan_instances_client_birth_plan_visibility ON plan_instances;
