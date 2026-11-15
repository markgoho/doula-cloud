# 00178_contracts_sent_and_voided.sql

The migration adds four nullable columns to `contracts`, backfills them in two statements, fills any signed row that lacks `signed_at`, then adds three CHECKs and a generated column that the backfilled rows must satisfy.

## DML (UPDATE / DELETE / INSERT)

```sql
UPDATE contracts
   SET signed_at = COALESCE(sent_at, created_at)
 WHERE status = 'signed' AND signed_at IS NULL
```

It touches only signed rows with no `signed_at`, which no code path writes. Nothing refuses it: `signed_at` is a plain nullable column with no foreign key, no unique index and no CHECK before the one added below, `created_at` is `NOT NULL`, and `sent_at` was set by the statements above for every non-draft row.

## ADD CONSTRAINT ... CHECK

```sql
ALTER TABLE contracts ADD CONSTRAINT contracts_sent_at_matches_status CHECK (
    (status = 'draft') = (sent_at IS NULL)
)
```

`sent_at` is new, so every row starts NULL. The first backfill statement sets `sent_at` on every row where `status <> 'draft'` to `COALESCE(<the Engagement's contract_sent Activity in the Contract's window>, signed_at, created_at)`. `created_at` is `NOT NULL`, so the result is never NULL, and every non-draft row leaves the statement with a `sent_at`. A draft row is not touched and keeps NULL. So `sent_at IS NULL` holds for exactly the draft rows, which is what the CHECK says.

```sql
ALTER TABLE contracts ADD CONSTRAINT contracts_voided_at_matches_status CHECK (
    (status = 'voided') = (voided_at IS NOT NULL)
)
```

`voided_at` is new, so every row starts NULL. The second backfill statement sets it on every row where `status = 'voided'` to `COALESCE(<the Engagement's contract_voided Activity in the Contract's window>, signed_at, created_at)`, which is never NULL for the same reason. A row in any other status is not touched and keeps NULL. So `voided_at IS NOT NULL` holds for exactly the voided rows.

```sql
ALTER TABLE contracts ADD CONSTRAINT contracts_signed_at_matches_status CHECK (
    status <> 'signed' OR signed_at IS NOT NULL
)
```

`signed_at` already exists and no code path writes a signed row without it, but nothing refused one. Before the CHECK, a statement sets `signed_at` to `COALESCE(sent_at, created_at)` on every row where `status = 'signed'` and `signed_at IS NULL`. `sent_at` was backfilled for every non-draft row above and `created_at` is `NOT NULL`, so the result is never NULL, and every signed row leaves the statement with a `signed_at`. A row in any other status satisfies the first branch. So the CHECK holds for every existing row.

## ADD COLUMN ... GENERATED ALWAYS AS

```sql
ALTER TABLE contracts ADD COLUMN stepped_at timestamptz GENERATED ALWAYS AS (
    CASE status
        WHEN 'draft' THEN created_at
        WHEN 'sent' THEN sent_at
        WHEN 'signed' THEN signed_at
        ELSE voided_at
    END
) STORED
```

The expression is total. It reads only `status`, which is `NOT NULL`, and four timestamptz columns, with no cast of a stored value and no function call that could fail on an existing row; a NULL operand gives a NULL result rather than an error. An existing row's `status` is one of the four labels of `contract_status`, so every row falls into a branch.
