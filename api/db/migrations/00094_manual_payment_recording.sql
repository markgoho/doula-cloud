-- +goose Up
-- #271: a Payment that never came through Stripe (a check, a bank
-- transfer, cash), and the Invoice it is recorded against when that
-- Invoice never went to Stripe either -- a Practice that bills its own
-- Clients by hand had nothing to raise an Invoice against, let alone
-- record a Payment on, until this migration.
--
-- billing_mode is a Practice-level choice between the two rails, nullable
-- at birth (staffauth/signup.go writes no value) and asked inline the
-- first time any Staff raises an Invoice -- never a business fact assumed
-- from an absent Stripe Connect account, per the grilling session's
-- explicit rejection of that shortcut.
CREATE TYPE practice_billing_mode AS ENUM ('stripe', 'by_hand');
ALTER TABLE practices ADD COLUMN billing_mode practice_billing_mode;

-- next_invoice_sequence backs a by-hand Invoice's human-readable
-- reference (INV-0001, INV-0002, ...) -- a per-Practice counter claimed
-- with one atomic UPDATE ... RETURNING rather than a MAX(sequence)+1
-- read-then-write, so two concurrent by-hand Invoices can never claim the
-- same number. Starts at 1; a claim reads and increments in the same
-- statement.
ALTER TABLE practices ADD COLUMN next_invoice_sequence integer NOT NULL DEFAULT 1;

-- A Stripe-backed Invoice's stripe_invoice_id was NOT NULL because every
-- Invoice used to be one. A by-hand Invoice never calls Stripe at all, so
-- the column drops NOT NULL and NULL becomes the rail flag itself --
-- there is no second column naming which rail an Invoice is on. UNIQUE
-- is untouched: Postgres treats every NULL as distinct from every other
-- value under a UNIQUE constraint, so any number of by-hand Invoices can
-- coexist with it.
ALTER TABLE invoices ALTER COLUMN stripe_invoice_id DROP NOT NULL;

-- Every Invoice, on either rail, now carries a human-readable reference a
-- check "for invoice ___" can be matched against on paper: a by-hand
-- Invoice's own per-Practice sequence, or Stripe's own `number`, captured
-- once PostInvoiceHandler finalizes the Stripe Invoice (Stripe does not
-- assign `number` before finalization, and finalization already happens
-- inside that same request). NOT NULL with no default: nothing selects
-- into this table before the application populates it, per #78's
-- "no migration of live data" standing rule -- there is no production
-- instance provisioned yet (docs/testing.md).
ALTER TABLE invoices ADD COLUMN reference text NOT NULL;

-- payments.kind distinguishes a row the Connect webhook wrote from money
-- Stripe never touched (a manual Payment, whether against a by-hand
-- Invoice or paid_out_of_band against a Stripe one) -- an enum rather
-- than a boolean so #945's reversal kind is an additive value later, with
-- no migration of existing rows. method and note exist only on a manual
-- row: the CHECK below ties them to kind rather than trusting every
-- caller to leave them alone. stripe_payment_reference drops NOT NULL --
-- a manual Payment has none, on either rail, since paid_out_of_band
-- creates no PaymentIntent or charge to reference.
CREATE TYPE payment_kind AS ENUM ('stripe', 'manual');
CREATE TYPE payment_method AS ENUM ('check', 'bank_transfer', 'cash', 'other');

ALTER TABLE payments ALTER COLUMN stripe_payment_reference DROP NOT NULL;
ALTER TABLE payments ADD COLUMN kind payment_kind NOT NULL DEFAULT 'stripe';
ALTER TABLE payments ALTER COLUMN kind DROP DEFAULT;
ALTER TABLE payments ADD COLUMN method payment_method;
ALTER TABLE payments ADD COLUMN note text;

ALTER TABLE payments ADD CONSTRAINT payments_method_matches_kind CHECK (
    (kind = 'manual' AND method IS NOT NULL) OR (kind = 'stripe' AND method IS NULL)
);
ALTER TABLE payments ADD CONSTRAINT payments_other_method_requires_note CHECK (
    method IS DISTINCT FROM 'other' OR note IS NOT NULL
);

-- note is the one payments column erasure ever changes (ADR-0027:
-- client.redactPaymentNotes empties it, the same in-place-redaction rule
-- applied to a Contract's merge fields). A column-scoped grant, not a
-- table-wide one, is what lets that single UPDATE through without
-- reopening the table's own append-only rule (SELECT, INSERT only,
-- 00025_payments.sql) for every other column -- same shape as
-- 00079_portal_sign_in_address_change.sql's GRANT UPDATE (sign_in_address).
GRANT UPDATE (note) ON payments TO app_runtime;

-- +goose Down
REVOKE UPDATE (note) ON payments FROM app_runtime;
ALTER TABLE payments DROP CONSTRAINT payments_other_method_requires_note;
ALTER TABLE payments DROP CONSTRAINT payments_method_matches_kind;
ALTER TABLE payments DROP COLUMN note;
ALTER TABLE payments DROP COLUMN method;
ALTER TABLE payments DROP COLUMN kind;
ALTER TABLE payments ALTER COLUMN stripe_payment_reference SET NOT NULL;
DROP TYPE payment_method;
DROP TYPE payment_kind;

ALTER TABLE invoices DROP COLUMN reference;
ALTER TABLE invoices ALTER COLUMN stripe_invoice_id SET NOT NULL;

ALTER TABLE practices DROP COLUMN next_invoice_sequence;
ALTER TABLE practices DROP COLUMN billing_mode;
DROP TYPE practice_billing_mode;
