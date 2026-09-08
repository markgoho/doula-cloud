-- +goose Up
-- #940: ADR-0015's engagements_completed_is_explained, finally whole.
-- 00090 landed half of it as engagements_completed_has_reason -- an
-- ending_reason and nothing else -- because birth_outcome did not exist
-- yet, and 00093 added the column but left the constraint alone because
-- tightening it is a change to the completion path, not a one-line
-- constraint edit. This migration is that change.
--
-- Why the constraint is worth its cost: ADR-0015 accepts up front that a
-- Client who vanished during intake, whom the Practice never got near a
-- birth with, must still be marked 'unknown'. "unknown is the honest
-- answer, not an admission of sloppiness, and this constraint is the
-- only thing that ever causes it to be written down." Without it the
-- column stays null forever on every abandoned Engagement, and *was this
-- recorded, or did nobody ask?* has no answer.
--
-- ---------------------------------------------------------------------
-- How completion satisfies it: by refusing, not by collecting.
-- ---------------------------------------------------------------------
-- #940 left open which of two shapes the completion path takes --
-- completion refuses until the outcome has already been recorded through
-- 00093's own endpoint, or the status-transition request grows fields
-- and carries the outcome itself. This migration ships the first, and
-- engagement.TransitionHandler carries the matching named refusal
-- (BIRTH_OUTCOME_REQUIRED, 409).
--
-- 1. The freeze is the deciding argument. RecordBirthOutcomeHandler owns
--    the whole of ADR-0015's outcome rule: the vocabulary, the date rule
--    this file's neighbor engagements_outcome_is_dated states, the
--    BEFORE UPDATE freeze, the Owner-only correction door, and the
--    'birth_outcome_recorded' audit row. A transition that carried an
--    outcome would have to re-answer every one of those questions --
--    above all "a completion never silently overwrites an outcome
--    already recorded", which would become a second freeze
--    implementation. Refusing instead makes that hold by construction:
--    the completion path never writes birth_outcome at all.
-- 2. The model already separates the two facts. ADR-0015 records the
--    outcome "whenever it becomes known", not at completion -- a
--    postpartum-only Engagement records live_birth at intake, before
--    there is any status move to attach it to. A field on the transition
--    request would make completion look like the fact's natural home,
--    which is exactly the reading ADR-0015 rejects.
-- 3. The refusal is a pointer, not a dead end. #943 put the recording
--    control on the Engagement hub, a few hundred pixels from the
--    "Mark care complete" button, so the refusal names a control already
--    on the reader's screen.
-- 4. The transition request contract does not move, so no caller and no
--    fixture has to learn a new field to keep completing an Engagement.

ALTER TABLE engagements DROP CONSTRAINT engagements_completed_has_reason;

-- Every row already 'completed' was completed under the half-constraint,
-- so it may carry no outcome and would fail the new one. 'unknown' is
-- the backfill for the same reason it is the answer a Practice types:
-- the Engagement ended and nobody recorded what happened, which is what
-- 'unknown' says. It carries no pregnancy_ended_on, which
-- engagements_outcome_is_dated (00093) allows, so this invents no date.
-- Written as a plain UPDATE rather than NOT VALID: pre-launch there is
-- no production data and no table big enough for the lock to matter, and
-- a NOT VALID constraint would leave exactly the un-answered rows this
-- ticket exists to stop.
UPDATE engagements SET birth_outcome = 'unknown'
 WHERE status = 'completed' AND birth_outcome IS NULL;

ALTER TABLE engagements ADD CONSTRAINT engagements_completed_is_explained CHECK (
    status <> 'completed'
    OR (birth_outcome IS NOT NULL AND ending_reason IS NOT NULL)
);

-- +goose Down
ALTER TABLE engagements DROP CONSTRAINT engagements_completed_is_explained;

ALTER TABLE engagements ADD CONSTRAINT engagements_completed_has_reason CHECK (
    status <> 'completed' OR ending_reason IS NOT NULL
);
