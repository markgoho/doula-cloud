# 00173_engagement_attachment_state.sql

The migration adds a generated column to `engagement_attachments`, rebuilds its one-open unique index on that column, and adds three CHECKs that rows already in the table could otherwise refuse. The fourth CHECK, `engagement_attachments_ending_has_a_reason`, is added `NOT VALID` and is not reported.

## ADD COLUMN ... GENERATED ALWAYS AS

```sql
ALTER TABLE engagement_attachments ADD COLUMN state attachment_state GENERATED ALWAYS AS (
    CASE
        WHEN ended_at IS NOT NULL THEN 'ended'::attachment_state
        WHEN origin = 'granted' THEN 'granted'::attachment_state
        ELSE 'accrued'::attachment_state
    END
) STORED
```

The expression is total. The first branch tests only `IS NOT NULL` on `ended_at`, the second compares `origin`, which is `NOT NULL` since `00030`, with a label of its own enum, the `ELSE` covers every remaining row, and every result is a literal of the enum created in the same migration. There is no cast of a stored value and no function call that could fail on an existing row.

## ADD COLUMN ... NOT NULL without DEFAULT

```sql
ALTER TABLE engagement_attachments ADD COLUMN state attachment_state GENERATED ALWAYS AS (
    CASE
        WHEN ended_at IS NOT NULL THEN 'ended'::attachment_state
        WHEN origin = 'granted' THEN 'granted'::attachment_state
        ELSE 'accrued'::attachment_state
    END
) STORED
```

The column carries no `NOT NULL` constraint: the words are the expression's own `IS NOT NULL` test. A generated column is computed for every existing row as it is added, and the expression above gives each one a value, so no row is left without one.

## CREATE UNIQUE INDEX

```sql
CREATE UNIQUE INDEX engagement_attachments_one_open
    ON engagement_attachments (engagement_id, staff_id)
    WHERE state <> 'ended'
```

`state <> 'ended'` holds for exactly the rows where `ended_at IS NULL`, by the generation expression's first branch. Those are the rows `00030`'s `engagement_attachments_one_open`, dropped by the statement before this one, already held unique on `(engagement_id, staff_id)`, so no two existing rows can collide under the new index.

## ADD CONSTRAINT ... CHECK

```sql
ALTER TABLE engagement_attachments ADD CONSTRAINT engagement_attachments_ending_is_attributed CHECK (
    (ended_at IS NULL) = (ended_by IS NULL)
)
```

Two statements have ever written `ended_at`, both in `api/internal/staffauth/attach.go`: `EndAttachments` set `ended_at = now(), ended_by = $1` and `EndGranted` set `ended_at = $4, ended_by = $3`, each bound to the acting Staff member's id from the request, which is never empty. Nothing has ever written `ended_by` alone or cleared either column. So every existing row has both or neither.

```sql
ALTER TABLE engagement_attachments ADD CONSTRAINT engagement_attachments_accrued_is_her_own CHECK (
    origin <> 'accrued' OR attached_by = staff_id
)
```

The one writer of an accrued row, `staffauth.attachActor`, inserted `VALUES ($1, $2, 'accrued', $2)`, so `attached_by` is `staff_id`. The only statement that ever changed `attached_by` afterward, `staffauth.Grant`'s `ON CONFLICT ... DO UPDATE`, sets `origin = 'granted'` in the same `SET`, so no row is left accrued with another author.

```sql
ALTER TABLE engagement_attachments ADD CONSTRAINT engagement_attachments_fee_only_when_granted CHECK (
    origin = 'granted' OR (fee_amount_cents IS NULL AND fee_terms IS NULL)
)
```

`attachActor`'s accrued insert names neither fee column. The only statement that ever wrote either, `staffauth.Grant`, either inserted the row as `'granted'` or, on conflict with an open accrued row, set `origin = 'granted'` in the same `SET` that copied the fee on. So no existing row carries a fee without being granted.
