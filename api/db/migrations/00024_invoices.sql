-- +goose Up
-- An "Invoice" is a bill issued against a Contract (see CONTEXT.md),
-- created via Stripe's Invoicing API on behalf of a Practice's connected
-- account (#79). status mirrors Stripe's own Invoice statuses exactly, so
-- the webhook handler #82 adds can write it straight through without a
-- translation layer. Unlike credit_ledger (append-only, SELECT+INSERT
-- only), an invoices row's status legitimately changes after creation --
-- draft -> open here, then open -> paid/uncollectible via #82's webhook
-- -- so app_runtime also gets UPDATE. currency is stored even though v1
-- only ever writes 'usd' (per #78's ticket body), rather than assumed,
-- so a later multi-currency ticket is an additive read, not a schema
-- change. stripe_invoice_id is UNIQUE so #82's webhook handler can
-- resolve an incoming Stripe event straight to a row.
CREATE TYPE invoice_status AS ENUM ('draft', 'open', 'paid', 'uncollectible', 'void');

CREATE TABLE invoices (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    practice_id uuid NOT NULL REFERENCES practices (id),
    contract_id uuid NOT NULL REFERENCES contracts (id),
    stripe_invoice_id text NOT NULL UNIQUE,
    status invoice_status NOT NULL DEFAULT 'draft',
    amount_cents bigint NOT NULL,
    currency text NOT NULL DEFAULT 'usd',
    created_at timestamptz NOT NULL DEFAULT now(),
    paid_at timestamptz
);

GRANT SELECT, INSERT, UPDATE ON invoices TO app_runtime;

ALTER TABLE invoices ENABLE ROW LEVEL SECURITY;

-- Same shape as credit_ledger_practice_visibility in 00015_credit_ledger.sql:
-- a direct practice_id column matched against app.current_practice_id,
-- fail-closed when unset.
CREATE POLICY invoices_practice_visibility ON invoices
    USING (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid);

-- +goose Down
DROP TABLE invoices;
DROP TYPE invoice_status;
