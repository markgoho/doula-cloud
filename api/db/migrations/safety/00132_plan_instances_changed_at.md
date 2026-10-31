# 00132_plan_instances_changed_at.sql

Both row-dependent statements in this migration run over `plan_instances`, and both rest on one fact: `changed_at` is added on the line above them, nullable and with no DEFAULT, so every existing row holds NULL in it until the UPDATE fills it, and `plan_instances.created_at` is `timestamptz NOT NULL` (00012), so the value it is filled from is never NULL.

## DML (UPDATE / DELETE / INSERT)

```sql
UPDATE plan_instances SET changed_at = created_at
```

This UPDATE has no WHERE, so it touches every `plan_instances` row, and nothing can refuse any of them. `changed_at` is a plain `timestamptz` column with no CHECK, no foreign key, no unique index and no trigger, and the value it takes is the row's own `created_at`, the same type, copied as is. The statement runs as the migration role, which bypasses the row-level security policies on `plan_instances`.

## ALTER COLUMN ... SET NOT NULL

```sql
ALTER TABLE plan_instances ALTER COLUMN changed_at SET NOT NULL
```

Every row already holds a non-NULL `changed_at`: the UPDATE just above, in the same migration and transaction, set it to `created_at` on every row, and `created_at` is `NOT NULL`. Nothing else writes the column between the two statements, because goose runs the migration in one transaction and the column did not exist before it.
