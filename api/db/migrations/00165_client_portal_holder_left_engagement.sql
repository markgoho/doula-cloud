-- +goose Up
-- #2596: the Client portal leaves a Doula's name off a booked Visit when
-- that Doula has been taken off the Engagement (a Staff view flags the
-- same Visit; see visit/off_engagement.go). The portal's transaction sees
-- no engagement_attachments row, and an inline policy would be expanded
-- into every query that reads the table (00111 says why that costs), so
-- the question is a SECURITY DEFINER function, as 00111's is.
--
-- It answers one boolean about one Doula on one Engagement, and only for
-- an Engagement of the Client whose id clientauth.Middleware set in
-- app.current_client_id; for any other caller it is false. She is "gone"
-- when she has no open Attachment and a granted one has ended.
-- +goose StatementBegin
CREATE FUNCTION client_portal_holder_left_engagement(target_engagement uuid, target_staff uuid) RETURNS boolean
    LANGUAGE sql SECURITY DEFINER STABLE
    SET search_path = public, pg_temp
    AS $$
        SELECT EXISTS (
            SELECT 1 FROM engagements e
            WHERE e.id = target_engagement AND e.status <> 'completed'
              AND e.client_id = NULLIF(current_setting('app.current_client_id', true), '')::uuid
        )
        AND NOT EXISTS (
            SELECT 1 FROM engagement_attachments ea
            WHERE ea.engagement_id = target_engagement AND ea.staff_id = target_staff AND ea.ended_at IS NULL
        )
        AND EXISTS (
            SELECT 1 FROM engagement_attachments ea
            WHERE ea.engagement_id = target_engagement AND ea.staff_id = target_staff
              AND ea.origin = 'granted' AND ea.ended_at IS NOT NULL
        )
    $$;
-- +goose StatementEnd

-- A handler calls this with arguments of its choosing, so the grant is part
-- of the boundary (00010's reason): only app_runtime executes it.
REVOKE EXECUTE ON FUNCTION client_portal_holder_left_engagement(uuid, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION client_portal_holder_left_engagement(uuid, uuid) TO app_runtime;

-- +goose Down
DROP FUNCTION client_portal_holder_left_engagement(uuid, uuid);
