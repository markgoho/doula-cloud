# 00124_staff_first_and_last_name.sql

Both row-dependent statements in this migration run over `staff`, and both rest on the same facts: the two columns they write or read from (`first_name`, `last_name`) are added a few lines above with `NOT NULL DEFAULT ''`, so every existing row holds `''` in them, and `staff.name` is `NOT NULL` (00002), so the expression below always has a string to work on.

## DML (UPDATE / DELETE / INSERT)

```sql
UPDATE staff
   SET first_name = COALESCE(substring(btrim(name) from '^\S+'), ''),
       last_name  = COALESCE(btrim(substring(btrim(name) from '^\S+\s+(.*)$')), '')
```

This UPDATE has no WHERE, so it touches every `staff` row, and nothing can refuse any of them. `first_name` and `last_name` are plain `text NOT NULL` columns with no CHECK, no foreign key, no unique index and no trigger, and each right-hand side is wrapped in `COALESCE(..., '')`, so a name with no match (an empty string, or one with no trailing words) yields `''` rather than NULL. `substring(... from <pattern>)` returns NULL, not an error, when the pattern does not match, and `btrim` of NULL is NULL, which `COALESCE` absorbs. The statement runs as the migration role, which bypasses the row-level security policies on `staff`.

## ADD COLUMN ... GENERATED ALWAYS AS

```sql
ALTER TABLE staff ADD COLUMN name text
    GENERATED ALWAYS AS (btrim(first_name || ' ' || last_name)) STORED
```

The expression is evaluated for every existing row, and cannot fail on any of them. `first_name` and `last_name` are `NOT NULL` (the statement above has filled both), so the concatenation never meets a NULL. `||` and `btrim` on `text` are immutable, which Postgres requires of a generated column's expression, and neither can raise on any string. The column is added immediately after `DROP COLUMN name`, in the same migration and transaction, so no other object is left depending on a column that no longer exists: the migration test builds the whole schema through 00124 and would have refused the `DROP COLUMN` had a view or index depended on `name`.
