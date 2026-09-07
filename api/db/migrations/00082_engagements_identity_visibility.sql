-- +goose Up
-- #312, ADR-0015 ("The portal root list has no :engagementId, so it gets
-- one new read-only identity-tier policy on engagements"): the portal
-- root lists a signed-in person's Engagements across every Practice her
-- Portal Account reaches, before any single Engagement has been chosen
-- and so before app.current_client_id is ever set. Without this,
-- engagements_practice_visibility (00005, Staff) and
-- engagements_client_visibility (00006, one addressed Engagement) are
-- the only two permissive policies on this table, and neither matches --
-- the one cross-Client read this product has would have no
-- database-level guard at all, only the API handler's own read.
--
-- Same guard shape as client_portal_users_self_visibility (00006): both
-- other tiers' session variables unset, then a match on the caller's own
-- identity. No join to portal_accounts is needed the way ADR-0015's own
-- SQL sketch has one -- client_portal_users.identity_uid already *is*
-- the portal_accounts.identifier it names (00073), so matching it
-- directly against app.current_identity_uid is the whole join.
CREATE POLICY engagements_identity_visibility ON engagements
    FOR SELECT
    USING (
        NULLIF(current_setting('app.current_practice_id', true), '') IS NULL
        AND NULLIF(current_setting('app.current_client_id', true), '') IS NULL
        AND EXISTS (
            SELECT 1 FROM client_portal_users cpu
            WHERE cpu.client_id = engagements.client_id
              AND cpu.identity_uid = NULLIF(current_setting('app.current_identity_uid', true), '')
        )
    );

-- +goose Down
DROP POLICY engagements_identity_visibility ON engagements;
