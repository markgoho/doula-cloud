-- +goose Up
-- #2749: the Contracts list sorts and dates every Contract by the day of its
-- current step, so the day it was sent and the day it was voided are columns
-- of the Contract, with who did each. Until now both lived only as Activity
-- rows on the Engagement, which a list cannot sort by or join cheaply.
--
-- stepped_at is the day of the Contract's current step, stored and indexed,
-- so every date order of the list reads one column.
-- No index: under row security the planner sorts the Practice's Contracts
-- instead (12-24 ms at 5,000), so an index on it would never be read.

ALTER TABLE contracts
    ADD COLUMN sent_at timestamptz,
    ADD COLUMN sent_by uuid REFERENCES staff (id),
    ADD COLUMN voided_at timestamptz,
    ADD COLUMN voided_by uuid REFERENCES staff (id);

-- Activity's subject is the Engagement, and an Engagement can hold a voided
-- Contract and its replacement, so each Contract owns a window: from its own
-- created_at to the created_at of the next Contract on the same Engagement.
-- A Contract is sent inside its window; it is voided after it was signed, or
-- made, and before the next one exists.
WITH windows AS (
    SELECT c.id, c.engagement_id, c.created_at AS lo,
           COALESCE(LEAD(c.created_at) OVER (PARTITION BY c.engagement_id ORDER BY c.created_at, c.id), 'infinity'::timestamptz) AS hi
      FROM contracts c
),
sends AS (
    SELECT w.id, a.created_at, a.actor_staff_id
      FROM windows w
      JOIN LATERAL (
          SELECT act.created_at, act.actor_staff_id
            FROM activity act
           WHERE act.subject_kind = 'engagement' AND act.subject_id = w.engagement_id
             AND act.action = 'contract_sent'
             AND act.created_at >= w.lo AND act.created_at < w.hi
           ORDER BY act.created_at, act.id
           LIMIT 1
      ) a ON true
)
UPDATE contracts c
   SET sent_at = COALESCE(s.created_at, c.signed_at, c.created_at),
       sent_by = s.actor_staff_id
  FROM contracts cc
  LEFT JOIN sends s ON s.id = cc.id
 WHERE c.id = cc.id AND c.status <> 'draft';

WITH windows AS (
    SELECT c.id, c.engagement_id, COALESCE(c.signed_at, c.created_at) AS lo,
           COALESCE(LEAD(c.created_at) OVER (PARTITION BY c.engagement_id ORDER BY c.created_at, c.id), 'infinity'::timestamptz) AS hi
      FROM contracts c
),
voids AS (
    SELECT w.id, a.created_at, a.actor_staff_id
      FROM windows w
      JOIN LATERAL (
          SELECT act.created_at, act.actor_staff_id
            FROM activity act
           WHERE act.subject_kind = 'engagement' AND act.subject_id = w.engagement_id
             AND act.action = 'contract_voided'
             AND act.created_at >= w.lo AND act.created_at < w.hi
           ORDER BY act.created_at, act.id
           LIMIT 1
      ) a ON true
)
UPDATE contracts c
   SET voided_at = COALESCE(v.created_at, c.signed_at, c.created_at),
       voided_by = v.actor_staff_id
  FROM contracts cc
  LEFT JOIN voids v ON v.id = cc.id
 WHERE c.id = cc.id AND c.status = 'voided';

-- A signed row that never kept its signed_at gets the instant it was sent, so
-- stepped_at below is never NULL for a signed Contract.
UPDATE contracts
   SET signed_at = COALESCE(sent_at, created_at)
 WHERE status = 'signed' AND signed_at IS NULL;

-- sent_by and voided_by stay nullable: a backfilled row whose Activity is
-- gone has no one to name.
ALTER TABLE contracts ADD CONSTRAINT contracts_sent_at_matches_status CHECK (
    (status = 'draft') = (sent_at IS NULL)
);

ALTER TABLE contracts ADD CONSTRAINT contracts_voided_at_matches_status CHECK (
    (status = 'voided') = (voided_at IS NOT NULL)
);

ALTER TABLE contracts ADD CONSTRAINT contracts_signed_at_matches_status CHECK (
    status <> 'signed' OR signed_at IS NOT NULL
);

ALTER TABLE contracts ADD COLUMN stepped_at timestamptz GENERATED ALWAYS AS (
    CASE status
        WHEN 'draft' THEN created_at
        WHEN 'sent' THEN sent_at
        WHEN 'signed' THEN signed_at
        ELSE voided_at
    END
) STORED;

-- +goose Down
ALTER TABLE contracts DROP COLUMN stepped_at;
ALTER TABLE contracts DROP CONSTRAINT contracts_signed_at_matches_status;
ALTER TABLE contracts DROP CONSTRAINT contracts_voided_at_matches_status;
ALTER TABLE contracts DROP CONSTRAINT contracts_sent_at_matches_status;
ALTER TABLE contracts
    DROP COLUMN voided_by,
    DROP COLUMN voided_at,
    DROP COLUMN sent_by,
    DROP COLUMN sent_at;
