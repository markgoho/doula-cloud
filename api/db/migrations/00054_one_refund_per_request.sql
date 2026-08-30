-- +goose Up
-- #420's refund endpoint is run by hand, and a hand-run request that times
-- out gets run again. Each call draws FIFO from the oldest eligible lot on
-- its own, so a blind retry would issue a second Stripe Refund and write a
-- second negative row: two Credits asked for, four given back.
--
-- The caller now names the request -- an Idempotency-Key header, the same
-- contract docs/api-design.md sets for every other repeatable write -- and
-- that name is kept on the row it produced. A retry under the same name
-- finds the row and is answered with the refund already made, without
-- touching Stripe. The index is what makes that safe under two requests
-- arriving at once, rather than merely likely.
ALTER TABLE credit_ledger ADD COLUMN refund_request_key text;

CREATE UNIQUE INDEX credit_ledger_refund_request
    ON credit_ledger (refund_request_key)
    WHERE refund_request_key IS NOT NULL;

-- The Stripe Refund itself is unique too, a second guard on the same
-- failure from the other side: even a refund reached some other way can
-- only ever be recorded once.
CREATE UNIQUE INDEX credit_ledger_stripe_refund
    ON credit_ledger (stripe_refund_id)
    WHERE stripe_refund_id IS NOT NULL;

-- A refund names the request that asked for it; nothing else carries one.
ALTER TABLE credit_ledger
    ADD CONSTRAINT credit_ledger_refund_request_key CHECK (
        (origin = 'refund') = (refund_request_key IS NOT NULL)
    );

-- +goose Down
ALTER TABLE credit_ledger DROP CONSTRAINT credit_ledger_refund_request_key;
DROP INDEX credit_ledger_stripe_refund;
DROP INDEX credit_ledger_refund_request;
ALTER TABLE credit_ledger DROP COLUMN refund_request_key;
