-- +goose Up
-- #449. #439 settled that a Practice joining the pilot receives three
-- Credits for each Staff member it has on the day it joins -- counted
-- once, never topped up, and a different thing from the signup bonus any
-- new Practice gets. One enum value cannot say "one-time, list pricing
-- beyond it" about one grant and something else about the other, so
-- 'founding_grant' arrives beside 'signup_bonus' rather than replacing it.
--
-- Keeping the two apart is also what makes the escheat question
-- answerable. #450's research reads APL 1315(1) as reaching gift
-- certificates "sold" and 1315(1-b) as reaching an amount "which amount
-- was received", and 2 NYCRR 115.1 -- the Comptroller's own regulation on
-- what must be reported -- copies both verbs rather than broadening them.
-- A Credit nobody paid for satisfies neither trigger, and `origin` is the
-- whole record that proves it: a query grouping outstanding balance by
-- origin separates the reportable purchases from the grants with no
-- inference. No further column serves that; losing the distinction would.
--
-- granted_by is the one new column, and it exists because a human decides
-- to issue a founding grant. "Who gave this Practice $840 of product, and
-- when?" had no answer: credit_ledger records a Practice, an origin, a
-- quantity and timestamps and nothing about who caused the row. That is
-- fine for signup_bonus, which the signup path writes for itself
-- (staffauth/signup.go), and for purchase, which #420 tied to a Stripe
-- payment object. It is not fine here.
--
-- It is a column rather than an `activity` row, and ADR-0022 is why
-- rather than in spite of. That table's actor_kind is staff | client |
-- system, and the person issuing a founding grant is none of the three:
-- they are the platform operator holding X-Internal-Secret, who is not
-- Staff of any Practice. Recording them as 'system' would make the
-- product the author of an act a person took -- the one thing the ADR
-- says an audit trail exists not to do -- and activity.practice_id plus
-- its INSERT-gating policy would need app.current_practice_id set by a
-- handler that sits outside staffauth.Middleware entirely. The ADR's own
-- precedent for that shape is 00043: where `activity` cannot hold the
-- actor, the record lives on the table that owns the fact. credit_ledger
-- is already an append-only ledger with a reader, so it holds this one.
--
-- The enum is rebuilt rather than extended with ALTER TYPE ... ADD VALUE,
-- the same reason 00052 gave: the CHECK constraints below name
-- 'founding_grant' in the same transaction, and Postgres refuses an
-- unsafe use of a new enum value there. 00052's constraints are dropped
-- first and restated after, because three of them are being widened
-- anyway and a constraint left standing across a column type change is
-- revalidated against a type it was not written for.
ALTER TABLE credit_ledger
    DROP CONSTRAINT credit_ledger_lot_or_draw,
    DROP CONSTRAINT credit_ledger_purchase_priced,
    DROP CONSTRAINT credit_ledger_grant_free,
    DROP CONSTRAINT credit_ledger_consumption_shape,
    DROP CONSTRAINT credit_ledger_refund_shape,
    DROP CONSTRAINT credit_ledger_refund_request_key;

ALTER TYPE credit_ledger_origin RENAME TO credit_ledger_origin_old;
CREATE TYPE credit_ledger_origin AS ENUM ('signup_bonus', 'founding_grant', 'purchase', 'consumption', 'refund');
ALTER TABLE credit_ledger
    ALTER COLUMN origin TYPE credit_ledger_origin
    USING origin::text::credit_ledger_origin;
DROP TYPE credit_ledger_origin_old;

ALTER TABLE credit_ledger ADD COLUMN granted_by text;

