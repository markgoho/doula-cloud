-- +goose Up
-- Widens 00062's actor-shape CHECK to admit the two reasons 00071 added
-- to staff_auth_event_reason -- split into its own migration because
-- Postgres forbids using an ALTER TYPE ... ADD VALUE in the same
-- transaction that added it (see 00071's comment).
ALTER TABLE staff_auth_events DROP CONSTRAINT staff_auth_events_actor_shape;
ALTER TABLE staff_auth_events ADD CONSTRAINT staff_auth_events_actor_shape CHECK (
    (reason IN ('owner_vouched', 'self_service', 'enrolled', 'removed') AND actor_staff_id IS NOT NULL AND actor_operator IS NULL)
    OR (reason = 'support' AND actor_operator IS NOT NULL AND actor_operator <> '' AND actor_staff_id IS NULL)
);

-- +goose Down
ALTER TABLE staff_auth_events DROP CONSTRAINT staff_auth_events_actor_shape;
ALTER TABLE staff_auth_events ADD CONSTRAINT staff_auth_events_actor_shape CHECK (
    (reason IN ('owner_vouched', 'self_service') AND actor_staff_id IS NOT NULL AND actor_operator IS NULL)
    OR (reason = 'support' AND actor_operator IS NOT NULL AND actor_operator <> '' AND actor_staff_id IS NULL)
);
