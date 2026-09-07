-- +goose Up
-- #253: the write side of ADR-0015's six-move status table. This
-- migration builds the half that does not depend on the birth outcome --
-- #293 owns that fact and is not yet built, so the `CHECK` here demands
-- only `ending_reason`, and #293's own migration tightens it to also
-- demand `birth_outcome` once that column exists.

-- ---------------------------------------------------------------------
-- The ending reason: staff-only, required at 'completed', cleared on
-- reopen. `ending_note` sits beside it, always optional.
-- ---------------------------------------------------------------------
CREATE TYPE engagement_ending_reason AS ENUM (
    'care_complete', 'client_withdrew', 'practice_ended',
    'transferred', 'no_response', 'entered_in_error'
);

ALTER TABLE engagements ADD COLUMN ending_reason engagement_ending_reason;
ALTER TABLE engagements ADD COLUMN ending_note text;

ALTER TABLE engagements ADD CONSTRAINT engagements_completed_has_reason CHECK (
    status <> 'completed' OR ending_reason IS NOT NULL
);

-- ---------------------------------------------------------------------
-- engagement_events: ADR-0015's audit table, shaped on
-- practice_membership_events (00039). One row per change to a mutable
-- Engagement fact, both sides recorded. event_type carries all four
-- values ADR-0015 names even though this ticket only ever writes
-- 'status_changed' -- #293 (birth outcome) and any future kind-change
-- writer share this table rather than each growing their own, and the
-- birth_outcome columns are deliberately absent here: that enum does not
-- exist yet, and #293 ALTERs this table to add them alongside its own
-- writer.
-- ---------------------------------------------------------------------
CREATE TYPE engagement_event_type AS ENUM
    ('status_changed', 'kind_changed', 'birth_outcome_recorded', 'ending_changed');

CREATE TABLE engagement_events (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    practice_id           uuid NOT NULL REFERENCES practices (id),
    engagement_id         uuid NOT NULL REFERENCES engagements (id),
    event_type            engagement_event_type NOT NULL,
    previous_status       engagement_status,
    status                engagement_status,
    previous_kind         engagement_kind,
    kind                  engagement_kind,
    previous_ending_reason engagement_ending_reason,
    ending_reason         engagement_ending_reason,
    previous_ending_note  text,
    ending_note           text,
    actor_staff_id        uuid REFERENCES staff (id),   -- nullable: an
        -- automation may record no human actor, the same shape
        -- engagement_offers.decided_by uses on the completion cascade
    created_at            timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX engagement_events_engagement
    ON engagement_events (practice_id, engagement_id, created_at);

GRANT SELECT, INSERT ON engagement_events TO app_runtime;   -- no UPDATE, no DELETE: an event is a fact about the past

ALTER TABLE engagement_events ENABLE ROW LEVEL SECURITY;

-- Practice-tier only, the same shape
-- practice_membership_events_practice_visibility (00039) uses. No
-- client-tier policy exists or is added: the audit trail is the
-- Practice's own record of its acts, and a portal session never sets
-- app.current_practice_id (00006), so it fails closed here by
-- construction rather than by an extra check.
CREATE POLICY engagement_events_practice_visibility ON engagement_events
    USING (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid);

-- +goose Down
DROP TABLE engagement_events;
DROP TYPE engagement_event_type;

ALTER TABLE engagements DROP CONSTRAINT engagements_completed_has_reason;
ALTER TABLE engagements DROP COLUMN ending_note;
ALTER TABLE engagements DROP COLUMN ending_reason;
DROP TYPE engagement_ending_reason;
