# 00168_portal_access_state.sql

The migration gives every `client_portal_users` row a `state`, then adds the CHECKs and the one-open-invitation index that read it. Every statement below runs after the backfill, and each is safe because of what the backfill wrote.

## DML (UPDATE / DELETE / INSERT)

```sql
UPDATE client_portal_users pu
   SET state = s.state,
       invite_token = CASE WHEN s.state IN ('invited', 'refused') THEN pu.invite_token END,
       invite_token_expires_at = CASE WHEN s.state IN ('invited', 'refused')
                                      THEN COALESCE(pu.invite_token_expires_at, pu.created_at) END,
       access_refused_at = CASE WHEN s.state = 'refused' THEN pu.access_refused_at END,
       revoked_at = CASE WHEN s.state = 'revoked' THEN COALESCE(s.merged_at, pu.created_at) END,
       revoked_by_staff_id = CASE WHEN s.state = 'revoked' THEN s.merged_by_staff_id END,
       revoke_reason = CASE WHEN s.state = 'revoked' THEN 'record_merged'::portal_access_revoke_reason END,
       unlinked_at = CASE WHEN s.state = 'unlinked' THEN COALESCE(s.erased_at, pu.created_at) END
  FROM (
      SELECT p.id, c.merged_at, c.merged_by_staff_id, c.erased_at,
             CASE
                 WHEN p.identity_uid IS NOT NULL THEN 'accepted'
                 WHEN p.invite_token IS NOT NULL AND p.access_refused_at IS NOT NULL THEN 'refused'
                 WHEN p.invite_token IS NOT NULL THEN 'invited'
                 WHEN c.merged_into IS NOT NULL THEN 'revoked'
                 ELSE 'unlinked'
             END::portal_access_state AS state
        FROM client_portal_users p
        JOIN clients c ON c.id = p.client_id
  ) s
 WHERE s.id = pu.id
```

It reaches every row: `client_portal_users.client_id` is `NOT NULL REFERENCES clients (id)` (`00006`), so each row joins exactly one `clients` row in the subquery, and the subquery's `id` is the table's primary key, so each row is matched exactly once. Nothing it writes can be refused. No CHECK on this table exists yet; they are added after it. It never changes `identity_uid` or `client_id`, so neither the `(identity_uid, client_id)` unique constraint (`00107`), nor the identity FK (`00073`), nor `client_portal_users_one_pending_per_client` (`00026`, still in force at this point) can be met. It changes `invite_token` only to its own value or to NULL, so its `UNIQUE` (`00026`) cannot be met. `revoked_by_staff_id` takes `clients.merged_by_staff_id`, which is itself `REFERENCES staff (id)` (`00112`), so the new FK holds. No trigger exists on the table yet. Nothing references the columns it clears.

## ALTER COLUMN ... SET NOT NULL

```sql
ALTER TABLE client_portal_users ALTER COLUMN state SET NOT NULL
```

The backfill above runs first in the same migration and reaches every row (see above). Its `CASE` ends in an `ELSE`, so it never writes NULL.

## ADD CONSTRAINT ... CHECK

Each constraint follows from the backfill. A state's branch of the `CASE` is reached only when the columns that state owns are as the constraint requires, and the `SET` list keeps or fills those columns for that state and clears them for every other state.

```sql
ALTER TABLE client_portal_users ADD CONSTRAINT client_portal_users_accepted_has_identity
    CHECK ((state = 'accepted') = (identity_uid IS NOT NULL))
```

`accepted` is exactly the branch `identity_uid IS NOT NULL`. Every other branch is reached only when it is NULL, and the backfill never writes `identity_uid`.

```sql
ALTER TABLE client_portal_users ADD CONSTRAINT client_portal_users_open_invite_has_token
    CHECK ((state IN ('invited', 'refused')) = (invite_token IS NOT NULL))
```

`refused` and `invited` are reached only with `invite_token IS NOT NULL`, and they keep it. `accepted` has it set to NULL. `revoked` and `unlinked` are reached only when it is already NULL.

```sql
ALTER TABLE client_portal_users ADD CONSTRAINT client_portal_users_open_invite_has_expiry
    CHECK ((state IN ('invited', 'refused')) = (invite_token_expires_at IS NOT NULL))
```

For an open invitation it is `COALESCE(invite_token_expires_at, created_at)`. `created_at` is `NOT NULL` (`00006`), so the value is never NULL. For every other state it is NULL.

```sql
ALTER TABLE client_portal_users ADD CONSTRAINT client_portal_users_refusal
    CHECK ((state = 'refused') = (access_refused_at IS NOT NULL))
```

`refused` is reached only with `access_refused_at IS NOT NULL`, and it keeps it. Every other state has it set to NULL.

```sql
ALTER TABLE client_portal_users ADD CONSTRAINT client_portal_users_revocation
    CHECK (
        (state = 'revoked') = (revoked_at IS NOT NULL)
        AND (state = 'revoked') = (revoke_reason IS NOT NULL)
        AND (state = 'revoked' OR revoked_by_staff_id IS NULL)
    )
```

For `revoked`, `revoked_at` is `COALESCE(merged_at, created_at)`, which is never NULL, and `revoke_reason` is a literal. For every other state, all three columns are NULL. They were added in this migration and are written only by this statement.

```sql
ALTER TABLE client_portal_users ADD CONSTRAINT client_portal_users_unlink
    CHECK ((state = 'unlinked') = (unlinked_at IS NOT NULL))
```

For `unlinked` it is `COALESCE(erased_at, created_at)`, which is never NULL. For every other state it is NULL.

## CREATE UNIQUE INDEX

```sql
CREATE UNIQUE INDEX client_portal_users_one_open_invite_per_client
    ON client_portal_users (client_id)
    WHERE state IN ('invited', 'refused')
```

Both open states are reached only when `identity_uid IS NULL`. Until the `DROP INDEX` just before this statement, `client_portal_users_one_pending_per_client` (`00026`) held at most one `identity_uid IS NULL` row per `client_id`. The backfill never changes `identity_uid` or `client_id`. So at most one row per Client is open, and the new index's predicate covers a subset of the rows the old index covered.
