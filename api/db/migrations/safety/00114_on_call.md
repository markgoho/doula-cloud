# 00114_on_call.sql

Four `ADD CONSTRAINT ... CHECK` statements (#1093), each added in the same migration as the columns it constrains. Every existing row already satisfies each predicate, because each column is new: its only value in an existing row is the one this migration itself wrote a moment earlier.

## ADD CONSTRAINT ... CHECK

`practices_on_call_start_week_range` and `practices_on_call_grace_days_range` check `on_call_start_week BETWEEN 20 AND 42` and `on_call_grace_days BETWEEN 0 AND 42`. Both columns are added in the statements just before with `NOT NULL DEFAULT 37` and `NOT NULL DEFAULT 14`, so every existing `practices` row holds 37 and 14, and both are inside their bounds.

`engagements_on_call_override_shape` checks that `on_call_start_rule` and `on_call_start_week` are both NULL, or name a legal override. Both columns are added in the statements just before, nullable and with no DEFAULT, so every existing `engagements` row holds NULL in both, which is the first branch of the predicate.

`engagement_attachments_on_call_order` checks `on_call_from IS NULL OR on_call_to IS NULL OR on_call_from <= on_call_to`. Both columns are added in the statements just before, nullable and with no DEFAULT, so every existing `engagement_attachments` row holds NULL in both, which satisfies the first branch.
