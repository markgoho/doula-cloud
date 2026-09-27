-- +goose Up
-- #1423: engagement_events folds into activity, the one ledger ADR-0022
-- names, and is dropped.
--
-- 00090 built engagement_events nine days after ADR-0022 rejected a
-- third event table, for one reason its own comment gives: it had no
-- client-tier policy, so a portal session could never read it. Three
-- writers followed it (status_changed, birth_outcome_recorded,
-- kind_changed) and nothing ever read it, so "how did this Engagement's
-- status, kind or birth outcome come to be?" had no answer in the
-- product.
--
-- activity already holds that reason without a second table. The
-- portal's one reader (portal.ActivityHandler, via
-- activityfeed.ListForSubject) selects no diff column at all, and
-- activity.StaffingActions() names the actions it excludes outright;
-- engagement_reopened, kind_changed and birth_outcome_recorded join that
-- list. The status writers record the both-sides diff on the actions
-- they already wrote (care_phase_changed, engagement_completed), so an
-- activation or a completion stays one row on the ledger, not two.
--
-- Nothing is copied across. Doula Cloud has no production data
-- (CLAUDE.md), and a row written before this migration in a developer's
-- database is test data.
DROP TABLE engagement_events;
DROP TYPE engagement_event_type;

-- +goose Down
-- The shape 00090 and 00093 left, without the rows: this Down restores
-- the table so an older binary can write to it, not the history the Up
-- discarded.
CREATE TYPE engagement_event_type AS ENUM
    ('status_changed', 'kind_changed', 'birth_outcome_recorded', 'ending_changed');

CREATE TABLE engagement_events (
    id                          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    practice_id                 uuid NOT NULL REFERENCES practices (id),
    engagement_id               uuid NOT NULL REFERENCES engagements (id),
    event_type                  engagement_event_type NOT NULL,
    previous_status             engagement_status,
    status                      engagement_status,
    previous_kind               engagement_kind,
    kind                        engagement_kind,
    previous_ending_reason      engagement_ending_reason,
    ending_reason               engagement_ending_reason,
    previous_ending_note        text,
    ending_note                 text,
    actor_staff_id              uuid REFERENCES staff (id),
    created_at                  timestamptz NOT NULL DEFAULT now(),
    previous_birth_outcome      birth_outcome,
    birth_outcome               birth_outcome,
    previous_pregnancy_ended_on date,
    pregnancy_ended_on          date
);

CREATE INDEX engagement_events_engagement
    ON engagement_events (practice_id, engagement_id, created_at);

GRANT SELECT, INSERT ON engagement_events TO app_runtime;

ALTER TABLE engagement_events ENABLE ROW LEVEL SECURITY;

CREATE POLICY engagement_events_practice_visibility ON engagement_events
    USING (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid);
