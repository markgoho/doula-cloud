-- +goose Up
-- #303: a durable Client push-notification preference, so the Portal
-- Account can review, mute and re-enable push without signing out, and so
-- the Message push path (api/internal/message/push.go) can refuse to send
-- where muted.
--
-- Keyed on identity_uid -- ADR-0015's "the person lives in the login",
-- CONTEXT.md's Portal Account, not the Client row -- carrying an
-- engagement_id so muting one Engagement's thread never touches another,
-- including one at a different Practice. No FK from identity_uid to
-- client_portal_users(identity_uid): 00064_client_erasure.sql clears that
-- column to NULL on the Client's own row once she is erased, which would
-- violate a REFERENCES constraint the moment an erased Client's preference
-- row still named her old identity. push_subscriptions.owner_id
-- (00008_messaging.sql) already carries no FK for a different reason
-- (polymorphic owner); this is the same absence for an erasure-safety
-- reason instead. Left untouched by erasure, same as push_subscriptions --
-- neither table holds anything erasure's redaction is about.
--
-- channel is an enum of one value today. #347 (email preferences) is out
-- of scope here, but adding 'email' later is `ALTER TYPE ... ADD VALUE`, a
-- metadata-only change -- no migration ever has to rewrite an existing
-- push row to make room for it.
CREATE TYPE notification_channel AS ENUM ('push');

CREATE TABLE notification_preferences (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    identity_uid text NOT NULL,
    engagement_id uuid NOT NULL REFERENCES engagements (id),
    channel notification_channel NOT NULL DEFAULT 'push',
    muted boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (identity_uid, engagement_id, channel)
);

GRANT SELECT, INSERT, UPDATE ON notification_preferences TO app_runtime;

ALTER TABLE notification_preferences ENABLE ROW LEVEL SECURITY;

-- Mirrors messages_client_visibility's shape (00008_messaging.sql): the
-- table carries no client_id column, so ownership is an EXISTS subquery
-- through engagements. identity_uid is checked directly too, belt and
-- suspenders -- a preference names the Portal Account, not only the
-- Engagement, so both facts are worth asserting even though today one
-- Client has exactly one identity_uid.
-- +goose StatementBegin
CREATE POLICY notification_preferences_client_visibility ON notification_preferences
    USING (
        identity_uid = NULLIF(current_setting('app.current_identity_uid', true), '')
        AND EXISTS (
            SELECT 1 FROM engagements e
            WHERE e.id = notification_preferences.engagement_id
              AND e.client_id = NULLIF(current_setting('app.current_client_id', true), '')::uuid
        )
    );
-- +goose StatementEnd

-- push_subscriptions_for_message_recipient (00010) gets one more clause:
-- the Client branch now refuses to return a subscription when the
-- recipient's Engagement is muted for push. Permissive by default -- "no
-- preference row" still means push sends, exactly as it always has -- a
-- push_subscriptions row only ever exists because the Client's own device
-- called the register endpoint, which is itself the consent, so this
-- clause only ever narrows that down further, on an explicit mute. The
-- Staff branch is unchanged: this ticket is Client-portal only.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION push_subscriptions_for_message_recipient(p_engagement_id uuid, p_recipient_type actor_type)
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
      AND NOT EXISTS (
          SELECT 1 FROM engagements e
          JOIN client_portal_users cpu ON cpu.client_id = e.client_id
          JOIN notification_preferences np
            ON np.identity_uid = cpu.identity_uid
           AND np.engagement_id = e.id
           AND np.channel = 'push'
           AND np.muted = true
          WHERE e.id = p_engagement_id
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

REVOKE EXECUTE ON FUNCTION push_subscriptions_for_message_recipient(uuid, actor_type) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION push_subscriptions_for_message_recipient(uuid, actor_type) TO app_runtime;

-- +goose Down
-- Restores 00010's original function body first, so it stops referencing
-- notification_preferences before that table is dropped.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION push_subscriptions_for_message_recipient(p_engagement_id uuid, p_recipient_type actor_type)
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

REVOKE EXECUTE ON FUNCTION push_subscriptions_for_message_recipient(uuid, actor_type) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION push_subscriptions_for_message_recipient(uuid, actor_type) TO app_runtime;

DROP TABLE notification_preferences;
DROP TYPE notification_channel;
