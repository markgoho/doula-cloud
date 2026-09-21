-- +goose NO TRANSACTION
-- +goose Up
-- #1009: a Practice returning money a Client already paid -- a Refund.
-- The concrete case is a deposit taken and then the Engagement canceled
-- before the work started: the money genuinely arrived, and the Practice
-- now owes it back. Until this migration that state was stored
-- identically to a squared-up account (an Invoice at 'paid' with a
-- Payment against it), which is why #982 had to word the Client's
-- settled-state sentence around the gap rather than state it plainly.
--
-- A Refund is NOT #945's reversal, and the two must not be built twice.
-- A reversal says the money never arrived (a check bounced, a Payment was
-- logged against the wrong Invoice) and returns the Invoice to 'open', so
-- the Client owes it again. A Refund says the opposite: the money arrived,
-- the bill was settled, and the Practice sent it back. The Invoice stays
-- 'paid'.
--
-- That the Invoice stays 'paid' is not an accounting preference adopted
-- here, it is the only thing Stripe's own record does, verified in the
-- Sandbox before this was built (#1009's own issue comment). A credit note
-- against a paid Stripe Invoice -- whether it carries refund_amount (card)
-- or out_of_band_amount (a paid_out_of_band Invoice, #271) -- leaves
-- status at 'paid' and amount_paid untouched, and carries the money back
-- on post_payment_credit_notes_amount, a field beside the status rather
-- than a change to it. invoice_status mirrors Stripe's statuses by design
-- and Stripe has no refunded value; any other local choice would put
-- Doula Cloud's record out of step with Stripe's on the Stripe rail. So
-- "is this Client owed money back" is answered by the Refund rows and
-- never by the Invoice's status, and the Practice-wide outstanding figure
-- -- a sum over 'open' Invoices -- is untouched by a Refund.
--
-- ALTER TYPE ... ADD VALUE cannot be used in the same transaction as a
-- later statement referencing the new value (the CHECK constraints
-- below), hence NO TRANSACTION -- 00103's own precedent, which 00040 set.
ALTER TYPE payment_kind ADD VALUE 'refund';

-- 00103 named this column reversed_payment_id because a reversal was the
-- only thing that had ever pointed at another Payment. A Refund points at
-- the Payment it returns through the same column -- #1009's own brief is
-- explicit that there must not be a second column for the same
-- relationship -- but on a Refund row the old name asserts the one thing
-- this ticket exists to deny, that the target was reversed. Renamed
-- rather than duplicated, with its index and CHECK renamed alongside so
-- nothing in the schema still says "reversed" about a Payment that was
-- not. Cheap now, expensive in January (CLAUDE.md: pre-launch, no data).
ALTER TABLE payments RENAME COLUMN reversed_payment_id TO target_payment_id;
ALTER TABLE payments RENAME CONSTRAINT payments_reversed_payment_id_matches_kind TO payments_target_payment_id_matches_kind;

-- 00103's partial unique index was ON (reversed_payment_id) WHERE NOT
-- NULL, which enforced "at most one reversal per original Payment" only
-- because a reversal was the sole row kind that ever set the column. A
-- Refund sets it too, and a partial Refund is the same shape as a full
-- one (#1009: a Practice who keeps a canceled Engagement's first two
-- visits and returns the rest), so several Refund rows must be able to
-- name one Payment. Rebuilt scoped to kind = 'reversal', which is the
-- invariant 00103 actually meant.
DROP INDEX payments_reversed_payment_id_unique;
CREATE UNIQUE INDEX payments_one_reversal_per_payment ON payments (target_payment_id)
    WHERE kind = 'reversal';

-- The Refund count against a target is bounded by amount, not by row
-- count, and that bound is a SUM across rows -- something no CHECK can
-- see. PostRefundPaymentHandler locks the target row and sums the Refunds
-- already against it before inserting, the same row-locking shape
-- resolvePaymentForReversal already uses for its own half of the
-- reversal invariant.

-- A Refund that came back through Stripe carries the id of the Stripe
-- object that returned it: the Refund object's re_... when one exists (a
-- card Payment), or the credit note's cn_... when none does (a
-- paid_out_of_band Payment, where Stripe never held the money and the
-- credit note is only a record). That id is also the echo key: the
-- Connect webhook recognizes a credit_note.created or refund.created
-- event as the echo of a Refund this system just issued by finding a
-- Refund row already carrying it. This index is the database's backstop
-- for that guard -- the webhook's own EXISTS check turns a duplicate into
-- a clean skip, but a lost race must not be able to write the row twice.
CREATE UNIQUE INDEX payments_refund_stripe_reference_unique ON payments (stripe_payment_reference)
    WHERE kind = 'refund' AND stripe_payment_reference IS NOT NULL;

