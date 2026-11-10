-- +goose Up
-- A Payment's standing made explicit. payments is append-only and has no
-- status column (00025): a 'manual' or 'stripe' row stands Live, Partly
-- refunded, Fully refunded or Reversed according to the rows that point
-- at it, which api/internal/paymentstanding derives once (Lateral) for
-- every reader and guard, and payments' PaymentActs declare who may
-- write which pointing row. 00103 and 00115 left the cross-row rules to
-- the handlers alone, because "a Postgres CHECK cannot see another row's
-- kind". That is true of a CHECK, not of a trigger: this migration is the
-- schema's half, refusing every row no declared act could write.
--
-- A trigger, not a CHECK or an index, for the reason 00168 gave for its
-- one: each rule here is a fact about another row (the target's kind,
-- its invoice, what already points at it, what else stands on the
-- Invoice), which only a statement that reads that row can see. It
-- writes nothing and moves nothing -- it only refuses -- so no transition
-- happens in the database that Go does not declare.
--
-- It runs as the inserting role (SECURITY INVOKER, the default), so it
-- reads payments under the same RLS the insert is held to: a row naming
-- another Practice's Payment finds no target and is refused. It locks the
-- target row, so two rows pointing at one Payment serialize here even
-- when a caller forgot to lock it first -- the handlers' own lock
-- (lockPaymentFor) is taken first, so for them this lock is already held.
-- Each refusal names the rule as its CONSTRAINT, so a test and a log line
-- can tell them apart.
--
--   payments_one_standing_payment_per_invoice  A Payment is recorded only
--     on an Invoice with no Payment standing on it. An Invoice is paid
--     exactly while one Payment stands on it; every writer records one
--     beside the Invoice's own move to paid, and this keeps a second
--     from ever being written beside the first.
--   payments_target_is_a_payment_on_its_invoice  A reversal or Refund
--     points at a Payment ('manual' or 'stripe', never another reversal
--     or Refund) on the same Invoice.
--   payments_target_stands  Nothing points at a Reversed Payment: it was
--     never really paid, so it can be neither reversed again nor
--     refunded (#945, #1009).
--   payments_reversal_of_a_manual_payment  Only a by-hand Payment is
--     reversed; Stripe has no undo for a card Payment (#945).
--   payments_reversal_of_an_unrefunded_payment  A Payment money has gone
--     back against is not reversed (MsgRefundedPaymentCannotBeReversed).
--   payments_refunds_within_payment  The Refunds against a Payment never
--     sum to more than it.
--
-- The rows already in trunk's database are not re-read: the trigger
-- holds each new row, as 00167's NOT VALID CHECKs do.
--
-- Who wrote each row, and when, is its activity row (payment_recorded,
-- payment_reversed, payment_refunded, invoice_paid), and payments gains
-- no actor column: a Payment Stripe collected has no Staff member behind
-- it (attachment_fee_payments.recorded_by can be NOT NULL because a Fee
-- Payment always has one), and a second copy of the actor would be one
-- more thing to keep in step with the activity row.
-- +goose StatementBegin
CREATE FUNCTION payments_standing_guard() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path = public
AS $$
DECLARE
    target payments%ROWTYPE;
    refunded bigint;
BEGIN
    IF NEW.kind IN ('manual', 'stripe') THEN
        IF EXISTS (
            SELECT 1 FROM payments p
             WHERE p.invoice_id = NEW.invoice_id AND p.kind IN ('manual', 'stripe')
               AND NOT EXISTS (SELECT 1 FROM payments pt WHERE pt.target_payment_id = p.id AND pt.kind = 'reversal')
        ) THEN
            RAISE EXCEPTION 'a Payment already stands on invoice %', NEW.invoice_id
                USING ERRCODE = 'check_violation', CONSTRAINT = 'payments_one_standing_payment_per_invoice';
        END IF;
        RETURN NEW;
    END IF;

    SELECT * INTO target FROM payments WHERE id = NEW.target_payment_id FOR UPDATE;
    IF NOT FOUND OR target.kind NOT IN ('manual', 'stripe') OR target.invoice_id <> NEW.invoice_id THEN
        RAISE EXCEPTION 'a % must point at a Payment on its own invoice', NEW.kind
            USING ERRCODE = 'check_violation', CONSTRAINT = 'payments_target_is_a_payment_on_its_invoice';
    END IF;
    IF EXISTS (SELECT 1 FROM payments pt WHERE pt.target_payment_id = target.id AND pt.kind = 'reversal') THEN
        RAISE EXCEPTION 'payment % was reversed', target.id
            USING ERRCODE = 'check_violation', CONSTRAINT = 'payments_target_stands';
    END IF;
    SELECT COALESCE(-SUM(pt.amount_cents), 0) INTO refunded
      FROM payments pt WHERE pt.target_payment_id = target.id AND pt.kind = 'refund';

    IF NEW.kind = 'reversal' THEN
        IF target.kind <> 'manual' THEN
            RAISE EXCEPTION 'payment % is a % payment, and only a manual one is reversed', target.id, target.kind
                USING ERRCODE = 'check_violation', CONSTRAINT = 'payments_reversal_of_a_manual_payment';
        END IF;
        IF refunded > 0 THEN
            RAISE EXCEPTION 'payment % has money returned against it', target.id
                USING ERRCODE = 'check_violation', CONSTRAINT = 'payments_reversal_of_an_unrefunded_payment';
        END IF;
    ELSIF refunded - NEW.amount_cents > target.amount_cents THEN
        RAISE EXCEPTION 'refunds against payment % would exceed it', target.id
            USING ERRCODE = 'check_violation', CONSTRAINT = 'payments_refunds_within_payment';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER payments_standing_guard
    BEFORE INSERT ON payments
    FOR EACH ROW EXECUTE FUNCTION payments_standing_guard();

-- The Refunds against one Payment, the lookup paymentstanding.Lateral and
-- the trigger above both make for every Payment they read. The reversal
-- lookup already rides payments_one_reversal_per_payment (00115), which
-- is the same shape scoped to kind = 'reversal'.
CREATE INDEX payments_refunds_by_target ON payments (target_payment_id)
    WHERE kind = 'refund';

-- refund.created reaches its Payment by the PaymentIntent the Refund
-- names, stored as a card Payment's stripe_payment_reference
-- (refundedStripePayment). 00115's partial index covers only
-- kind = 'refund', so this lookup was a scan of the Practice's rows
-- under RLS, once per Refund (#1410's second webhook gap).
CREATE INDEX payments_stripe_reference ON payments (stripe_payment_reference)
    WHERE kind = 'stripe';

-- A Fee Payment (00136) is the same shape without Refunds: a 'recorded'
-- row stands until one 'reversal' row targets it
-- (paymentstanding.FeeStandsExpr). The same two rules a reversal is held
-- to above, for the same reason:
--
--   attachment_fee_payments_one_standing_per_attachment  A Fee Payment is
--     recorded only on an Attachment with none standing on it: the record
--     is the whole accepted fee, so a second would count it twice
--     (MsgFeeAlreadyPaid).
--   attachment_fee_payments_reversal_of_a_recorded_payment  A reversal
--     targets a recorded Fee Payment on its own Attachment, never another
--     reversal (attachment_fee_payments_one_reversal, 00136, already
--     allows one per target).
-- +goose StatementBegin
CREATE FUNCTION attachment_fee_payments_standing_guard() RETURNS trigger
    LANGUAGE plpgsql
    SET search_path = public
AS $$
DECLARE
    target attachment_fee_payments%ROWTYPE;
BEGIN
    IF NEW.kind = 'recorded' THEN
        IF EXISTS (
            SELECT 1 FROM attachment_fee_payments f
             WHERE f.attachment_id = NEW.attachment_id AND f.kind = 'recorded'
               AND NOT EXISTS (SELECT 1 FROM attachment_fee_payments rv WHERE rv.target_id = f.id AND rv.kind = 'reversal')
        ) THEN
            RAISE EXCEPTION 'a Fee Payment already stands on attachment %', NEW.attachment_id
                USING ERRCODE = 'check_violation', CONSTRAINT = 'attachment_fee_payments_one_standing_per_attachment';
        END IF;
        RETURN NEW;
    END IF;

    SELECT * INTO target FROM attachment_fee_payments WHERE id = NEW.target_id FOR UPDATE;
    IF NOT FOUND OR target.kind <> 'recorded' OR target.attachment_id <> NEW.attachment_id THEN
        RAISE EXCEPTION 'a reversal must target a recorded Fee Payment on its own attachment'
            USING ERRCODE = 'check_violation', CONSTRAINT = 'attachment_fee_payments_reversal_of_a_recorded_payment';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER attachment_fee_payments_standing_guard
    BEFORE INSERT ON attachment_fee_payments
    FOR EACH ROW EXECUTE FUNCTION attachment_fee_payments_standing_guard();

-- +goose Down
DROP TRIGGER attachment_fee_payments_standing_guard ON attachment_fee_payments;
DROP FUNCTION attachment_fee_payments_standing_guard();
DROP INDEX payments_stripe_reference;
DROP INDEX payments_refunds_by_target;
DROP TRIGGER payments_standing_guard ON payments;
DROP FUNCTION payments_standing_guard();
