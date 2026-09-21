-- +goose Up
-- #1093: who is on call tonight, and is anyone covering two births at
-- once. Four additions, and none of them is a new reach mechanism -- all
-- reach still follows the Attachment (ADR-0006, ADR-0008).
--
-- The on-call window itself is NOT stored. It is derived on every read
-- from the Engagement's due date, its pregnancy_ended_on and the rule
-- below, the same way a Visit's type is derived (CONTEXT.md, Visit):
-- correcting a due date must move the window at once, with no backfill.
-- What is stored is only what a person states: the rule, a narrowing,
-- and a coverage gap.

-- ---------------------------------------------------------------------
-- The start rule. A gestational week (the common case, 37w0d) or the
-- day the Attachment was granted ("on call from time of hire"). Chosen
-- once per Practice, overridable per Engagement.
-- ---------------------------------------------------------------------
CREATE TYPE on_call_start_rule AS ENUM ('gestational_week', 'attachment_granted');

-- The DEFAULTs stay, unlike guardrail_test.go's usual DEFAULT-then-DROP
-- form, for 00110's reason: nothing on the signup path asks for an
-- on-call rule, so every INSERT into practices would fail without them.
-- 37 weeks and 14 days are the ticket's own stated defaults, not a
-- choice this migration makes for a Practice.
ALTER TABLE practices ADD COLUMN on_call_start_rule on_call_start_rule NOT NULL DEFAULT 'gestational_week';
ALTER TABLE practices ADD COLUMN on_call_start_week smallint NOT NULL DEFAULT 37;
ALTER TABLE practices ADD COLUMN on_call_grace_days smallint NOT NULL DEFAULT 14;

-- Bounds wide enough for every real practice and narrow enough that a
-- typo cannot put a window a year long on the roster. 20 weeks is the
-- earliest a gestational rule is meaningful; 42 is post-dates.
ALTER TABLE practices ADD CONSTRAINT practices_on_call_start_week_range
    CHECK (on_call_start_week BETWEEN 20 AND 42);
ALTER TABLE practices ADD CONSTRAINT practices_on_call_grace_days_range
    CHECK (on_call_grace_days BETWEEN 0 AND 42);

-- The per-Engagement override. Both NULL means "the Practice's rule".
-- A week is carried only with the gestational rule, so the row can never
-- say "from time of hire, at week 38".
ALTER TABLE engagements ADD COLUMN on_call_start_rule on_call_start_rule;
ALTER TABLE engagements ADD COLUMN on_call_start_week smallint;
ALTER TABLE engagements ADD CONSTRAINT engagements_on_call_override_shape CHECK (
    (on_call_start_rule IS NULL AND on_call_start_week IS NULL)
    OR (on_call_start_rule = 'attachment_granted' AND on_call_start_week IS NULL)
    OR (on_call_start_rule = 'gestational_week' AND on_call_start_week BETWEEN 20 AND 42)
);

-- ---------------------------------------------------------------------
-- Per-provider narrowing. Where two Doulas hold granted Attachments on
-- one Engagement, each may be on call for only part of the window -- a
-- primary and a backup, with no second entity. On the Attachment row
-- because it is a fact about that one Doula on that one Engagement, and
-- ends when the Attachment ends. Calendar days in the Practice's zone,
-- inclusive, like the window it narrows. Either bound may be open:
-- "from week 39 on" is a narrowing with no end.
-- ---------------------------------------------------------------------
ALTER TABLE engagement_attachments ADD COLUMN on_call_from date;
ALTER TABLE engagement_attachments ADD COLUMN on_call_to date;
ALTER TABLE engagement_attachments ADD CONSTRAINT engagement_attachments_on_call_order
    CHECK (on_call_from IS NULL OR on_call_to IS NULL OR on_call_from <= on_call_to);

-- ---------------------------------------------------------------------
-- engagement_coverage_gaps: a stated interval within a window during
-- which one attached Doula is not reachable. Instants, not days: "I want
-- a few drinks on Saturday night" is a gap of hours. Half-open
-- [starts_at, ends_at).
--
-- covering_staff_id is optional. A gap with nobody covering it is a
-- real record -- it is the Practice's signal that cover is still needed
-- -- and the roster shows it as a hole. Both people must hold an open,
-- granted Attachment on the Engagement; that is checked by the handler,
-- because a gap must never be a back door to attaching someone.
--
-- Cleared, never deleted: "she was covered that night" stays answerable.
-- ---------------------------------------------------------------------
CREATE TABLE engagement_coverage_gaps (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    engagement_id     uuid NOT NULL REFERENCES engagements (id),
    staff_id          uuid NOT NULL REFERENCES staff (id),
    covering_staff_id uuid REFERENCES staff (id),
    starts_at         timestamptz NOT NULL,
    ends_at           timestamptz NOT NULL,
    reason            text,
    created_by        uuid NOT NULL REFERENCES staff (id),
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_by        uuid REFERENCES staff (id),
    updated_at        timestamptz,
    cleared_by        uuid REFERENCES staff (id),
    cleared_at        timestamptz,
    CONSTRAINT engagement_coverage_gaps_order CHECK (starts_at < ends_at),
    CONSTRAINT engagement_coverage_gaps_cover_is_someone_else
        CHECK (covering_staff_id IS NULL OR covering_staff_id <> staff_id),
    CONSTRAINT engagement_coverage_gaps_cleared_pair
        CHECK ((cleared_at IS NULL) = (cleared_by IS NULL)),
    CONSTRAINT engagement_coverage_gaps_reason_length
        CHECK (reason IS NULL OR char_length(reason) <= 500)
);

