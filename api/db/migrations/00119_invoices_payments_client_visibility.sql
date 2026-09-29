-- +goose Up
-- Second RLS policy on invoices (00024_invoices.sql) and on payments
-- (00025_payments.sql), giving the Client-portal population narrowly-scoped
-- read access (#1019). Until now both tables carried exactly one policy,
-- keyed on app.current_practice_id, which clientauth.Middleware never sets:
-- a Client-portal session read zero rows from either, and no Go handler can
-- widen a policy that does not exist.
--
-- Both policies are SELECT-only. The Client population gains no INSERT or
-- UPDATE on either table: paying an Invoice is a Stripe act that flows back
-- by webhook, never a direct write from her session. Postgres OR's multiple
-- permissive policies on the same table together, so these add to the
-- practice-tier policies rather than replacing them -- the same two-tier
-- shape 00017_contracts_client_visibility.sql added for contracts.
--
-- invoices_client_visibility reaches the Client through contracts, then
-- engagements.client_id, matched against app.current_client_id. The status
-- filter lives in the policy, not the handler: a draft Invoice never reached
-- a Client (#981), and keeping it out of reach in Postgres is the same
-- fail-closed reasoning that keeps a Draft Contract out of the portal.
-- void and uncollectible stay visible -- both read "No longer owed" (#981),
-- and a Practice voiding a bereaved Client's Invoice (#982) only works if she
-- can see it change.
CREATE POLICY invoices_client_visibility ON invoices
    FOR SELECT
    USING (
        status <> 'draft'
        AND EXISTS (
            SELECT 1 FROM contracts c
            JOIN engagements e ON e.id = c.engagement_id
            WHERE c.id = invoices.contract_id
              AND e.client_id = NULLIF(current_setting('app.current_client_id', true), '')::uuid
        )
    );

-- payments has no client or contract column of its own, so its Client-tier
-- visibility is an EXISTS through invoice_id, the shape
-- payments_practice_visibility takes, reaching the same Client test and the
-- same draft exclusion one join further rather than trusting the nested
-- invoices policy to apply it implicitly.
CREATE POLICY payments_client_visibility ON payments
    FOR SELECT
    USING (
        EXISTS (
            SELECT 1 FROM invoices i
            JOIN contracts c ON c.id = i.contract_id
            JOIN engagements e ON e.id = c.engagement_id
            WHERE i.id = payments.invoice_id
              AND i.status <> 'draft'
              AND e.client_id = NULLIF(current_setting('app.current_client_id', true), '')::uuid
        )
    );

-- +goose Down
DROP POLICY payments_client_visibility ON payments;
DROP POLICY invoices_client_visibility ON invoices;
