# 00178_contracts_sent_and_voided.sql

The migration adds four nullable columns to `contracts`, backfills them in two statements, then adds two CHECKs and a generated column that the backfilled rows must satisfy.

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
