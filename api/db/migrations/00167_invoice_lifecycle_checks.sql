-- +goose Up
-- The Invoice lifecycle made explicit (api/internal/payments/lifecycle.go
-- declares every move between invoice_status's five stages). These CHECKs
-- are the schema's half: what each stage carries, so a row no declared
-- move could leave is refused by the database rather than stored.
--
-- A paid Invoice has the instant it was paid, and nothing else does: every
-- move into 'paid' writes paid_at, and the one move out of it (a reversal)
-- clears it. A refunded Invoice stays 'paid' with its paid_at (00115).
ALTER TABLE invoices ADD CONSTRAINT invoices_paid_at_only_while_paid
    CHECK ((status = 'paid') = (paid_at IS NOT NULL));

-- A payment on its way (00135) is a fact about an open Invoice only. Every
-- move out of 'open' that Stripe reports clears it, and nothing sets it on
-- an Invoice that is not open.
ALTER TABLE invoices ADD CONSTRAINT invoices_processing_only_while_open
    CHECK (payment_processing_since IS NULL OR status = 'open');

-- Only Stripe reports a payment on its way, so a by-hand Invoice
-- (stripe_invoice_id IS NULL, 00095's rail flag) never carries one.
ALTER TABLE invoices ADD CONSTRAINT invoices_processing_only_on_stripe
    CHECK (payment_processing_since IS NULL OR stripe_invoice_id IS NOT NULL);

-- A by-hand Invoice is born open; only a Stripe-backed one is ever a draft
-- (Stripe creates it before it finalizes it).
ALTER TABLE invoices ADD CONSTRAINT invoices_draft_only_on_stripe
    CHECK (status <> 'draft' OR stripe_invoice_id IS NOT NULL);

-- Who voided or wrote off an Invoice, and when, is its activity row
-- (invoice_voided, invoice_written_off, invoice_not_sent: a Staff actor
-- for a move made here, DoulaCloud with a stripe_dashboard origin for one
-- reported by Stripe), the same record every other move writes. The row
-- gains no column for it: a second copy of the instant would be one more
-- thing a later move has to keep in step.

-- +goose Down
ALTER TABLE invoices DROP CONSTRAINT invoices_draft_only_on_stripe;
ALTER TABLE invoices DROP CONSTRAINT invoices_processing_only_on_stripe;
ALTER TABLE invoices DROP CONSTRAINT invoices_processing_only_while_open;
ALTER TABLE invoices DROP CONSTRAINT invoices_paid_at_only_while_paid;
