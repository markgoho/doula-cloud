-- +goose Up
-- The website a Practice declares to Stripe (#440), and the audit trail
-- for it.
--
-- Stripe's hosted onboarding demands a website URL from every connected
-- account, and #421 walked what happens when it does not get one: the
-- field accepts an empty value, she clicks through every remaining step,
-- submits, and comes back to us "done" with charges_enabled false and
-- nothing on screen saying why. A social profile satisfies the field --
-- a Facebook page URL cleared the requirement on the probe account --
-- but Stripe then derives the statement descriptor from the URL, which
-- put FACEBOOK.COM/ROCHESTER onto that account's Clients' card
-- statements.
--
-- So a Practice gets a choice, and this table is where the choice lives:
-- declare her own website (or social profile), or have Doula Cloud
-- publish a page for her at doula.cloud/p/<slug> (#441). Nothing in the
-- schema could hold either answer -- practices carries a name, four
-- Stripe Connect columns and nothing describing the business.
--
-- A table rather than more columns on practices, because two tickets
-- already need to land on this fact and both are additive here: #441
-- needs the slug the page is published under, and #443 needs whether the
-- published page actually resolved after a deploy. Both belong beside
-- the mode and neither belongs on practices.
--
-- Absence of a row is the third state. A Practice that has not answered
-- has no row, which is what every Practice that exists today is, so
-- there is no backfill and no invented default that could be mistaken
-- for an answer she gave.
-- ---------------------------------------------------------------------

CREATE TABLE practice_websites (
    -- One row per Practice, and the primary key says so: a Practice
    -- declares one website to Stripe, and Stripe holds one URL per
    -- connected account.
    practice_id uuid PRIMARY KEY REFERENCES practices (id),

    -- 'own'    -- she declared a URL of her own; nothing is published
    --             under doula.cloud and own_url is what Stripe is told.
    -- 'hosted' -- she published a page here; #441 generates it and
    --             Stripe is told doula.cloud/p/<slug>.
    --
    -- There is no 'draft'. Publishing is explicit (#440): the two facts
    -- are collected, shown back to her assembled as the page will read,
    -- and written only when she publishes. A row with mode 'hosted'
    -- therefore means a live page, which is exactly what #441's build
    -- step needs to select on.
    mode text NOT NULL CHECK (mode IN ('own', 'hosted')),

    own_url text,

    -- The two facts only she has. Everything else the page carries --
    -- the business name, the support contact, the shared privacy
    -- statement -- is assembled from what she has already told us
    -- (#441). A blank box for the rest would let her publish something
    -- Stripe rejects.
    service_description text,
    cancellation_policy text,

    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),

    -- Each mode demands its own facts, at the boundary that can enforce
    -- it. The other mode's columns are deliberately left alone rather
    -- than nulled out: a Practice who publishes a page, then switches to
    -- her own site, then changes her mind again should not have to write
    -- her cancellation policy twice.
    CONSTRAINT practice_websites_own_url_present
        CHECK (mode <> 'own' OR own_url IS NOT NULL),
    CONSTRAINT practice_websites_hosted_facts_present
        CHECK (mode <> 'hosted'
               OR (service_description IS NOT NULL AND cancellation_policy IS NOT NULL)),

    -- The character budget, in the one place that can actually hold the
    -- line. The screen counts down as she types and the handler checks
    -- before it writes; this is what makes a page that cannot be
    -- rendered impossible rather than merely unlikely. 500 characters is
    -- roughly eighty words -- room for a real cancellation policy, and
    -- short enough that the published page stays a page.
    CONSTRAINT practice_websites_service_description_budget
        CHECK (service_description IS NULL
               OR char_length(service_description) BETWEEN 1 AND 500),
    CONSTRAINT practice_websites_cancellation_policy_budget
        CHECK (cancellation_policy IS NULL
               OR char_length(cancellation_policy) BETWEEN 1 AND 500),
    -- The longest URL every mainstream browser handles, and far longer
    -- than any social profile.
    CONSTRAINT practice_websites_own_url_budget
        CHECK (own_url IS NULL OR char_length(own_url) BETWEEN 1 AND 2048)
);

COMMENT ON TABLE practice_websites IS
    'The website a Practice declares to Stripe: her own URL, or a page published for her at doula.cloud/p/<slug>. No row means she has not answered.';

-- No DELETE grant: switching mode is an UPDATE, and a Practice never
-- goes back to having never answered.
GRANT SELECT, INSERT, UPDATE ON practice_websites TO app_runtime;

ALTER TABLE practice_websites ENABLE ROW LEVEL SECURITY;

-- practice_id is on the row, so the policy is a plain column comparison
-- rather than an EXISTS subquery -- the shape practice_memberships
-- (00002) uses. Unset app.current_practice_id matches nothing, which is
-- fail-closed.
CREATE POLICY practice_websites_practice_visibility ON practice_websites
    USING (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid)
    WITH CHECK (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid);

-- ---------------------------------------------------------------------
-- practice_website_events: how the current answer came to be. The same
-- append-only shape practice_membership_events (00039) and
-- staff_work_state_events (00043) have, for CLAUDE.md's audit-trail
-- expectation.
--
-- Unlike those two, this one snapshots the content and not only the
-- transition. Stripe's review of a declared website is ongoing and has
-- no published SLA (#382), so "what did this page say when Stripe last
-- looked at it?" is a question with a real answer date attached, and the
-- current row cannot answer it after an edit.
--
-- previous_mode is NULL for the first event -- the one written when a
-- Practice answers for the first time.
--
-- actor_staff_id exists because only an Owner may write, and which Owner
-- is the point: a fourteen-doula agency has one page and more than one
-- person who could have published it.
--
-- No UPDATE, no DELETE grant: an event is a fact about the past.
-- ---------------------------------------------------------------------
CREATE TABLE practice_website_events (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    practice_id         uuid NOT NULL REFERENCES practices (id),
    previous_mode       text,
    mode                text NOT NULL CHECK (mode IN ('own', 'hosted')),
    own_url             text,
    service_description text,
    cancellation_policy text,
    actor_staff_id      uuid NOT NULL REFERENCES staff (id),
    created_at          timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX practice_website_events_practice
    ON practice_website_events (practice_id, created_at DESC);

GRANT SELECT, INSERT ON practice_website_events TO app_runtime;

ALTER TABLE practice_website_events ENABLE ROW LEVEL SECURITY;

CREATE POLICY practice_website_events_practice_visibility ON practice_website_events
    USING (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid)
    WITH CHECK (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid);

-- +goose Down
DROP TABLE practice_website_events;
DROP TABLE practice_websites;
