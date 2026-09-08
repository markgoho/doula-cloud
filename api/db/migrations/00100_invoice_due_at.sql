-- +goose Up
-- #768: an Invoice's own due date, and the per-Practice payment terms it
-- is computed from. Before this, nothing in the model could be late
-- against anything: the Stripe rail set days_until_due=30 on Stripe's
-- object only because Stripe's API rejects collection_method=send_invoice
-- without it, and that date never came home to our row; a by-hand
-- Invoice (00095) had no due date anywhere, in any system.
--
-- The column is deliberately NOT named due_date: "due date" already means
-- the *pregnancy* due date in this domain (ADR-0015, engagements), and
-- the two must not share a word.
--
-- Overdue is derived, never stored: `status = 'open' AND due_at < now()`.
-- Stripe emits no event when a send_invoice due date passes (verified
-- against the Sandbox on a test clock advanced a month past a $50,000
-- Invoice's due date -- invoice.finalized and invoice.updated, and
-- nothing else), and nothing in this repo fires on time either, so a
-- stored flag would need a scheduled sweep to keep it true and would be
-- wrong between sweeps. invoice_status gains no value for the same
-- reason: an overdue Invoice is an 'open' one.
--
-- DEFAULT-then-DROP per guardrail_test.go: the DEFAULT backfills rows
-- already in the table (there are none -- pre-launch, no production
-- data), and dropping it means every later INSERT must say what it
-- means rather than silently inherit a 30-day term.
ALTER TABLE invoices ADD COLUMN due_at timestamptz NOT NULL DEFAULT now() + interval '30 days';
ALTER TABLE invoices ALTER COLUMN due_at DROP DEFAULT;

-- The overdue read is (practice_id, due_at) over open Invoices alone, so
-- the index is partial on exactly that predicate rather than a third
-- full index on the table: an Invoice stops being able to become overdue
-- the moment it leaves 'open'. This keeps both the ?overdue=true
-- narrowing and the whole-book overdue totals index-supported over a
-- Practice's real book instead of a sequential scan of it.
CREATE INDEX invoices_practice_due_open_idx ON invoices (practice_id, due_at)
    WHERE status = 'open';

-- A Practice's payment terms: how many days after an Invoice is raised
-- it falls due. One row per Practice, and a missing row is a valid
-- state meaning **30 days** -- practice_rates (00096) is the precedent
-- for a per-Practice money setting whose absent row is meaningful rather
-- than zero, with one difference: absent here is a real default, not
-- "unset", because 30 is what the Stripe rail already applied before
-- this migration, so adopting it changes no existing Practice's
-- behavior. The default lives in one place in Go
-- (payments.DefaultPaymentTermsDays) and is documented here because this
-- is where the setting lives.
--
-- Terms are whole days: net-15, net-30 and net-45 are how a US payer
-- states them, and nothing in the pilot bills by the hour of a day.
CREATE TABLE practice_payment_terms (
    practice_id uuid PRIMARY KEY REFERENCES practices (id),
    net_days integer NOT NULL CHECK (net_days > 0 AND net_days <= 365),
    created_at timestamptz NOT NULL DEFAULT now()
);

GRANT SELECT, INSERT, UPDATE, DELETE ON practice_payment_terms TO app_runtime;

ALTER TABLE practice_payment_terms ENABLE ROW LEVEL SECURITY;

-- Same plain practice_id comparison practice_rates_practice_visibility
-- uses (00096), fail-closed when app.current_practice_id is unset.
CREATE POLICY practice_payment_terms_practice_visibility ON practice_payment_terms
    USING (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid);

-- +goose Down
DROP TABLE practice_payment_terms;
DROP INDEX invoices_practice_due_open_idx;
ALTER TABLE invoices DROP COLUMN due_at;