-- A Refund's amount is negative for the same reason a reversal's is: "how
-- much of this Invoice is still covered" stays one SUM over the Invoice's
-- rows, with no second code path and no second table.
ALTER TABLE payments DROP CONSTRAINT payments_amount_sign_matches_kind;
ALTER TABLE payments ADD CONSTRAINT payments_amount_sign_matches_kind CHECK (
    (kind IN ('reversal', 'refund') AND amount_cents < 0)
    OR (kind NOT IN ('reversal', 'refund') AND amount_cents > 0)
);

-- Both pointing kinds require a target, and neither of the two
-- non-pointing kinds may carry one.
ALTER TABLE payments DROP CONSTRAINT payments_target_payment_id_matches_kind;
ALTER TABLE payments ADD CONSTRAINT payments_target_payment_id_matches_kind CHECK (
    (kind IN ('reversal', 'refund') AND target_payment_id IS NOT NULL)
    OR (kind NOT IN ('reversal', 'refund') AND target_payment_id IS NULL)
);

-- A Refund's method is the one place this CHECK deliberately stops short
-- of the real rule. Whether a Refund has a method depends on its target:
-- returning a by-hand Payment is itself a by-hand act and names how the
-- money went back (a check written, a bank transfer sent), while a card
-- Payment is returned by Stripe and there is no method to name. That is a
-- fact about another row, which a Postgres CHECK cannot see. So the
-- database bounds only which kinds may ever carry a method, and
-- PostRefundPaymentHandler -- which has already locked and read the
-- target's kind -- enforces when. Same division 00103 drew for
-- reason/'reversal' and 00104 was forced back to for note/'other'.
ALTER TABLE payments DROP CONSTRAINT payments_method_matches_kind;
ALTER TABLE payments ADD CONSTRAINT payments_method_matches_kind CHECK (
    (kind = 'manual' AND method IS NOT NULL)
    OR (kind = 'stripe' AND method IS NULL)
    OR (kind = 'reversal' AND method IS NULL)
    OR kind = 'refund'
);

-- A Refund's free text is note, not reason: #1009 asks for "a method and
-- a note", the same optional scratch space a manually recorded Payment
-- carries (00095), rather than the always-required why a reversal needs.
-- note already has no kind-scoped CHECK to widen, and it is already
-- covered by the erasure sweep that reaches every payments note
-- (client.redactPaymentNotes, ADR-0027) and by payment.csv's export --
-- neither filters on kind, so a Refund's note is swept and exported the
-- day this lands. reason stays exclusively a reversal's; nothing changes.

-- +goose Down
DROP INDEX payments_refund_stripe_reference_unique;
ALTER TABLE payments DROP CONSTRAINT payments_method_matches_kind;
ALTER TABLE payments ADD CONSTRAINT payments_method_matches_kind CHECK (
    (kind = 'manual' AND method IS NOT NULL)
    OR (kind = 'stripe' AND method IS NULL)
    OR (kind = 'reversal' AND method IS NULL)
);
ALTER TABLE payments DROP CONSTRAINT payments_target_payment_id_matches_kind;
ALTER TABLE payments DROP CONSTRAINT payments_amount_sign_matches_kind;
ALTER TABLE payments ADD CONSTRAINT payments_amount_sign_matches_kind CHECK (
    (kind = 'reversal' AND amount_cents < 0) OR (kind <> 'reversal' AND amount_cents > 0)
);
DROP INDEX payments_one_reversal_per_payment;
ALTER TABLE payments RENAME COLUMN target_payment_id TO reversed_payment_id;
CREATE UNIQUE INDEX payments_reversed_payment_id_unique ON payments (reversed_payment_id)
    WHERE reversed_payment_id IS NOT NULL;
ALTER TABLE payments ADD CONSTRAINT payments_reversed_payment_id_matches_kind CHECK (
    (kind = 'reversal' AND reversed_payment_id IS NOT NULL) OR (kind <> 'reversal' AND reversed_payment_id IS NULL)
);

-- Postgres has no DROP VALUE. Rebuilding the enum means rewriting every
-- column that uses it, a larger and riskier operation than the Up it
-- reverses; a spare enum member is inert (00040's and 00103's reasoning).
SELECT 1;
