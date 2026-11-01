# 00143_ops_runtime_and_operator_tables.sql

## DO block

```sql
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'ops_runtime') THEN
        CREATE ROLE ops_runtime NOLOGIN;
    END IF;
END
$$;
```

The block reads only `pg_roles`, the cluster's role catalog, and writes no table. It creates `ops_runtime` when the role is missing and does nothing when it exists, so no row in any table can refuse it. It is the same guard 00002 put around `app_runtime`: roles are cluster-wide, and the guard keeps a second database on the same instance from failing on a role that is already there.
