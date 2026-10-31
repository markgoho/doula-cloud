-- +goose Up
-- #2225: a payment that is on its way. A bank debit takes days to clear,
-- and Stripe keeps the Invoice 'open' while its PaymentIntent is
-- 'processing', so this is a fact beside invoice_status, never a sixth
-- value of it: our status mirrors Stripe's (#82, #2224).
--
-- Set by the Connect webhook's payment_intent.processing, from
-- clock.Now(ctx), while the Invoice is open. Cleared by invoice.paid, by
-- invoice.payment_failed (the attempt that was on its way did not go
-- through, and the Invoice stays open for another) and by
-- invoice.marked_uncollectible. A reader counts it only while the Invoice
-- is open, so a value left behind by any other way out of 'open' never
-- reads as a payment on its way.
--
-- Nullable with no DEFAULT: NULL is "nothing on its way", which is every
-- existing row.
ALTER TABLE invoices ADD COLUMN payment_processing_since timestamptz;

-- +goose Down
ALTER TABLE invoices DROP COLUMN payment_processing_since;
