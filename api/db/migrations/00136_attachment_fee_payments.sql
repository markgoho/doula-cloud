-- +goose Up
-- #2271: a Fee Payment (GLOSSARY.md) is the Practice's record that it paid
-- a contractor Doula the fee she accepted, outside DoulaCloud. A record
-- only: no money moves through the product (#391, ADR-0007).
--
-- Keyed to the Attachment the accepted Offer copied the fee onto, so the
-- record outlives the Attachment's end the way the Attachment itself does.
-- practice_id is stored on the row so its policy is a plain column
-- compare rather than an EXISTS through engagements.
--
-- Append-only, as payments is (#945): a mistaken record is undone by a
-- 'reversal' row that targets it, never by an UPDATE or a DELETE.
CREATE TABLE attachment_fee_payments (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    practice_id   uuid NOT NULL REFERENCES practices (id),
    attachment_id uuid NOT NULL REFERENCES engagement_attachments (id),
    kind          text NOT NULL,
    amount_cents  bigint NOT NULL,
    method        payment_method,
    note          text,
    paid_on       date,
    target_id     uuid REFERENCES attachment_fee_payments (id),
    reason        text,
    recorded_by   uuid NOT NULL REFERENCES staff (id),
    recorded_at   timestamptz NOT NULL,

    -- note and reason are free text a Client's erasure empties
    -- (client.redactPaymentNotes, ADR-0027), so neither is required here;
    -- the handlers require them.
    CONSTRAINT attachment_fee_payments_shape CHECK (
        (kind = 'recorded'
            AND amount_cents > 0 AND method IS NOT NULL AND paid_on IS NOT NULL
            AND target_id IS NULL AND reason IS NULL)
        OR (kind = 'reversal'
            AND amount_cents < 0 AND method IS NULL AND note IS NULL AND paid_on IS NULL
            AND target_id IS NOT NULL)
    )
);

CREATE UNIQUE INDEX attachment_fee_payments_one_reversal
    ON attachment_fee_payments (target_id)
    WHERE kind = 'reversal';

CREATE INDEX attachment_fee_payments_attachment
    ON attachment_fee_payments (attachment_id);

GRANT SELECT, INSERT ON attachment_fee_payments TO app_runtime;
GRANT UPDATE (note, reason) ON attachment_fee_payments TO app_runtime;

ALTER TABLE attachment_fee_payments ENABLE ROW LEVEL SECURITY;

CREATE POLICY attachment_fee_payments_practice_visibility ON attachment_fee_payments
    USING (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid);

-- +goose Down
DROP TABLE attachment_fee_payments;
