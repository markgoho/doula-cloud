-- +goose NO TRANSACTION
-- +goose Up
-- #945: reversing a manually recorded Payment (#271) -- one logged
-- against the wrong Invoice, or a check that later bounces. 'reversal' is
-- the additive payment_kind value 00095's own comment set aside for this
-- (line 47), so this is the only migration of existing rows: none. Every
-- 'stripe' and 'manual' row is untouched.
--
-- Scope, decided against a Sandbox finding rather than the ticket's own
-- premise: Stripe's detach_payment call only detaches a
-- PaymentIntent-backed payment, and paid_out_of_band (#271) creates a
-- PaymentRecord-backed one instead -- verified against every preview API
-- version that exists, not just the docs (see #945's issue comment).
-- There is no working Stripe-side undo today, so PostReversePaymentHandler
-- refuses a Stripe-backed Invoice outright, the same door void and
-- write-off already use (MsgStripeInvoiceCannotBeVoidedOrWrittenOff). A
-- by-hand Invoice's Payment is the only one this schema, or that handler,
-- ever reaches in practice.
--
-- ALTER TYPE ... ADD VALUE cannot be used in the same transaction as a
-- later statement that references the new value (here, the CHECK
-- constraints below) -- hence NO TRANSACTION, 00040's own precedent.
ALTER TYPE payment_kind ADD VALUE 'reversal';

-- reversed_payment_id is the pointer #945's own body calls for: not to
-- the Invoice (already reachable via invoice_id) but to the specific
-- Payment being undone. Nullable -- only a 'reversal' row ever carries
-- one. The FK alone cannot stop a reversal from targeting another
-- reversal (Postgres CHECK cannot see another row's kind); that half of
-- "a reversal cannot itself be reversed" is enforced by
-- PostReversePaymentHandler locking and reading the target row's own kind
-- before inserting, the same row-locking shape resolveInvoiceForPractice
-- already uses. The partial unique index below enforces the other half a
-- CHECK can reach: at most one reversal per original Payment.
ALTER TABLE payments ADD COLUMN reversed_payment_id uuid REFERENCES payments (id);
CREATE UNIQUE INDEX payments_reversed_payment_id_unique ON payments (reversed_payment_id)
    WHERE reversed_payment_id IS NOT NULL;

-- reason is a reversal's own free-text field, distinct from a manually
-- recorded Payment's note (00095): recording a Payment's note is optional
-- scratch space, but undoing one always needs a stated why, per #945's
-- own Activity-log acceptance criterion. A second personal-data surface,
-- so it gets the same in-place-redaction treatment note already has
-- (ADR-0027) -- see client/erase.go and export/entities.go.
ALTER TABLE payments ADD COLUMN reason text;
GRANT UPDATE (reason) ON payments TO app_runtime;

-- payments_method_matches_kind (00095) only knew two kinds. A reversal
-- carries no method -- it is not a way money arrived, it is the record
-- that it left again -- so it joins the 'stripe' branch's shape rather
-- than getting a new one.
ALTER TABLE payments DROP CONSTRAINT payments_method_matches_kind;
ALTER TABLE payments ADD CONSTRAINT payments_method_matches_kind CHECK (
    (kind = 'manual' AND method IS NOT NULL)
    OR (kind = 'stripe' AND method IS NULL)
    OR (kind = 'reversal' AND method IS NULL)
);

-- reversed_payment_id is structural, never personal data, so it stays
-- NOT NULL on a 'reversal' row for its whole life -- nothing ever nulls
-- it back out.
ALTER TABLE payments ADD CONSTRAINT payments_reversed_payment_id_matches_kind CHECK (
    (kind = 'reversal' AND reversed_payment_id IS NOT NULL) OR (kind <> 'reversal' AND reversed_payment_id IS NULL)
);

-- reason is the opposite: personal data, so its CHECK only bounds which
-- kind may ever carry a value (a 'manual' or 'stripe' row never does),
-- not whether a 'reversal' row still has one -- erasure (#945's own
-- redactPaymentReversalReasons) nulls it out later on a still-'reversal'
-- row, the same shape 00095's note/method CHECK already has to tolerate
-- for erasure's redactPaymentNotes. PostReversePaymentHandler is what
-- actually requires a reason at creation; the database only stops it
-- appearing anywhere it shouldn't.
ALTER TABLE payments ADD CONSTRAINT payments_reason_matches_kind CHECK (
    kind = 'reversal' OR reason IS NULL
);

-- amount_cents carried no sign constraint before this migration (#945's
-- own body flags it). A 'reversal' row must net its target to zero when
-- summed, so it is always negative; every other kind is always positive
-- -- formalizing the invariant "is this Invoice still covered" already
-- relies on staying a single SUM with no second code path.
ALTER TABLE payments ADD CONSTRAINT payments_amount_sign_matches_kind CHECK (
    (kind = 'reversal' AND amount_cents < 0) OR (kind <> 'reversal' AND amount_cents > 0)
);

-- +goose Down
ALTER TABLE payments DROP CONSTRAINT payments_amount_sign_matches_kind;
ALTER TABLE payments DROP CONSTRAINT payments_reversed_payment_id_matches_kind;
ALTER TABLE payments DROP CONSTRAINT payments_reason_matches_kind;
ALTER TABLE payments DROP CONSTRAINT payments_method_matches_kind;
ALTER TABLE payments ADD CONSTRAINT payments_method_matches_kind CHECK (
    (kind = 'manual' AND method IS NOT NULL) OR (kind = 'stripe' AND method IS NULL)
);
REVOKE UPDATE (reason) ON payments FROM app_runtime;
ALTER TABLE payments DROP COLUMN reason;
DROP INDEX payments_reversed_payment_id_unique;
ALTER TABLE payments DROP COLUMN reversed_payment_id;

-- Postgres has no DROP VALUE. Rebuilding the enum means rewriting every
-- column that uses it, which is a larger and riskier operation than the
-- Up it reverses; a spare enum member is inert (00040's own reasoning).
SELECT 1;
