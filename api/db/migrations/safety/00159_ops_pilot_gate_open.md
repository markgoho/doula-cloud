# 00159_ops_pilot_gate_open.sql

## ADD CONSTRAINT ... CHECK

```sql
ALTER TABLE operator_acts ADD CONSTRAINT operator_acts_action_check
    CHECK (action IN ('sign_in', 'sign_out', 'step_up_refused', 'refund', 'founding_grant', 'second_factor_cleared', 'owner_contact_read', 'feedback_read', 'pilot_address_added', 'pilot_gate_opened'))
```

`operator_acts_action_check` is dropped and re-added, widened by one value. Every existing row was held by `00158`'s version, `action IN ('sign_in', 'sign_out', 'step_up_refused', 'refund', 'founding_grant', 'second_factor_cleared', 'owner_contact_read', 'feedback_read', 'pilot_address_added')`, and each of those nine values is in the new list, so no row the old constraint admitted can fail the new one. `operator_acts` refuses every UPDATE, so no row has changed since it was checked.

```sql
ALTER TABLE pilot_gate ADD CONSTRAINT pilot_gate_act_opened_it
    CHECK (operator_act_id IS NULL OR opened_at IS NOT NULL)
```

`operator_act_id` is added by the statement before this one, so on the one existing `pilot_gate` row it is NULL and the check holds whether the gate is open or closed. A gate opened through `doula-api` before this migration stays open with no act on record (no backfill, ADR-0062).
