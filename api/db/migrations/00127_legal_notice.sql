-- +goose Up
-- #1548 (ADR-0053): a material change to the Terms of Service or the
-- Privacy Policy emails each Owner of each Practice 30 days or more
-- before it starts. Two tables: the record of the act that published a
-- version, and the outbox that carries one email to one Owner.
--
-- The version history itself stays in api/internal/legal, the one place
-- the BFF reads versions from (#1556). These tables hold only what
-- happened to a version: who published it and when, and who was told.

-- =====================================================================
-- legal_version_publications: who published a version, and when
-- =====================================================================
--
-- One row per published version. document is the document's address
-- without its slash (legal.Terms.Path is /terms), and version is the
-- date it takes effect, which is the version's identity (ADR-0053).
-- material is copied from the version's own record at the moment it was
-- published, so the row still says what was decided if the history in
-- Go is ever wrong. published_at comes from clock.Now(ctx) (#773).
--
-- No RLS, on purpose. That a document changes on a date is a public
-- fact, printed on the marketing site, and #1549 reads this table from
-- an Owner's own session to know that a material version was announced
-- before it asks her to agree. Only the operator endpoint behind the
-- internal guard writes it, and nothing updates or deletes a row: the
-- first record of who published a version is the answer for good.
CREATE TABLE legal_version_publications (
    document     text NOT NULL
                 CONSTRAINT legal_version_publications_document_known
                 CHECK (document IN ('terms', 'privacy')),
    version      date NOT NULL,
    material     boolean NOT NULL,
    published_by text NOT NULL
                 CONSTRAINT legal_version_publications_published_by_named
                 CHECK (btrim(published_by) <> ''),
    published_at timestamptz NOT NULL,
    PRIMARY KEY (document, version)
);

GRANT SELECT, INSERT ON legal_version_publications TO app_runtime;

-- =====================================================================
-- legal_notice_outbox: one email to one Owner (ADR-0010)
-- =====================================================================
--
-- One row per Owner per Practice, not one per Practice. With one row per
-- Practice, a suppressed address (ADR-0029) among several Owners is
-- skipped silently by outbox.Worker.SendAll, and "was she told?" has no
-- answer. With one row each, a suppressed address dead-letters its own
-- row with the reason in last_error.
--
-- The row names the recipient by staff_id, never by address: the worker
-- reads the address at send time, so this table is not a fourth place an
-- Owner's address survives the deletion of her login (ADR-0033).
-- created_at is the instant the email was queued, from clock.Now(ctx),
-- and has no default for that reason. next_attempt_at keeps Postgres's
-- clock as its default, the rule outbox.Worker.HandlePending's claim
-- query depends on.
--
-- Platform-level like low_credit_outbox (00033): the worker runs with no
-- Practice session, so this table carries no RLS of its own, and it
-- resolves the address through 00033's notification-worker policies on
-- staff.
CREATE TYPE legal_notice_outbox_status AS ENUM ('pending', 'sent', 'dead_lettered');

CREATE TABLE legal_notice_outbox (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    document        text NOT NULL,
    version         date NOT NULL,
    practice_id     uuid NOT NULL REFERENCES practices (id),
    staff_id        uuid NOT NULL REFERENCES staff (id),
    status          legal_notice_outbox_status NOT NULL DEFAULT 'pending',
    attempt_count   int NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    created_at      timestamptz NOT NULL,
    sent_at         timestamptz,
    last_error      text,
    FOREIGN KEY (document, version) REFERENCES legal_version_publications (document, version),
    CONSTRAINT legal_notice_outbox_one_per_owner UNIQUE (document, version, practice_id, staff_id)
);

CREATE INDEX legal_notice_outbox_due
    ON legal_notice_outbox (next_attempt_at)
    WHERE status = 'pending';

GRANT SELECT, INSERT, UPDATE ON legal_notice_outbox TO app_runtime;

-- +goose Down
DROP TABLE legal_notice_outbox;
DROP TYPE legal_notice_outbox_status;
DROP TABLE legal_version_publications;
