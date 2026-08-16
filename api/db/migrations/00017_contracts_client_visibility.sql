-- +goose Up
-- Second RLS policy on contracts (00016_contracts.sql), giving the
-- Client-portal population narrowly-scoped read access: SELECT-only, and
-- restricted to status IN ('sent', 'signed', 'voided') -- a Draft
-- Contract must stay unreachable from a Client-portal session no matter
-- what a future bug in the Go handler layer does, since #69's Send
-- transition is what's supposed to be the only gate on Client
-- visibility. Mirrors plan_instances_client_birth_plan_visibility's
-- EXISTS-against-engagements shape (00013_plan_instances_client_birth_plan_read.sql),
-- keyed on app.current_client_id -- the Client-portal session variable
-- #53 establishes (see clientauth.Middleware) -- rather than
-- app.current_practice_id.
--
-- Postgres OR's multiple permissive policies on the same table together,
-- so this adds to contracts_practice_visibility rather than replacing
-- it, the same two-tier shape 00013 added for plan_instances.
CREATE POLICY contracts_client_visibility ON contracts
    FOR SELECT
    USING (
        status IN ('sent', 'signed', 'voided')
        AND EXISTS (
            SELECT 1 FROM engagements e
            WHERE e.id = contracts.engagement_id
              AND e.client_id = NULLIF(current_setting('app.current_client_id', true), '')::uuid
        )
    );

-- +goose Down
DROP POLICY contracts_client_visibility ON contracts;
