-- +goose Up
-- #301's acknowledge action needs a narrow write for the Client-portal
-- role: only client_acknowledged_at on her own Birth Plan row, nothing
-- else. A plain RLS UPDATE policy can't express that -- USING/WITH CHECK
-- scope which *rows* a command may touch, not which *columns* change
-- within them, so a Client-tier UPDATE policy permissive enough to admit
-- this write would just as happily admit one that overwrites `answers`
-- (Staff's own drafted content) or `fields`. This is the same "no
-- session context an RLS policy could use" case
-- portal_account_reuse_for_accept (00081) and
-- push_subscriptions_for_message_recipient (00067) already solve with a
-- SECURITY DEFINER function that checks ownership itself and performs
-- exactly one column's write -- never generalizing into a client-tier
-- grant on the table.
--
-- No RLS policy is added for this: the Client-portal role keeps its
-- existing SELECT-only reach into plan_instances
-- (plan_instances_client_birth_plan_visibility, 00013), and
-- TestRLS_PlanInstancesClientUpdateRejected (client_rls_test.go) keeps
-- proving a raw UPDATE from that role still affects zero rows.
-- +goose StatementBegin
CREATE FUNCTION acknowledge_birth_plan(p_engagement_id uuid)
RETURNS timestamptz
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
DECLARE
    v_acknowledged_at timestamptz;
BEGIN
    UPDATE plan_instances pi
    SET client_acknowledged_at = now()
    FROM engagements e
    WHERE pi.engagement_id = e.id
      AND pi.engagement_id = p_engagement_id
      AND pi.plan_type = 'birth_plan'
      AND e.client_id = NULLIF(current_setting('app.current_client_id', true), '')::uuid
    RETURNING pi.client_acknowledged_at INTO v_acknowledged_at;

    RETURN v_acknowledged_at;
END;
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION acknowledge_birth_plan(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION acknowledge_birth_plan(uuid) TO app_runtime;

-- +goose Down
DROP FUNCTION acknowledge_birth_plan(uuid);
