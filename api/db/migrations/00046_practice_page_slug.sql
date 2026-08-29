-- +goose Up
-- The slug a hosted Practice page is published under, and the read-only
-- role that builds the site from it (#441).
--
-- 00045 anticipated both halves of this: it said #441 "needs the slug
-- the page is published under", and it said the mode is what the build
-- step selects on. This adds the slug beside the mode, exactly where
-- 00045 said it belonged, and adds the one thing 00045 could not have
-- known was missing -- a way for anything to read across Practices at
-- all.
--
-- Two facts make the slug's shape non-negotiable. Stripe holds the
-- declared URL for the life of the connected account, and #382
-- established that its review of that URL is ongoing with no published
-- SLA. So a page that moves is a live account's review pointed at a 404.
-- The slug is therefore assigned once, when a Practice first publishes,
-- and is never recomputed -- not on a rename, not on a rebuild. It is a
-- stored fact and not a function of practices.name, which an Owner may
-- edit whenever she likes.
-- ---------------------------------------------------------------------

ALTER TABLE practice_websites ADD COLUMN slug text;

COMMENT ON COLUMN practice_websites.slug IS
    'The path segment of doula.cloud/p/<slug>. Assigned once at first publish and never recomputed: Stripe holds the URL and its review is ongoing (#382), so a slug that moves is a 404 under a live review.';

-- Unique across every Practice, because it is a path on one host. NULL
-- is not a value for uniqueness purposes, so every Practice that has
-- never published a hosted page coexists happily.
CREATE UNIQUE INDEX practice_websites_slug_key ON practice_websites (slug);

-- A hosted row without a slug is a page with no address -- the build
-- step would have nowhere to write it. The constraint is what makes
-- that impossible rather than merely unlikely, in the same place 00045
-- put the two facts a hosted page needs.
--
-- Deliberately one-directional: a Practice who publishes and then
-- switches to her own website keeps the slug, so switching back
-- republishes the same URL Stripe was already told about instead of
-- minting a second one.
ALTER TABLE practice_websites ADD CONSTRAINT practice_websites_hosted_slug_present
    CHECK (mode <> 'hosted' OR slug IS NOT NULL);

-- No rows exist in production -- there is none of it yet -- but a
-- development database seeded before this migration can hold a hosted
-- row, and the CHECK above would refuse to validate against it. The
-- backfill derives what the handler now derives, close enough for a
-- fixture: lowercase, non-alphanumerics collapsed to single hyphens,
-- trimmed, and the Practice id when the name leaves nothing behind.
UPDATE practice_websites w
   SET slug = COALESCE(
           NULLIF(trim(both '-' from regexp_replace(lower(p.name), '[^a-z0-9]+', '-', 'g')), ''),
           'practice-' || left(w.practice_id::text, 8))
  FROM practices p
 WHERE p.id = w.practice_id
   AND w.mode = 'hosted'
   AND w.slug IS NULL;

-- ---------------------------------------------------------------------
-- site_builder: the role the static site generator reads as.
--
-- Every policy in this schema scopes a read to one Practice, which is
-- exactly right for the BFF and exactly wrong for a build that has to
-- render every published page in one pass. Nothing could do that today.
--
-- The alternative was to hand the build job the app_runtime credential
-- and widen a policy. That credential can write, and it lives in a job
-- whose only output is a public website; a role that can read five
-- tables and write nothing is the boundary that actually enforces what
-- this job is allowed to do.
--
-- What it may read is deliberately not "the published pages" alone. The
-- page carries a support contact, and that comes from the Owner who
-- published it -- so the generator joins practice_website_events, staff
-- and practice_memberships to find her. Each is SELECT, and each is
-- read only in service of rendering a page that is public by
-- construction.
--
-- Guarded the same way 00002 guards app_runtime: roles are cluster-wide
-- in Cloud SQL, so re-applying must be a no-op rather than a blocked
-- deploy. NOLOGIN -- the login user the build job dials in as is granted
-- this role, so the password lives in Secret Manager and never in a
-- migration.
-- ---------------------------------------------------------------------
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'site_builder') THEN
        CREATE ROLE site_builder NOLOGIN;
    END IF;
END
$$;
-- +goose StatementEnd

GRANT SELECT ON practice_websites, practices, practice_website_events, staff, practice_memberships
    TO site_builder;

-- Permissive policies OR together, so these sit beside the per-Practice
-- policies rather than replacing them: app_runtime still sees exactly
-- one Practice, and site_builder sees every published page. The TO
-- clause is what keeps the two apart -- the existing policies name no
-- role and so apply to everyone, including this one, which is why a
-- grant alone would have returned zero rows.
CREATE POLICY practice_websites_site_builder ON practice_websites
    FOR SELECT TO site_builder USING (true);
CREATE POLICY practices_site_builder ON practices
    FOR SELECT TO site_builder USING (true);
CREATE POLICY practice_website_events_site_builder ON practice_website_events
    FOR SELECT TO site_builder USING (true);
CREATE POLICY staff_site_builder ON staff
    FOR SELECT TO site_builder USING (true);
CREATE POLICY practice_memberships_site_builder ON practice_memberships
    FOR SELECT TO site_builder USING (true);

-- +goose Down
DROP POLICY practice_memberships_site_builder ON practice_memberships;
DROP POLICY staff_site_builder ON staff;
DROP POLICY practice_website_events_site_builder ON practice_website_events;
DROP POLICY practices_site_builder ON practices;
DROP POLICY practice_websites_site_builder ON practice_websites;
REVOKE SELECT ON practice_websites, practices, practice_website_events, staff, practice_memberships
    FROM site_builder;
ALTER TABLE practice_websites DROP CONSTRAINT practice_websites_hosted_slug_present;
DROP INDEX practice_websites_slug_key;
ALTER TABLE practice_websites DROP COLUMN slug;
