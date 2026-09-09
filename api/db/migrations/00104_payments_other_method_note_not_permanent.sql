-- +goose Up
-- #1034: erasing a Client with a method = 'other' manually recorded
-- Payment (#271) against her currently fails outright.
-- client.redactPaymentNotes (#271, ADR-0027) nulls a manual Payment's
-- note during erasure with no method filter, and 00095's own
-- payments_other_method_requires_note re-evaluates on that UPDATE too --
-- it requires note to stay non-null forever once method = 'other', which
-- a redaction sweep can never satisfy.
--
-- The same mistake #945 made and corrected for reason/kind = 'reversal'
-- (00103's payments_reason_matches_kind): a CHECK cannot distinguish
-- "never had a note" (should stay refused) from "had one, now erased"
-- (must be allowed). Unlike reason, there is no forever-true bound left
-- to express in its place -- a 'check'/'bank_transfer'/'cash' row may
-- also carry an optional note, so nothing here is ever exclusively
-- true or false by method the way kind = 'reversal' bounds reason. The
-- rule that method = 'other' requires a note at creation stays enforced
-- in PostManualPaymentHandler alone (api/internal/payments/manual_payment.go),
-- the same trust boundary every other request-shape rule in that handler
-- already relies on (paidOn's date form, paidOn not in the future, method
-- being one of the four known values).
ALTER TABLE payments DROP CONSTRAINT payments_other_method_requires_note;

-- +goose Down
ALTER TABLE payments ADD CONSTRAINT payments_other_method_requires_note CHECK (
    method IS DISTINCT FROM 'other' OR note IS NOT NULL
);
