-- +goose Up
-- The site rebuild a Practice's publish has to trigger, and the proof
-- that her page actually resolves once it has (#443).
--
-- 00046 gave a hosted Practice her slug and gave the site generator a
-- role to read it as. Neither made anything *happen*: the deploy
-- workflow fires on a push touching hugo/**, and a Practice publishing
-- her page produces no commit, so nothing builds and her page never
-- appears. #421 decided the BFF fires the deploy itself rather than a
-- schedule that burns builds on nothing, and this is the durable half
-- of that -- a publish queues a row here, and a worker turns queued
-- rows into one repository_dispatch.
--
-- Two tables' worth of change, for two halves of the same problem: the
-- outbox that makes the build happen, and the liveness state that says
-- whether it worked.
-- ---------------------------------------------------------------------

CREATE TYPE site_build_outbox_status AS ENUM ('pending', 'dispatched', 'dead_lettered');

-- One row per act that makes the published site stale: a publish, an
-- edit, or a switch away to her own website (which has to prune her
-- page, and is therefore just as much a reason to rebuild as publishing
-- was).
--
-- Unlike the seven notification outboxes this copies, a row here is not
-- a message to anybody -- it carries no recipient and no payload,
-- because the build reads the database for itself. It is a claim that
-- the live site no longer matches what is stored, and every pending row
-- makes exactly the same claim. That is what lets the worker collapse
-- all of them into one dispatch: two Practices publishing a minute
-- apart need one rebuild between them, not two.
--
-- practice_id is kept anyway, and is the reason this is a table of rows
-- rather than a single dirty flag: it answers "why did the site rebuild
-- at 14:03?" without having to correlate timestamps against
-- practice_website_events. The actor is not duplicated here -- 00045's
-- event row already records who published.
CREATE TABLE site_build_outbox (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    practice_id uuid NOT NULL REFERENCES practices (id),
    status site_build_outbox_status NOT NULL DEFAULT 'pending',
    attempt_count int NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    dispatched_at timestamptz,
    last_error text
);

-- The worker's only query is "the oldest pending row, and then all of
-- them", so pending rows are what needs to be findable.
CREATE INDEX site_build_outbox_pending ON site_build_outbox (created_at)
    WHERE status = 'pending';

GRANT SELECT, INSERT, UPDATE ON site_build_outbox TO app_runtime;

-- No RLS, matching every other outbox: the worker runs with no Practice
-- or Client session context, and a row here is platform-level work
-- rather than one Practice's data.

-- ---------------------------------------------------------------------
-- Whether the page is actually there.
--
-- Stripe holds the declared URL for the life of the connected account
-- and reviews it on its own schedule with no published SLA (#382), so a
-- page that does not resolve is a rejection arriving weeks later with
-- no visible cause. The product therefore has to know -- and has to
-- distinguish "not confirmed yet" from "confirmed working", because a
-- build that fails produces no deploy, no callback and no news at all.
-- Absence of a report must never read as a pass.
--
-- Three states, and the transitions are the whole design:
--   pending  -- she published or edited, and no probe has confirmed the
--               result yet. Set by the write site, on every write.
--   live     -- a probe fetched doula.cloud/p/<slug> and got the page.
--   failed   -- a probe ran and did not get the page.
-- Only an affirmative probe leaves 'pending'. A page nothing ever
-- checks stays 'pending' forever and says so on her screen, which is
-- the honest answer rather than a silent success.
CREATE TYPE practice_page_state AS ENUM ('pending', 'live', 'failed');

ALTER TABLE practice_websites
    ADD COLUMN page_state practice_page_state,
    ADD COLUMN page_checked_at timestamptz,
    ADD COLUMN page_check_detail text;

COMMENT ON COLUMN practice_websites.page_state IS
    'Whether doula.cloud/p/<slug> has been confirmed to resolve. NULL for a Practice using her own website, which has no page here to check.';
COMMENT ON COLUMN practice_websites.page_checked_at IS
    'When a probe last ran against this page. NULL until one has.';
COMMENT ON COLUMN practice_websites.page_check_detail IS
    'Why the last probe failed, in a few words, for the Practice to read. NULL when it did not fail.';

-- The state exists only where there is a page. A Practice on her own
-- website keeps her slug (00046, deliberately) but has nothing
-- published, so the honest value is no value rather than a stale one.
ALTER TABLE practice_websites ADD CONSTRAINT practice_websites_page_state_matches_mode
    CHECK ((mode = 'hosted') = (page_state IS NOT NULL));

-- Existing hosted rows in a development database predate the column and
-- would fail the CHECK. Nothing has probed them, so 'pending' is true.
UPDATE practice_websites SET page_state = 'pending'
 WHERE mode = 'hosted' AND page_state IS NULL;

-- ---------------------------------------------------------------------
-- The door the probe worker reads and writes through.
--
-- Every policy on practice_websites scopes to one Practice, which is
-- right for the BFF and wrong for a sweep that has to check every
-- published page in one pass -- the same problem 00046 solved for the
-- site generator, and solved differently, because that job only reads
-- and this one has to write back what it found.
--
-- Its own setting name rather than app.notification_worker_trusted: the
-- seven notification workers each open that door on their own endpoint,
-- and none of them has any business writing a page's liveness. One
-- door, one worker.
-- +goose StatementBegin
CREATE POLICY practice_websites_site_worker ON practice_websites
    FOR SELECT
    USING (current_setting('app.site_worker_trusted', true) = 'true');
-- +goose StatementEnd

-- +goose StatementBegin
CREATE POLICY practice_websites_site_worker_update ON practice_websites
    FOR UPDATE
    USING (current_setting('app.site_worker_trusted', true) = 'true')
    WITH CHECK (current_setting('app.site_worker_trusted', true) = 'true');
-- +goose StatementEnd

-- +goose Down
DROP POLICY practice_websites_site_worker_update ON practice_websites;
DROP POLICY practice_websites_site_worker ON practice_websites;
ALTER TABLE practice_websites DROP CONSTRAINT practice_websites_page_state_matches_mode;
ALTER TABLE practice_websites
    DROP COLUMN page_check_detail,
    DROP COLUMN page_checked_at,
    DROP COLUMN page_state;
DROP TYPE practice_page_state;
DROP TABLE site_build_outbox;
DROP TYPE site_build_outbox_status;
