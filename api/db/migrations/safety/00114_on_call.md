# 00114_on_call.sql

Four `ADD CONSTRAINT ... CHECK` statements (#1093), each added in the same migration as the columns it constrains. Every existing row already satisfies each predicate, because each column is new: its only value in an existing row is the one this migration itself wrote a moment earlier.

## ADD CONSTRAINT ... CHECK

```sql
ALTER TABLE practices ADD CONSTRAINT practices_on_call_start_week_range
    CHECK (on_call_start_week BETWEEN 20 AND 42)
```

`practices_on_call_start_week_range` checks `on_call_start_week BETWEEN 20 AND 42`. The column is added in the statement just before with `NOT NULL DEFAULT 37`, so every existing `practices` row holds 37, which is inside the bound.

```sql
ALTER TABLE practices ADD CONSTRAINT practices_on_call_grace_days_range
    CHECK (on_call_grace_days BETWEEN 0 AND 42)
```

`practices_on_call_grace_days_range` checks `on_call_grace_days BETWEEN 0 AND 42`. The column is added in the statement just before with `NOT NULL DEFAULT 14`, so every existing `practices` row holds 14, which is inside the bound.

```sql
ALTER TABLE engagements ADD CONSTRAINT engagements_on_call_override_shape CHECK (
    (on_call_start_rule IS NULL AND on_call_start_week IS NULL)
    OR (on_call_start_rule = 'attachment_granted' AND on_call_start_week IS NULL)
    OR (on_call_start_rule = 'gestational_week' AND on_call_start_week BETWEEN 20 AND 42)
)
```

`engagements_on_call_override_shape` checks that `on_call_start_rule` and `on_call_start_week` are both NULL, or name a legal override. Both columns are added in the statements just before, nullable and with no DEFAULT, so every existing `engagements` row holds NULL in both, which is the first branch of the predicate.

```sql
ALTER TABLE engagement_attachments ADD CONSTRAINT engagement_attachments_on_call_order
    CHECK (on_call_from IS NULL OR on_call_to IS NULL OR on_call_from <= on_call_to)
```

`engagement_attachments_on_call_order` checks `on_call_from IS NULL OR on_call_to IS NULL OR on_call_from <= on_call_to`. Both columns are added in the statements just before, nullable and with no DEFAULT, so every existing `engagement_attachments` row holds NULL in both, which satisfies the first branch.
