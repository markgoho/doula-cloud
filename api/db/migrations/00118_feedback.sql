-- +goose Up
-- The Feedback table (#1523, under the #1519 pilot feedback mechanism).
-- GLOSSARY.md's Feedback glossary entry and #1501's resolution comment
-- (Q1) settle the field list; #1498's resolution comment (Q5) settles
-- the three kinds. This migration stores a piece of Feedback only --
-- opening a private GitHub issue for it is #1524's outbox, erasure and
-- retention are #1501's own follow-up tickets, and the founder read page
-- is #1499. None of those are built here.
--
-- A piece of Feedback is sent by exactly one of a Staff member
-- (staff_id) or a Portal Account (portal_account, portal_accounts'
-- identifier -- ADR-0015, migration 00108: one Portal Account reaches
-- Clients at more than one Practice, which is why this column names the
-- login rather than a client_portal_users row). client_id and
-- practice_id ride together, and only for a Portal piece, when the
-- screen it was sent from was inside one Engagement; roles rides with
-- practice_id, and only for a Staff piece, when the screen was under
-- practices/[practiceId]. feedback_sender below is the single CHECK that
-- ties all of that together, so a row that names both a Staff member and
-- a Portal Account, or a Staff piece carrying a client_id, is impossible
-- rather than merely undocumented.
--
-- id is not DEFAULT gen_random_uuid(): the handler mints it in Go so the
-- insert never needs a RETURNING clause, the same reason message and
-- visit already do this.
--
-- issue_number is nullable and untouched here -- #1524's own outbox
-- worker fills it in once the private GitHub issue exists. No status
-- column: with nothing else reading or writing this table yet, "is this
-- row's issue open" is exactly "issue_number IS NULL", and #1524 is free
-- to widen this if the outbox needs more than that.
CREATE TYPE feedback_kind AS ENUM ('not_working', 'idea_or_request', 'something_else');

CREATE TABLE feedback (
    id             uuid PRIMARY KEY,
    kind           feedback_kind NOT NULL,
    -- Free text, never NULL -- an empty piece of Feedback (kind and
    -- nothing typed) is allowed, and NULL would mean two representations
    -- of "nothing to add" for every reader downstream to reconcile.
    text           text NOT NULL,
    -- The full URL with its ids, database-only per #1501 Q1 -- the
    -- private GitHub issue carries route_id, never this.
    page_url       text NOT NULL,
    -- The route pattern, with its route groups, e.g.
    -- "/(app)/practices/[practiceId]/clients/[clientId]" -- what #1501
    -- Q1 says does cross to GitHub.
    route_id       text NOT NULL,
    app_build      text NOT NULL,
    screen_width   integer NOT NULL,
    -- Family plus major version only, e.g. "Safari 18" -- the BFF parses
    -- this from User-Agent and never stores the raw header.
    browser        text NOT NULL,
    staff_id       uuid REFERENCES staff (id),
    portal_account text REFERENCES portal_accounts (identifier),
    practice_id    uuid REFERENCES practices (id),
    client_id      uuid REFERENCES clients (id),
    roles          practice_role[],
    issue_number   integer,
    -- The audit trail is the row itself: who sent it (staff_id or
    -- portal_account, above) and when.
    sent_at        timestamptz NOT NULL,
    CONSTRAINT feedback_sender CHECK (
        (staff_id IS NOT NULL AND portal_account IS NULL AND client_id IS NULL
            AND (practice_id IS NULL) = (roles IS NULL))
        OR
        (staff_id IS NULL AND portal_account IS NOT NULL
            AND (practice_id IS NULL) = (client_id IS NULL)
            AND roles IS NULL)
    )
);

-- INSERT only. No SELECT: neither a Staff member nor a Client may read a
-- piece of Feedback back (#1523's own AC) -- the founder read page
-- (#1499) is the one surface that will ever query this table, and it
-- adds its own grant and policy when it is built. No UPDATE or DELETE:
-- #1524's outbox will need UPDATE to fill in issue_number, and adds that
-- grant itself rather than this migration guessing its shape early.
GRANT INSERT ON feedback TO app_runtime;

ALTER TABLE feedback ENABLE ROW LEVEL SECURITY;

-- The whole of this table's access story: a caller may insert a row
-- naming herself as the sender and nobody else. current_staff_id()
-- (00003) resolves app.current_identity_uid to a staff id and works
-- whether or not app.current_practice_id is set, which is what makes it
-- fit here -- POST /api/staff/feedback is a pre-Practice route
-- (mountSessionRoutes), so no {practiceId} is ever chosen for this
-- request. The Portal half compares portal_account against
-- app.current_identity_uid directly, the same shape portal_accounts_
-- self_insert (00073) already uses -- a Portal Account's identifier
-- *is* what that session variable holds once a Portal caller is
-- resolved.
--
-- No SELECT half, and none is added by WITH CHECK defaulting to USING:
-- an INSERT-only policy has no USING clause to default from, so this
-- table admits no read to app_runtime at all, which the missing GRANT
-- above already forces regardless.
-- +goose StatementBegin
CREATE POLICY feedback_sender_insert ON feedback
    FOR INSERT
    WITH CHECK (
        (staff_id IS NOT NULL AND staff_id = current_staff_id())
        OR (portal_account IS NOT NULL AND portal_account = NULLIF(current_setting('app.current_identity_uid', true), ''))
    );
-- +goose StatementEnd

-- +goose Down
DROP POLICY feedback_sender_insert ON feedback;
DROP TABLE feedback;
DROP TYPE feedback_kind;
