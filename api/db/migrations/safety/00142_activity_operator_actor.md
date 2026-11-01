# 00142_activity_operator_actor.sql

## ADD CONSTRAINT ... CHECK

```sql
ALTER TABLE activity ADD CONSTRAINT activity_actor CHECK (
    (actor_kind = 'staff' AND actor_staff_id IS NOT NULL AND actor_client_id IS NULL AND actor_operator_act_id IS NULL)
    OR (actor_kind = 'client' AND actor_client_id IS NOT NULL AND actor_staff_id IS NULL AND actor_operator_act_id IS NULL)
    OR (actor_kind = 'system' AND actor_staff_id IS NULL AND actor_client_id IS NULL AND actor_operator_act_id IS NULL)
    OR (actor_kind = 'operator' AND actor_operator_act_id IS NOT NULL AND actor_staff_id IS NULL AND actor_client_id IS NULL)
)
```

`activity_actor` is dropped and re-added. Every existing row was held by `00051`'s version, whose three branches this one keeps word for word, each with one added clause, `actor_operator_act_id IS NULL`. That clause holds for every existing row, because the column is added two statements above with no DEFAULT, so it is NULL in every row. The fourth branch, `'operator'`, matches no existing row: the value was added to `activity_actor_kind` by `00141`, and nothing wrote it before this migration.
