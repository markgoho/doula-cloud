# 00172_practice_standing.sql

## ADD CONSTRAINT ... CHECK

```sql
ALTER TABLE practices ADD CONSTRAINT practices_deletion_request_whole CHECK (
    (deletion_requested_at IS NULL) = (deletion_requested_by IS NULL)
    AND (deletion_requested_at IS NULL) = (deletion_finalize_at IS NULL)
)
```

Three statements have ever written these columns since `00088` added them, all in `api/internal/practicedeletion`: the request sets all three in one `UPDATE`, the restore sets all three to `NULL` in one `UPDATE`, and finalize writes only `deleted_at`. No other writer exists, and a `practices` row inserted by signup names none of the three, so it holds three `NULL`s. Every existing row therefore holds all three or none.

```sql
ALTER TABLE practices ADD CONSTRAINT practices_deleted_after_request CHECK (
    deleted_at IS NULL OR deletion_requested_at IS NOT NULL
)
```

`deleted_at` is written only by the finalize worker, which reads the row `FOR UPDATE` first and does nothing unless `deletion_requested_at` is set, and nothing clears `deletion_requested_at` once `deleted_at` is set: the restore's own `UPDATE` matches only a row whose `deleted_at` is `NULL`. So every row with `deleted_at` set also has `deletion_requested_at` set.

## ADD COLUMN ... NOT NULL without DEFAULT

```sql
ALTER TABLE practices ADD COLUMN standing practice_standing NOT NULL GENERATED ALWAYS AS (
    CASE
        WHEN deleted_at IS NOT NULL THEN 'deleted'::practice_standing
        WHEN deletion_requested_at IS NOT NULL THEN 'pending_deletion'::practice_standing
        ELSE 'active'::practice_standing
    END
) STORED
```

The column is generated, not defaulted: Postgres computes it for every existing row from the expression, and the `CASE` has an `ELSE`, so it is never `NULL` for any row.

## ADD COLUMN ... GENERATED ALWAYS AS

```sql
ALTER TABLE practices ADD COLUMN standing practice_standing NOT NULL GENERATED ALWAYS AS (
    CASE
        WHEN deleted_at IS NOT NULL THEN 'deleted'::practice_standing
        WHEN deletion_requested_at IS NOT NULL THEN 'pending_deletion'::practice_standing
        ELSE 'active'::practice_standing
    END
) STORED
```

The expression is total: it only asks whether two columns are `NULL`, which cannot fail for any value, and each branch is a constant of the column's own enum type.
