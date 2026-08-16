-- +goose Up
-- #61 (Push notifications on new message) needs to resolve the *other*
-- party's push_subscriptions row when a Message is created (Staff sends,
-- notify the Client; Client sends, notify the Staff at that Practice).
-- push_subscriptions' RLS (00008_messaging.sql) is deliberately
-- self-identity-only -- #57's AC says "never cross-identity readable",
-- and api/internal/message/rls_test.go's
-- TestRLS_PushSubscriptionsStaffRowHiddenDuringClientPortalContext proves
-- it even for a shared-identity Staff+Client person -- so widening that
-- table's own policies is off the table.
--
-- Instead, a SECURITY DEFINER function runs as this migration's owner
-- (the same role that owns the table, so it bypasses push_subscriptions'
-- RLS the same way an ordinary superuser/table-owner query would --
-- Postgres's own design, per 00002_practice_staff_tenancy.sql's RLS
-- comment) rather than as the calling app_runtime role. This keeps the
-- table's own RLS surface exactly as #57 shipped it and adds one narrow,
-- named read path instead: always scoped through an engagement_id, never
-- a bare owner id, so a caller can't use it to fish for a specific
-- identity's subscriptions. Safe to call only because engagementID's
-- ownership by the caller (Staff at its Practice, or the Client it
-- belongs to) is already established by staffauth/clientauth middleware
-- before message.notifyRecipient ever calls this.
-- +goose StatementBegin
CREATE FUNCTION push_subscriptions_for_message_recipient(p_engagement_id uuid, p_recipient_type actor_type)
RETURNS TABLE (endpoint text, p256dh_key text, auth_key text)
LANGUAGE sql
SECURITY DEFINER
SET search_path = public
AS $$
    SELECT ps.endpoint, ps.p256dh_key, ps.auth_key
    FROM push_subscriptions ps
    WHERE p_recipient_type = 'client'
      AND ps.owner_type = 'client'
      AND EXISTS (
          SELECT 1 FROM engagements e
          WHERE e.id = p_engagement_id AND e.client_id = ps.owner_id
      )
    UNION ALL
    SELECT ps.endpoint, ps.p256dh_key, ps.auth_key
    FROM push_subscriptions ps
    WHERE p_recipient_type = 'staff'
      AND ps.owner_type = 'staff'
      AND EXISTS (
          SELECT 1 FROM engagements e
          JOIN practice_memberships pm ON pm.practice_id = e.practice_id
          WHERE e.id = p_engagement_id AND pm.staff_id = ps.owner_id
      )
$$;
-- +goose StatementEnd

GRANT EXECUTE ON FUNCTION push_subscriptions_for_message_recipient(uuid, actor_type) TO app_runtime;

-- +goose Down
DROP FUNCTION push_subscriptions_for_message_recipient(uuid, actor_type);
