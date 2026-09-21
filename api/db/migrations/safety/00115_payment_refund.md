# 00115_payment_refund.sql

Every row-dependent statement in this migration rests on one fact: `payment_kind` gains the value `'refund'` in this same migration (`ALTER TYPE payment_kind ADD VALUE 'refund'`), so **no existing `payments` row has `kind = 'refund'`** when any of the statements below runs. Every row they meet is a `'stripe'`, `'manual'` or `'reversal'` row that `00103`'s constraints already hold, and each statement below is either the same rule for those kinds or a looser one.

## CREATE UNIQUE INDEX

```sql
CREATE UNIQUE INDEX payments_one_reversal_per_payment ON payments (target_payment_id)
    WHERE kind = 'reversal'
```

No duplicate can exist. It replaces `00103`'s `payments_reversed_payment_id_unique ON payments (reversed_payment_id) WHERE reversed_payment_id IS NOT NULL`, which is dropped just above it on the same (renamed) column. Under `00103`'s `payments_reversed_payment_id_matches_kind` CHECK, a non-null pointer appears only on a `'reversal'` row, so the old index's predicate and the new one's select exactly the same rows today, and the old index already proved those rows unique. The new predicate is narrower from here on, not wider: it stops covering the Refund rows this migration lets carry a pointer, which is the point of rebuilding it.

```sql
CREATE UNIQUE INDEX payments_refund_stripe_reference_unique ON payments (stripe_payment_reference)
    WHERE kind = 'refund' AND stripe_payment_reference IS NOT NULL
```

No duplicate can exist, because the predicate selects no row at all: no row has `kind = 'refund'` yet.

## ADD CONSTRAINT ... CHECK

```sql
ALTER TABLE payments ADD CONSTRAINT payments_amount_sign_matches_kind CHECK (
    (kind IN ('reversal', 'refund') AND amount_cents < 0)
    OR (kind NOT IN ('reversal', 'refund') AND amount_cents > 0)
)
```

`payments_amount_sign_matches_kind` is dropped and re-added. For every kind that exists today it states exactly what `00103`'s version stated — a `'reversal'` row is negative, a `'stripe'` or `'manual'` row positive — and `00103`'s version already held every existing row to it. The only new branch is `'refund'`, which no existing row has.

```sql
ALTER TABLE payments ADD CONSTRAINT payments_target_payment_id_matches_kind CHECK (
    (kind IN ('reversal', 'refund') AND target_payment_id IS NOT NULL)
    OR (kind NOT IN ('reversal', 'refund') AND target_payment_id IS NULL)
)
```

`payments_target_payment_id_matches_kind` is dropped and re-added. It is `00103`'s `payments_reversed_payment_id_matches_kind` on the renamed column, and for every kind that exists today it states exactly the same rule: a `'reversal'` row carries a pointer, a `'stripe'` or `'manual'` row does not. The only new branch is `'refund'`, which no existing row has.

```sql
ALTER TABLE payments ADD CONSTRAINT payments_method_matches_kind CHECK (
    (kind = 'manual' AND method IS NOT NULL)
    OR (kind = 'stripe' AND method IS NULL)
    OR (kind = 'reversal' AND method IS NULL)
    OR kind = 'refund'
)
```

`payments_method_matches_kind` is dropped and re-added strictly looser: its three existing branches are unchanged, and it gains `OR kind = 'refund'`. Every row the old constraint accepted, the new one accepts.
