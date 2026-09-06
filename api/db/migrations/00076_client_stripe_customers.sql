-- +goose Up
-- One Stripe Customer per (Client, connected account), rather than the
-- fresh Customer every Invoice used to raise (#780). A Client billed six
-- times by the same Practice had six Customers in that Practice's Stripe
-- account; CONTEXT.md's Erasure entry always described one. This table is
-- the mapping the invoice path resolves before it creates anything, so a
-- second Invoice bills the Customer the first one made.
--
-- It is also what lets a simulation run pre-create a Client's Customer
-- against a Stripe test clock from outside the product: the harness
-- writes the row, and the product then finds a Customer and never creates
-- one. That is why no test-only parameter exists on any api/ code path.
--
-- practice_id is carried (rather than reached through engagements) for
-- the same reason invoices carries it: it is what row-level security
-- matches on. stripe_account_id is the connected account the Customer
-- lives in, kept explicitly rather than read off practices, because that
-- is the half of the key that makes the Customer resolvable at all -- a
-- Practice that re-connects under a new account gets a new row, not a
-- Customer id that no longer exists.
--
-- created_by_staff_id is the audit trail CLAUDE.md asks for: who caused
-- this Customer to exist. It is null exactly when no Staff did -- a
-- simulation harness allocating the Customer onto a test clock before any
-- Invoice is raised.
CREATE TABLE client_stripe_customers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    practice_id uuid NOT NULL REFERENCES practices (id),
    client_id uuid NOT NULL REFERENCES clients (id),
    stripe_account_id text NOT NULL,
    stripe_customer_id text NOT NULL,
    created_by_staff_id uuid REFERENCES staff (id),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (client_id, stripe_account_id)
);

CREATE INDEX client_stripe_customers_client ON client_stripe_customers (client_id);

-- SELECT and INSERT only: a mapping is written once and read after that.
-- Repointing a Client at a different Customer is not an operation this
-- product has, and erasure deletes the Customer at Stripe rather than the
-- row that records it ever existed.
GRANT SELECT, INSERT ON client_stripe_customers TO app_runtime;

ALTER TABLE client_stripe_customers ENABLE ROW LEVEL SECURITY;

-- Same shape as invoices_practice_visibility (00024_invoices.sql): a
-- direct practice_id column matched against app.current_practice_id,
-- fail-closed when unset.
CREATE POLICY client_stripe_customers_practice_visibility ON client_stripe_customers
    USING (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid);

-- +goose Down
DROP TABLE client_stripe_customers;
