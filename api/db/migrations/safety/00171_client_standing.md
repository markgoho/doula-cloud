# 00171_client_standing.sql

The migration adds a generated column and two CHECKs to `clients` that rows already in the table could otherwise refuse, and a third CHECK it adds `NOT VALID`.

## ADD COLUMN ... GENERATED ALWAYS AS

```sql
ALTER TABLE clients ADD COLUMN standing client_standing GENERATED ALWAYS AS (
    CASE
        WHEN merged_into IS NOT NULL AND erased_at IS NOT NULL THEN 'merged_then_erased'::client_standing
        WHEN merged_into IS NOT NULL THEN 'merged'::client_standing
        WHEN erased_at IS NOT NULL THEN 'erased'::client_standing
        ELSE 'live'::client_standing
    END
) STORED
```

The expression is total. Each branch tests only `IS NULL` / `IS NOT NULL` on two existing columns, which is defined for every value either column can hold, the `ELSE` covers the last combination, and every result is a literal of the enum created in the same migration. There is no cast of a stored value and no function call that could fail on an existing row.

## ADD COLUMN ... NOT NULL without DEFAULT

```sql
ALTER TABLE clients ADD COLUMN standing client_standing GENERATED ALWAYS AS (
    CASE
        WHEN merged_into IS NOT NULL AND erased_at IS NOT NULL THEN 'merged_then_erased'::client_standing
        WHEN merged_into IS NOT NULL THEN 'merged'::client_standing
        WHEN erased_at IS NOT NULL THEN 'erased'::client_standing
        ELSE 'live'::client_standing
    END
) STORED
```

The column carries no `NOT NULL` constraint: the words are the expression's own `IS NOT NULL` tests. A generated column is computed for every existing row as it is added, and the expression above gives each one a value, so no row is left without one.

## ADD CONSTRAINT ... CHECK

```sql
ALTER TABLE clients ADD CONSTRAINT clients_merge_attribution_needs_a_merge CHECK (
    merged_into IS NOT NULL OR merged_by_staff_id IS NULL
)
```

`merged_by_staff_id` has one writer, `client.setMergedInto`, which sets it in the same `UPDATE` that sets `merged_into`. 00112's backfill wrote `merged_at` only, never `merged_by_staff_id`, and `clients_update`'s `USING merged_into IS NULL` (00080) has refused every later write to a tombstone, so nothing has ever cleared `merged_into` under a set `merged_by_staff_id`.

```sql
ALTER TABLE clients ADD CONSTRAINT clients_never_merged_into_itself CHECK (
    merged_into <> id
)
```

`clients_update`'s `WITH CHECK (merged_into IS NULL OR merged_into <> id)` has refused a self-merge since `merged_into` was added in 00080, which is the only migration that could have written one, and a row whose `merged_into` is `NULL` passes this CHECK.
