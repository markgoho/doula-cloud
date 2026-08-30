-- +goose Up
-- #420. credit_ledger recorded practice_id, origin, a signed quantity and
-- timestamps, and nothing about what a Credit cost. docs/copy/support-page.md
-- now promises a refund "at the price paid for them and together with any
-- sales tax charged on them" within three years, and that number was not
-- computable from our own data: Credits were fungible, and the only
-- Stripe-side table, stripe_webhook_events, holds a bare event id with no
-- link back to the ledger row it created.
--
-- Five columns arrive together, because a refund needs all of them:
--
--   * unit_price_cents -- what one Credit in this lot cost. NOT NULL
--     DEFAULT 0, so a granted Credit reads as a real $0.00 rather than as
--     an unknown price (#449's founding_grant lot depends on that).
--   * tax_cents -- the sales tax actually charged on the lot, signed the
--     same way quantity is: positive on money in, negative on money out.
--     So quantity * unit_price_cents + tax_cents is the cash a row moved,
--     for every row in the table.
--   * stripe_payment_intent_id -- the payment the money arrived on. It is
--     also the object a refund must be issued against, so Stripe Tax
--     reverses the tax it reported to New York.
--   * stripe_refund_id -- the Stripe Refund a refund row was issued as.
--   * drawn_lot_id -- which grant or purchase row a consumption or refund
--     row drew against, so lots drain FIFO and a lot's remaining quantity
--     is a query rather than a guess.
--
-- The enum is rebuilt rather than extended with ALTER TYPE ... ADD VALUE:
-- the CHECK constraints below name 'refund' in the same transaction, and
-- Postgres refuses an unsafe use of a new enum value there. #449 adds
-- 'founding_grant' next and adapts to whichever of the two lands first.
ALTER TYPE credit_ledger_origin RENAME TO credit_ledger_origin_old;
CREATE TYPE credit_ledger_origin AS ENUM ('signup_bonus', 'purchase', 'consumption', 'refund');
ALTER TABLE credit_ledger
    ALTER COLUMN origin TYPE credit_ledger_origin
    USING origin::text::credit_ledger_origin;
DROP TYPE credit_ledger_origin_old;

ALTER TABLE credit_ledger
    ADD COLUMN unit_price_cents integer NOT NULL DEFAULT 0,
    ADD COLUMN tax_cents integer NOT NULL DEFAULT 0,
    ADD COLUMN stripe_payment_intent_id text,
    ADD COLUMN stripe_refund_id text,
    ADD COLUMN drawn_lot_id uuid REFERENCES credit_ledger (id);

-- Pre-launch, so there is no production data (CLAUDE.md) -- this only has
-- to leave a developer's own database satisfying the constraints below.
-- Grants already read 0/0 from the defaults; a consumption row predates
-- lot selection entirely, so it is attached to its Practice's oldest lot
-- rather than to a reconstructed draw. A consumption row cannot exist
-- without a positive lot before it, because ConsumeCredit refuses a
-- balance of zero.
UPDATE credit_ledger c
SET drawn_lot_id = (
        SELECT l.id FROM credit_ledger l
        WHERE l.practice_id = c.practice_id AND l.quantity > 0
        ORDER BY l.created_at, l.id
        LIMIT 1
    )
WHERE c.origin = 'consumption';

-- What each origin means, stated once, in the one place that can refuse a
-- row that does not mean it. A grant or purchase is a lot: quantity
-- positive, nothing drawn against. A consumption or refund draws from
-- exactly one lot: quantity negative, drawn_lot_id set.
ALTER TABLE credit_ledger
    ADD CONSTRAINT credit_ledger_lot_or_draw CHECK (
        (origin IN ('signup_bonus', 'purchase') AND quantity > 0
            AND drawn_lot_id IS NULL AND stripe_refund_id IS NULL)
        OR (origin IN ('consumption', 'refund') AND quantity < 0
            AND drawn_lot_id IS NOT NULL)
    ),
    ADD CONSTRAINT credit_ledger_purchase_priced CHECK (
        origin <> 'purchase'
        OR (unit_price_cents > 0 AND tax_cents >= 0 AND stripe_payment_intent_id IS NOT NULL)
    ),
    ADD CONSTRAINT credit_ledger_grant_free CHECK (
        origin <> 'signup_bonus'
        OR (unit_price_cents = 0 AND tax_cents = 0 AND stripe_payment_intent_id IS NULL)
    ),
    ADD CONSTRAINT credit_ledger_consumption_shape CHECK (
        origin <> 'consumption'
        OR (quantity = -1 AND unit_price_cents = 0 AND tax_cents = 0
            AND consumed_engagement_id IS NOT NULL
            AND stripe_payment_intent_id IS NULL)
    ),
    -- A refund states what it returned: the lot's own unit price, the tax
    -- given back (negative, the mirror of what was charged), and the
    -- Stripe Refund it was issued as. It is never an engagement.
    ADD CONSTRAINT credit_ledger_refund_shape CHECK (
        origin <> 'refund'
        OR (unit_price_cents > 0 AND tax_cents <= 0
            AND stripe_refund_id IS NOT NULL
            AND consumed_engagement_id IS NULL)
    );

-- Lot selection reads a Practice's lots oldest first; every draw against a
-- lot is found by drawn_lot_id. Those are the only two shapes the FIFO
-- query has.
CREATE INDEX credit_ledger_lots ON credit_ledger (practice_id, created_at, id) WHERE quantity > 0;
CREATE INDEX credit_ledger_draws ON credit_ledger (drawn_lot_id);

-- +goose Down
DROP INDEX credit_ledger_draws;
DROP INDEX credit_ledger_lots;
ALTER TABLE credit_ledger
    DROP CONSTRAINT credit_ledger_refund_shape,
    DROP CONSTRAINT credit_ledger_consumption_shape,
    DROP CONSTRAINT credit_ledger_grant_free,
    DROP CONSTRAINT credit_ledger_purchase_priced,
    DROP CONSTRAINT credit_ledger_lot_or_draw,
    DROP COLUMN drawn_lot_id,
    DROP COLUMN stripe_refund_id,
    DROP COLUMN stripe_payment_intent_id,
    DROP COLUMN tax_cents,
    DROP COLUMN unit_price_cents;
ALTER TYPE credit_ledger_origin RENAME TO credit_ledger_origin_old;
CREATE TYPE credit_ledger_origin AS ENUM ('signup_bonus', 'purchase', 'consumption');
ALTER TABLE credit_ledger
    ALTER COLUMN origin TYPE credit_ledger_origin
    USING origin::text::credit_ledger_origin;
DROP TYPE credit_ledger_origin_old;
