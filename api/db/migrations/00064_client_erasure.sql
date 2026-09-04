-- +goose Up
-- ADR-0027 (docs/adr/0027-erasure-redacts-in-place-and-shreds-the-key.md),
-- the schema an Owner-run Client erasure needs (#394). Four pieces:
-- the proof the act ran (clients.erased_at), the key crypto-shredding
-- destroys (client_data_keys), the Stripe Customer id erasure has to
-- reach (invoices.stripe_customer_id, which nothing persisted before),
-- and the outbox that carries the three outside-world acts.
--
-- activity is untouched here on purpose. Its GRANT stays SELECT, INSERT
-- and its policy stays as 00051 wrote it -- the whole point of the
-- crypto-shredding design is that erasure never needs a schema change
-- there.

-- =====================================================================
-- clients.erased_at
-- =====================================================================

-- Proof the act ran, and the gate every other Client write reads: an
-- erased Client cannot be edited and cannot be erased a second time.
-- Nullable, because "not erased" is the normal state and has no date.
ALTER TABLE clients ADD COLUMN erased_at timestamptz;

-- =====================================================================
-- client_data_keys
-- =====================================================================

-- One random 256-bit AES-GCM key per Client, sealing the activity diffs
-- that carry her personal data. Unlike activity itself this is an
-- ordinary mutable table and it holds a DELETE grant, because deleting
-- the row IS the erasure -- that asymmetry is the design (ADR-0027).
--
-- practice_id rides on the row rather than being reached through clients,
-- so the RLS policy is the same plain comparison every other
-- practice-scoped table uses and a key read costs no join.
CREATE TABLE client_data_keys (
    client_id   uuid PRIMARY KEY REFERENCES clients (id),
    practice_id uuid NOT NULL REFERENCES practices (id),
    key         bytea NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT client_data_keys_key_length CHECK (octet_length(key) = 32)
);

GRANT SELECT, INSERT, DELETE ON client_data_keys TO app_runtime;   -- no UPDATE: a key is never rotated, only made and destroyed

ALTER TABLE client_data_keys ENABLE ROW LEVEL SECURITY;

-- +goose StatementBegin
CREATE POLICY client_data_keys_practice_visibility ON client_data_keys
    USING (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid)
    WITH CHECK (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid);
-- +goose StatementEnd

-- =====================================================================
-- client_portal_users: one more door, for erasure only
-- =====================================================================

-- Erasure clears identity_uid once the Identity Platform account behind
-- it is queued for deletion, so nothing in this database points at an
-- account that is about to stop existing. client_portal_users_invite_
-- update (00042) cannot carry that: it admits only rows whose
-- identity_uid is already NULL -- an accepted portal account is
-- deliberately immutable through it.
--
-- Policies are permissive, so this is a second, narrower door rather
-- than a widening of the first: it opens only for a Client already
-- marked erased, at the caller's own Practice, and only for an update
-- that ends with both the identity and the invite token gone. There is
-- no state it admits that is not the end state of an erasure.
-- +goose StatementBegin
CREATE POLICY client_portal_users_erasure_update ON client_portal_users
    FOR UPDATE
    USING (
        EXISTS (
            SELECT 1 FROM clients c
            WHERE c.id = client_portal_users.client_id
              AND c.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
              AND c.erased_at IS NOT NULL
        )
    )
    WITH CHECK (identity_uid IS NULL AND invite_token IS NULL);
-- +goose StatementEnd

-- =====================================================================
-- invoices.stripe_customer_id
-- =====================================================================

-- payments.CreateInvoice has always created a Stripe Customer per invoice
-- and thrown the id away, so before this column there was nothing for
-- erasure to delete. Nullable: rows written before this migration have no
-- id to backfill from, and erasure skips them (recording that it did).
-- Not UNIQUE -- one Customer per invoice is today's shape, not a rule.
ALTER TABLE invoices ADD COLUMN stripe_customer_id text;

CREATE INDEX invoices_stripe_customer ON invoices (stripe_customer_id)
    WHERE stripe_customer_id IS NOT NULL;

-- =====================================================================
-- client_erasure_outbox
-- =====================================================================

-- The three acts erasure cannot finish inside its own transaction,
-- because each is a call to somebody else's API: delete the Stripe
-- Customer, delete the Identity Platform account, and -- 90 days after
-- her newest transaction, never sooner (ADR-0027) -- run the Stripe
-- Redaction Job. Same row-for-row shape as engagement_offer_outbox
-- (00041) and staff_invite_outbox (00038), with two additions: kind,
-- because one erasure enqueues three different acts, and target, which
-- carries the one identifier the act needs (a cus_..., an Identity
-- Platform uid). next_attempt_at is what defers the redaction job: it is
-- set to the eligibility date at enqueue time rather than now().
CREATE TYPE client_erasure_act AS ENUM
    ('stripe_customer_delete', 'stripe_redaction_job', 'identity_account_delete');

CREATE TYPE client_erasure_outbox_status AS ENUM ('pending', 'sent', 'dead_lettered');

CREATE TABLE client_erasure_outbox (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id       uuid NOT NULL REFERENCES clients (id),
    practice_id     uuid NOT NULL REFERENCES practices (id),
    act             client_erasure_act NOT NULL,
    target          text NOT NULL,
    status          client_erasure_outbox_status NOT NULL DEFAULT 'pending',
    attempt_count   int NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    created_at      timestamptz NOT NULL DEFAULT now(),
    sent_at         timestamptz,
    last_error      text
);

-- At most one pending row per (Client, act, target) -- the same partial
-- unique index every other outbox uses, keyed on the triple because one
-- Client can hold several Stripe Customers (one per invoice, see above)
-- and each is its own act.
CREATE UNIQUE INDEX client_erasure_outbox_one_pending
    ON client_erasure_outbox (client_id, act, target)
    WHERE status = 'pending';

CREATE INDEX client_erasure_outbox_claim
    ON client_erasure_outbox (next_attempt_at)
    WHERE status = 'pending';

GRANT SELECT, INSERT, UPDATE ON client_erasure_outbox TO app_runtime;

-- No RLS -- platform-level like every other outbox table: the worker runs
-- with no Practice or Client session context. The enqueue side runs under
-- the Owner's own practice-scoped transaction and writes practice_id from
-- it; the column is here so the worker and a later report can say which
-- Practice an act belonged to without joining back through a Client whose
-- row has been redacted.

-- +goose Down
DROP TABLE client_erasure_outbox;
DROP TYPE client_erasure_outbox_status;
DROP TYPE client_erasure_act;

DROP INDEX invoices_stripe_customer;
ALTER TABLE invoices DROP COLUMN stripe_customer_id;

DROP POLICY client_portal_users_erasure_update ON client_portal_users;

DROP POLICY client_data_keys_practice_visibility ON client_data_keys;
DROP TABLE client_data_keys;

ALTER TABLE clients DROP COLUMN erased_at;
