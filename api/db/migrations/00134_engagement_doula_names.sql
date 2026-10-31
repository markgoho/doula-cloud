-- +goose Up
-- #2355: the Birth Plan masthead names the Engagement's Doula, on the
-- portal as on the Practice side, from the one reader
-- plans.loadMasthead runs on either tier's transaction.
--
-- The gap. engagement_attachments carries one policy (00030), keyed on
-- app.current_practice_id, which a Client-portal transaction never sets;
-- a plain join there reads zero rows and the Doula row silently vanishes
-- from her copy. Adding a client-tier policy to the table would put every
-- column of the row in reach of her transaction, fee_amount_cents
-- included, and 00111 records what an inline EXISTS policy costs every
-- query that reads the table. Setting app.current_practice_id on her
-- transaction, as portal/activity.go does for its last read, would widen
-- every practice-tier policy for whatever runs after it.
--
-- So one SECURITY DEFINER function returns the names only, the same
-- narrow read path 00010 adds for push subscriptions. Its own WHERE is
-- the boundary: the Engagement must be the current Client's own, or at
-- the current Staff session's Practice -- the same Engagement either
-- tier's middleware has already confirmed the caller reaches. The set is
-- engagement.readAttachedDoulas's, in its order: every open, granted
-- Attachment (GLOSSARY.md's Attachment, "who is working this birth").
-- A Doula who deleted her own login is left out rather than named by the
-- string ADR-0033 overwrites her name with.
-- +goose StatementBegin
CREATE FUNCTION engagement_doula_names(target_engagement_id uuid)
RETURNS TABLE (name text)
LANGUAGE sql SECURITY DEFINER STABLE
SET search_path = public, pg_temp
AS $$
    SELECT s.name
    FROM engagements e
    JOIN engagement_attachments ea ON ea.engagement_id = e.id
    JOIN staff s ON s.id = ea.staff_id
    WHERE e.id = target_engagement_id
      AND ea.origin = 'granted'
      AND ea.ended_at IS NULL
      AND s.deleted_at IS NULL
      AND (
          e.client_id = NULLIF(current_setting('app.current_client_id', true), '')::uuid
          OR e.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
      )
    ORDER BY ea.attached_at, ea.staff_id
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION engagement_doula_names(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION engagement_doula_names(uuid) TO app_runtime;

-- +goose Down
DROP FUNCTION engagement_doula_names(uuid);