-- The roster's read: every live gap on a set of Engagements, overlapping
-- a range. Partial on cleared_at because a cleared gap is never on the
-- roster.
CREATE INDEX engagement_coverage_gaps_live
    ON engagement_coverage_gaps (engagement_id, starts_at)
    WHERE cleared_at IS NULL;

-- No DELETE: a gap is cleared, never erased.
GRANT SELECT, INSERT, UPDATE ON engagement_coverage_gaps TO app_runtime;

ALTER TABLE engagement_coverage_gaps ENABLE ROW LEVEL SECURITY;

-- No practice_id column, so the Practice tier is the EXISTS against
-- engagements that engagement_attachments_practice_visibility (00030)
-- already uses.
CREATE POLICY engagement_coverage_gaps_practice_visibility ON engagement_coverage_gaps
    USING (
        EXISTS (
            SELECT 1 FROM engagements e
            WHERE e.id = engagement_coverage_gaps.engagement_id
              AND e.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        )
    );

-- The outbox worker's send-time recheck reads the gap with no Practice
-- set, so it needs 00033's trusted-worker door on this table too.
-- Without it every recheck would read "gap gone" and mark every row
-- sent with nothing mailed.
CREATE POLICY engagement_coverage_gaps_notification_worker ON engagement_coverage_gaps
    FOR SELECT
    USING (current_setting('app.notification_worker_trusted', true) = 'true');

-- ---------------------------------------------------------------------
-- coverage_gap_outbox: the content-free Platform Notification (ADR-0009,
-- ADR-0011) to every current Owner and Admin that a gap has nobody
-- covering it. Shaped after connect_nudge_outbox (00109): a person's act
-- queues it and nothing fires on a clock (ADR-0038); recipients are
-- resolved at send time and recorded afterward, never chosen at queue
-- time.
-- ---------------------------------------------------------------------
CREATE TYPE coverage_gap_outbox_status AS ENUM ('pending', 'sent', 'dead_lettered');

CREATE TABLE coverage_gap_outbox (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    practice_id           uuid NOT NULL REFERENCES practices (id),
    gap_id                uuid NOT NULL REFERENCES engagement_coverage_gaps (id),
    requested_by_staff_id uuid NOT NULL REFERENCES staff (id),
    status                coverage_gap_outbox_status NOT NULL DEFAULT 'pending',
    attempt_count         int NOT NULL DEFAULT 0,
    next_attempt_at       timestamptz NOT NULL DEFAULT now(),
    created_at            timestamptz NOT NULL DEFAULT now(),
    sent_at               timestamptz,
    last_error            text,
    -- Whom it went to, written by the worker at send time, for 00109's
    -- reason: the roster moves, and "every Owner and Admin at the time"
    -- is otherwise unanswerable a month later.
    notified_staff_ids    uuid[]
);

-- At most one pending row per gap. Two saves of one uncovered gap in a
-- minute are one hole, and one email says so.
CREATE UNIQUE INDEX coverage_gap_outbox_one_pending
    ON coverage_gap_outbox (gap_id)
    WHERE status = 'pending';

GRANT SELECT, INSERT, UPDATE ON coverage_gap_outbox TO app_runtime;

-- No RLS on the outbox itself, for 00034's reason: it is platform-level
-- like payout_outbox and connect_nudge_outbox, and the worker reuses
-- 00033's trusted policies on staff/practice_memberships.

-- +goose Down
DROP TABLE coverage_gap_outbox;
DROP TYPE coverage_gap_outbox_status;
DROP TABLE engagement_coverage_gaps;
ALTER TABLE engagement_attachments DROP CONSTRAINT engagement_attachments_on_call_order;
ALTER TABLE engagement_attachments DROP COLUMN on_call_to;
ALTER TABLE engagement_attachments DROP COLUMN on_call_from;
ALTER TABLE engagements DROP CONSTRAINT engagements_on_call_override_shape;
ALTER TABLE engagements DROP COLUMN on_call_start_week;
ALTER TABLE engagements DROP COLUMN on_call_start_rule;
ALTER TABLE practices DROP CONSTRAINT practices_on_call_grace_days_range;
ALTER TABLE practices DROP CONSTRAINT practices_on_call_start_week_range;
ALTER TABLE practices DROP COLUMN on_call_grace_days;
ALTER TABLE practices DROP COLUMN on_call_start_week;
ALTER TABLE practices DROP COLUMN on_call_start_rule;
DROP TYPE on_call_start_rule;
