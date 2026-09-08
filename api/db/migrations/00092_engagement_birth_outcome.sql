-- +goose Up
-- #293: ADR-0015's birth outcome -- the second of the three facts an
-- Engagement carries about how it is going. Staff-only, never
-- Client-facing, recorded whenever the fact becomes known rather than
-- only at completion, and frozen once recorded except through the
-- Owner-only correction door below.
--
-- What this migration deliberately does NOT do: tighten
-- engagements_completed_has_reason (00090) into ADR-0015's
-- engagements_completed_is_explained, which also demands a birth
-- outcome at 'completed'. 00090's own comment anticipated this ticket
-- doing it, but demanding the outcome at completion is a change to the
-- status-transition endpoint's request contract (it would have to
-- collect an outcome on the way to 'completed'), not a one-line
-- constraint -- so it is filed as its own ticket rather than smuggled
-- in here.

CREATE TYPE birth_outcome AS ENUM ('live_birth', 'loss', 'unknown');

ALTER TABLE engagements ADD COLUMN birth_outcome birth_outcome;      -- nullable until known
ALTER TABLE engagements ADD COLUMN pregnancy_ended_on date;          -- the event date, not the recording date

-- ADR-0015's engagements_outcome_is_dated, deliberately not
-- biconditional: a null outcome forbids a date outright, an outcome
-- other than 'unknown' requires one so the Visit-type derivation is
-- never handed a birth it cannot place in time, and 'unknown' -- "the
-- Engagement ended and the Practice never learned" -- carries a date
-- when one happens to be known and none when it is not, because
-- requiring one there would make every abandoned Engagement uncloseable
-- until somebody invented a date.
--
-- Written as a CASE rather than as the ADR's own three-way OR sketch,
-- which does not do what the paragraph beside it says: in that form a
-- row with a date and no outcome satisfies the third disjunct and passes,
-- while the ADR's text is explicit that "a null outcome still forbids a
-- date outright". The CASE says each of the three rules once, in the
-- ADR's own words, and no branch can be satisfied by another's.
ALTER TABLE engagements ADD CONSTRAINT engagements_outcome_is_dated CHECK (
    CASE
        WHEN birth_outcome IS NULL       THEN pregnancy_ended_on IS NULL
        WHEN birth_outcome = 'unknown'   THEN true
        ELSE pregnancy_ended_on IS NOT NULL
    END
);

-- ---------------------------------------------------------------------
-- The audit record. engagement_events (00090) is ADR-0015's one table
-- for all four mutable Engagement facts, so this ticket adds columns to
-- it rather than a second table. The date pair is here alongside the
-- outcome pair the ADR's column list names: the freeze covers both
-- columns, so a correction can move the date, and an audit row that
-- recorded only the outcome could not answer what the date used to be.
-- ---------------------------------------------------------------------
ALTER TABLE engagement_events ADD COLUMN previous_birth_outcome birth_outcome;
ALTER TABLE engagement_events ADD COLUMN birth_outcome birth_outcome;
ALTER TABLE engagement_events ADD COLUMN previous_pregnancy_ended_on date;
ALTER TABLE engagement_events ADD COLUMN pregnancy_ended_on date;

-- ---------------------------------------------------------------------
-- Immutability is a trigger, because a policy cannot see both rows: an
-- UPDATE policy's USING clause sees the old row and its WITH CHECK sees
-- the new one, and neither can compare the two. The *who* stays at the
-- handler per ADR-0006 -- the database only enforces that a frozen
-- outcome cannot be overwritten by an ordinary update, and the
-- Owner-only correction handler is the single caller that opens
-- app.allow_outcome_correction. That is the same narrow
-- session-variable door app.invite_token (00026) and
-- app.invite_token_digest (00039) already use: one writer sets it, one
-- trigger or policy reads it, and nothing else in the codebase touches
-- the name.
--
-- client_id rides along because ADR-0015's freeze rule states the two
-- together: the abuse being designed against is serving a second baby
-- on a Credit already spent, and re-pointing the row at another Client
-- is the other half of that door. Nothing in the product updates
-- client_id today -- a Client with Engagements cannot even be merged
-- away (client/merge.go).
--
-- `kind` is deliberately NOT frozen (ADR-0015: "the post-birth freeze on
-- kind is a rule, not a constraint") -- a postpartum-only Engagement
-- records live_birth at intake, so a database freeze keyed on the
-- outcome would freeze its kind from the moment the row was created.
-- ---------------------------------------------------------------------

-- +goose StatementBegin
CREATE FUNCTION engagements_freeze_outcome() RETURNS trigger
    LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.client_id IS DISTINCT FROM OLD.client_id THEN
        RAISE EXCEPTION 'an Engagement belongs to one Client for life; client_id cannot be changed';
    END IF;
    IF OLD.birth_outcome IS NOT NULL
       AND (NEW.birth_outcome, NEW.pregnancy_ended_on)
           IS DISTINCT FROM (OLD.birth_outcome, OLD.pregnancy_ended_on)
       AND coalesce(current_setting('app.allow_outcome_correction', true), '') <> 'on' THEN
        RAISE EXCEPTION 'a recorded birth outcome is frozen; only a Practice Owner may correct it';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- No REVOKE ... FROM PUBLIC here, unlike
-- push_subscriptions_for_message_recipient (00010): a trigger function's
-- EXECUTE privilege is checked against the role running the UPDATE, so
-- revoking it from PUBLIC would make every write to engagements fail for
-- app_runtime. It takes no arguments and reads nothing it is not already
-- being handed, so there is nothing to leak by calling it.
CREATE TRIGGER engagements_freeze_outcome
    BEFORE UPDATE ON engagements
    FOR EACH ROW EXECUTE FUNCTION engagements_freeze_outcome();

-- +goose Down
DROP TRIGGER engagements_freeze_outcome ON engagements;
DROP FUNCTION engagements_freeze_outcome();

ALTER TABLE engagement_events DROP COLUMN pregnancy_ended_on;
ALTER TABLE engagement_events DROP COLUMN previous_pregnancy_ended_on;
ALTER TABLE engagement_events DROP COLUMN birth_outcome;
ALTER TABLE engagement_events DROP COLUMN previous_birth_outcome;

ALTER TABLE engagements DROP CONSTRAINT engagements_outcome_is_dated;
ALTER TABLE engagements DROP COLUMN pregnancy_ended_on;
ALTER TABLE engagements DROP COLUMN birth_outcome;

DROP TYPE birth_outcome;
