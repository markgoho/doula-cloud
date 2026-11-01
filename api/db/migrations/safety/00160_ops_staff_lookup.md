# 00160_ops_staff_lookup.sql

## ADD CONSTRAINT ... CHECK

```sql
ALTER TABLE operator_acts ADD CONSTRAINT operator_acts_action_check
    CHECK (action IN ('sign_in', 'sign_out', 'step_up_refused', 'refund', 'founding_grant', 'second_factor_cleared', 'owner_contact_read', 'feedback_read', 'pilot_address_added', 'pilot_gate_opened', 'legal_version_published', 'staff_lookup'))
```

`operator_acts_action_check` is dropped and re-added, widened by one value. Every existing row was held by `00159`'s version, and each of its eleven values is in the new list, so no row the old constraint admitted can fail the new one. `operator_acts` refuses every UPDATE, so no row has changed since it was checked.
