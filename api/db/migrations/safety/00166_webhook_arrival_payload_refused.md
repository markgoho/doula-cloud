# 00166_webhook_arrival_payload_refused.sql

The new CHECK only widens the allowed set of `webhook_arrivals.outcome`.

## ADD CONSTRAINT ... CHECK

```sql
ALTER TABLE webhook_arrivals ADD CONSTRAINT webhook_arrivals_outcome_check CHECK (outcome IN (
    'accepted', 'dropped', 'body_refused', 'signature_refused', 'livemode_refused', 'payload_refused', 'refused', 'failed'
))
```

The constraint it replaces (00151) allowed the same list without `'payload_refused'`, and the column is `NOT NULL`. Every existing row therefore holds a value in the old list, which is a subset of the new one, and no existing row can violate the new predicate. The old constraint is dropped in the same migration and transaction, so no row is ever unconstrained.
