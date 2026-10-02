# 00121_feedback_erasure_and_retention.sql

Both row-dependent statements in this migration rest on one fact: the `act` column is added in this same migration with `DEFAULT 'open'`, so **every existing `feedback_issue_outbox` row has `act = 'open'`** when the statements below run. No row has `act = 'close_erased'` yet, because nothing could write one before this migration.

## ADD CONSTRAINT ... CHECK

```sql
ALTER TABLE feedback_issue_outbox ADD CONSTRAINT feedback_issue_outbox_act_shape CHECK (
    (act = 'open' AND feedback_id IS NOT NULL AND issue_number IS NULL)
    OR
    (act = 'close_erased' AND feedback_id IS NULL AND issue_number IS NOT NULL)
)
```

Every existing row satisfies the first branch. `act` is `'open'` on each one, from the column default. `feedback_id` is not null on each one: `00120` declared it `NOT NULL`, this migration drops that only in the statement directly before this one, and no statement between the two writes the column. `issue_number` is null on each one: the column is added by this migration with no default, and no statement before this one writes it.

## CREATE UNIQUE INDEX

```sql
CREATE UNIQUE INDEX feedback_issue_outbox_one_pending_close
    ON feedback_issue_outbox (issue_number)
    WHERE status = 'pending' AND act = 'close_erased'
```

No duplicate can exist, because the predicate selects no row at all: no existing row has `act = 'close_erased'`.
