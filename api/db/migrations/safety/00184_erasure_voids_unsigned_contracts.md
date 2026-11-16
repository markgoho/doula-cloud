# 00184_erasure_voids_unsigned_contracts.sql

The migration adds a boolean column with a constant default and a CHECK on it to `contracts`, then replaces `contracts_sent_at_matches_status` with a CHECK that is looser than the one it drops.

## ADD CONSTRAINT ... CHECK

```sql
ALTER TABLE contracts ADD CONSTRAINT contracts_voided_by_erasure_is_voided CHECK (
    NOT voided_by_erasure OR status = 'voided'
)
```

`voided_by_erasure` is new and `NOT NULL DEFAULT false`, so every existing row holds `false` and `NOT false` is true, whatever its status.

```sql
ALTER TABLE contracts ADD CONSTRAINT contracts_sent_at_matches_status CHECK (
    CASE status
        WHEN 'draft' THEN sent_at IS NULL
        WHEN 'voided' THEN voided_by_erasure OR sent_at IS NOT NULL
        ELSE sent_at IS NOT NULL
    END
)
```

The constraint it replaces held `(status = 'draft') = (sent_at IS NULL)` for every row. A draft row has `sent_at IS NULL`, which is the first branch. A voided or sent or signed row has `sent_at IS NOT NULL`, which satisfies the second branch's right-hand side (`voided_by_erasure` is `false` on every existing row) and the third. So every row the old constraint admitted, this one admits.