-- Restated from 00052, with 'founding_grant' added to the two branches
-- that describe a grant. A founding grant is a lot like any other:
-- quantity positive, nothing drawn against it, priced at a real $0.00 so
-- #420's lot selection reads it as free rather than as unknown.
ALTER TABLE credit_ledger
    ADD CONSTRAINT credit_ledger_lot_or_draw CHECK (
        (origin IN ('signup_bonus', 'founding_grant', 'purchase') AND quantity > 0
            AND drawn_lot_id IS NULL AND stripe_refund_id IS NULL)
        OR (origin IN ('consumption', 'refund') AND quantity < 0
            AND drawn_lot_id IS NOT NULL)
    ),
    ADD CONSTRAINT credit_ledger_purchase_priced CHECK (
        origin <> 'purchase'
        OR (unit_price_cents > 0 AND tax_cents >= 0 AND stripe_payment_intent_id IS NOT NULL)
    ),
    ADD CONSTRAINT credit_ledger_grant_free CHECK (
        origin NOT IN ('signup_bonus', 'founding_grant')
        OR (unit_price_cents = 0 AND tax_cents = 0 AND stripe_payment_intent_id IS NULL)
    ),
    ADD CONSTRAINT credit_ledger_consumption_shape CHECK (
        origin <> 'consumption'
        OR (quantity = -1 AND unit_price_cents = 0 AND tax_cents = 0
            AND consumed_engagement_id IS NOT NULL
            AND stripe_payment_intent_id IS NULL)
    ),
    ADD CONSTRAINT credit_ledger_refund_shape CHECK (
        origin <> 'refund'
        OR (unit_price_cents > 0 AND tax_cents <= 0
            AND stripe_refund_id IS NOT NULL
            AND consumed_engagement_id IS NULL)
    ),
    -- Restated from 00054, unchanged: a refund names the request that
    -- asked for it, and nothing else carries one. It is dropped and
    -- restated only because it names the enum column whose type is being
    -- rebuilt above.
    ADD CONSTRAINT credit_ledger_refund_request_key CHECK (
        (origin = 'refund') = (refund_request_key IS NOT NULL)
    ),
    -- A founding grant names who issued it; nothing else carries one.
    -- Non-empty, because an audit field holding '' answers nothing.
    ADD CONSTRAINT credit_ledger_granted_by CHECK (
        (origin = 'founding_grant') = (granted_by IS NOT NULL AND granted_by <> '')
    );

-- One founding grant per Practice, enforced where two requests arriving
-- at once still cannot both win. #439 sized it at three per Staff member
-- "on the day it joins, counted once and never topped up", so a second
-- grant is not a bigger grant, it is a mistake -- and the index is what
-- makes refusing it a fact rather than a race the handler usually wins.
CREATE UNIQUE INDEX credit_ledger_founding_grant
    ON credit_ledger (practice_id)
    WHERE origin = 'founding_grant';

-- +goose Down
DROP INDEX credit_ledger_founding_grant;
ALTER TABLE credit_ledger
    DROP CONSTRAINT credit_ledger_granted_by,
    DROP CONSTRAINT credit_ledger_refund_shape,
    DROP CONSTRAINT credit_ledger_consumption_shape,
    DROP CONSTRAINT credit_ledger_grant_free,
    DROP CONSTRAINT credit_ledger_purchase_priced,
    DROP CONSTRAINT credit_ledger_lot_or_draw,
    DROP CONSTRAINT credit_ledger_refund_request_key,
    DROP COLUMN granted_by;

DELETE FROM credit_ledger WHERE origin = 'founding_grant';
ALTER TYPE credit_ledger_origin RENAME TO credit_ledger_origin_old;
CREATE TYPE credit_ledger_origin AS ENUM ('signup_bonus', 'purchase', 'consumption', 'refund');
ALTER TABLE credit_ledger
    ALTER COLUMN origin TYPE credit_ledger_origin
    USING origin::text::credit_ledger_origin;
DROP TYPE credit_ledger_origin_old;

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
    ADD CONSTRAINT credit_ledger_refund_shape CHECK (
        origin <> 'refund'
        OR (unit_price_cents > 0 AND tax_cents <= 0
            AND stripe_refund_id IS NOT NULL
            AND consumed_engagement_id IS NULL)
    );
ALTER TABLE credit_ledger
    ADD CONSTRAINT credit_ledger_refund_request_key CHECK (
        (origin = 'refund') = (refund_request_key IS NOT NULL)
    );
