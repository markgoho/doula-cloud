-- +goose Up
-- A "Payment" is money received against an Invoice (see CONTEXT.md) --
-- distinct from the Invoice itself, per the domain's Contract/Invoice/
-- Payment split. Written only by #82's invoice.paid webhook handler,
-- never by any client-initiated call, so "was this actually paid" stays
-- server-authoritative -- app_runtime gets SELECT+INSERT only, no
-- UPDATE/DELETE, the same append-only shape as credit_ledger
-- (00015_credit_ledger.sql). No refund, dispute, or partial-payment state
-- is modeled (out of scope for all of #78), so there is no status column
-- and no FK back from invoices -- a Payment simply records what Stripe
-- reported paid. stripe_payment_reference is text, not UNIQUE: Stripe's
-- payment-intent id is stored for reference/reconciliation, but
-- uniqueness of the *event* (and therefore of this row) is already
-- enforced by stripe_webhook_events, reused unchanged from #80 rather
-- than adding a second idempotency mechanism here.
CREATE TABLE payments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id uuid NOT NULL REFERENCES invoices (id),
    stripe_payment_reference text NOT NULL,
    amount_cents bigint NOT NULL,
    paid_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

GRANT SELECT, INSERT ON payments TO app_runtime;

ALTER TABLE payments ENABLE ROW LEVEL SECURITY;

-- payments has no practice_id column of its own, so its practice-tier
-- visibility is an EXISTS subquery through invoice_id -> invoices, the
-- same shape as contracts_practice_visibility in 00016_contracts.sql
-- (which does the same thing one hop further, through engagements).
-- invoices' own policy (invoices_practice_visibility) is itself a direct
-- practice_id column compare -- this reaches the same practice_id column,
-- just one join away instead of stored redundantly on this table.
CREATE POLICY payments_practice_visibility ON payments
    USING (
        EXISTS (
            SELECT 1 FROM invoices i
            WHERE i.id = payments.invoice_id
              AND i.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        )
    );

-- +goose Down
DROP TABLE payments;
