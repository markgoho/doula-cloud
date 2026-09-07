-- +goose Up
-- 00085 added the 'forfeit' origin value but left credit_ledger_lot_or_draw
-- (00052, widened by 00055 for 'founding_grant') unable to admit a row
-- carrying it at all: the constraint's OR names every origin it accepts,
-- and 'forfeit' was in neither branch, so any such row failed
-- unconditionally regardless of shape. This is that widening, the same
-- restate-the-whole-constraint shape 00055 used for its own addition.
--
-- Forfeiture is neither a lot (it is not positive, and nothing is
-- allocated) nor a draw against one specific lot the way a consumption or
-- a refund is (00052's own distinction: "a consumption or refund draws
-- from exactly one lot"). A Practice's deletion forfeits its whole
-- remaining balance in one row, across however many lots it happens to
-- be sitting in, so it gets its own third branch: quantity negative,
-- drawn_lot_id left null because there is no single lot to name.
ALTER TABLE credit_ledger
    DROP CONSTRAINT credit_ledger_lot_or_draw,
    ADD CONSTRAINT credit_ledger_lot_or_draw CHECK (
        (origin IN ('signup_bonus', 'founding_grant', 'purchase') AND quantity > 0
            AND drawn_lot_id IS NULL AND stripe_refund_id IS NULL)
        OR (origin IN ('consumption', 'refund') AND quantity < 0
            AND drawn_lot_id IS NOT NULL)
        OR (origin = 'forfeit' AND quantity < 0
            AND drawn_lot_id IS NULL AND stripe_refund_id IS NULL)
    );

-- +goose Down
ALTER TABLE credit_ledger
    DROP CONSTRAINT credit_ledger_lot_or_draw,
    ADD CONSTRAINT credit_ledger_lot_or_draw CHECK (
        (origin IN ('signup_bonus', 'founding_grant', 'purchase') AND quantity > 0
            AND drawn_lot_id IS NULL AND stripe_refund_id IS NULL)
        OR (origin IN ('consumption', 'refund') AND quantity < 0
            AND drawn_lot_id IS NOT NULL)
    );
