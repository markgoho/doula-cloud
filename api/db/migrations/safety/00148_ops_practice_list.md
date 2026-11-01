# 00148_ops_practice_list.sql

## ADD CONSTRAINT ... CHECK

```sql
ALTER TABLE operator_acts ADD CONSTRAINT operator_acts_action_check
    CHECK (action IN ('sign_in', 'sign_out', 'step_up_refused', 'refund', 'founding_grant', 'second_factor_cleared', 'owner_contact_read'))
```

`operator_acts_action_check` is dropped and re-added, widened by one value. Every existing row was held by `00145`'s version, `action IN ('sign_in', 'sign_out', 'step_up_refused', 'refund', 'founding_grant', 'second_factor_cleared')`, and each of those six values is in the new list, so no row the old constraint admitted can fail the new one. `operator_acts` refuses every UPDATE, so no row has changed since it was checked.
