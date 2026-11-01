# 00153_ops_pilot_allowlist_add.sql

## ADD CONSTRAINT ... CHECK

```sql
ALTER TABLE operator_acts ADD CONSTRAINT operator_acts_action_check
    CHECK (action IN ('sign_in', 'sign_out', 'step_up_refused', 'refund', 'founding_grant', 'second_factor_cleared', 'owner_contact_read', 'feedback_read', 'pilot_address_added'))
```

`operator_acts_action_check` is dropped and re-added, widened by one value. Every existing row was held by `00155`'s version, `action IN ('sign_in', 'sign_out', 'step_up_refused', 'refund', 'founding_grant', 'second_factor_cleared', 'owner_contact_read', 'feedback_read')`, and each of those eight values is in the new list, so no row the old constraint admitted can fail the new one. `operator_acts` refuses every UPDATE, so no row has changed since it was checked.

`pilot_allowlist_operator_act` is added `NOT VALID`, so the addresses added through `doula-api` before this migration, which have no act to key, are not checked against it (no backfill, ADR-0062). Only a row written from here on is.
