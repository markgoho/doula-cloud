-- +goose Up
-- #2958: a Contract records who made it, as it already records who sent and
-- who voided it, so a Draft's row on the list can say who wrote it.
-- Nullable: a backfilled row whose Activity is gone has no one to name.

ALTER TABLE contracts ADD COLUMN created_by uuid REFERENCES staff (id);

WITH windows AS (
    SELECT c.id, c.engagement_id, c.created_at AS lo,
           COALESCE(LEAD(c.created_at) OVER (PARTITION BY c.engagement_id ORDER BY c.created_at, c.id), 'infinity'::timestamptz) AS hi
      FROM contracts c
),
makes AS (
    SELECT w.id, a.actor_staff_id
      FROM windows w
      JOIN LATERAL (
          SELECT act.actor_staff_id
            FROM activity act
           WHERE act.subject_kind = 'engagement' AND act.subject_id = w.engagement_id
             AND act.action = 'contract_created'
             AND act.created_at >= w.lo AND act.created_at < w.hi
           ORDER BY act.created_at, act.id
           LIMIT 1
      ) a ON true
)
UPDATE contracts c
   SET created_by = m.actor_staff_id
  FROM makes m
 WHERE c.id = m.id;

-- +goose Down
ALTER TABLE contracts DROP COLUMN created_by;
