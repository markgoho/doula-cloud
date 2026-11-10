# 00169_outbox_lifecycle.sql

Every row-dependent statement here is either an UPDATE that only moves rows toward the constraint that follows it, or a constraint or cast that the rows existing writers can have produced already satisfy. Each block below is proved against the writers in `api/internal/outbox` and the packages that call it.

## ALTER COLUMN ... TYPE

```sql
ALTER TABLE welcome_credits_outbox ALTER COLUMN status TYPE welcome_credits_outbox_status USING status::text::welcome_credits_outbox_status
```

Every label of `welcome_credits_outbox_status_old` (`pending`, `sent`, `dead_lettered`, 00133) is a label of the new type, so the cast succeeds for every value an existing row can hold, and the column is `NOT NULL` so there is no NULL to cast. The partial index and the CHECK that name the old type are dropped just above, so nothing else depends on it, and both are made again just below in the same transaction.

```sql
ALTER TABLE welcome_credits_outbox ALTER COLUMN skip_reason TYPE outbox_skip_reason
    USING (CASE skip_reason WHEN 'no_owners' THEN 'no_recipients' ELSE skip_reason END)::outbox_skip_reason
```

The CHECK just dropped (00133) held `skip_reason` to NULL or one of `engagement_started`, `founding_grant`, `practice_deleting`, `owners_suppressed`, `no_owners`. The CASE maps `no_owners` to `no_recipients`, and the other four are labels of `outbox_skip_reason` as they stand; NULL casts to NULL. So the cast succeeds for every value an existing row can hold.

## DML (UPDATE / DELETE / INSERT)

```sql
UPDATE welcome_credits_outbox SET status = 'skipped', skipped_at = sent_at, sent_at = NULL
 WHERE status = 'sent' AND skip_reason IS NOT NULL
```

An UPDATE that matches no row succeeds, so an empty table cannot break it. On a populated one it moves only rows the worker marked sent with a skip reason (welcomecredits.markSkipped, the only writer of `skip_reason`), to a label the type created above holds, so no value it writes can be refused by the column's type. Every such row was stamped `sent_at` by the same `MarkSent` call, so `skipped_at` gets a time. No constraint on the table yet reads `skipped_at` or `skip_reason`: the shape CHECK below is added after it.

```sql
UPDATE client_erasure_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL
```

An UPDATE that matches no row succeeds. On a populated table it writes one label of `outbox_dead_letter_reason` (00152) into rows that are `dead_lettered` and have none, which is exactly what makes the 00152 CHECK, `(status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)`, hold for them; no other constraint on the table reads the column.

```sql
UPDATE connect_nudge_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL
```

An UPDATE that matches no row succeeds. On a populated table it writes one label of `outbox_dead_letter_reason` (00152) into rows that are `dead_lettered` and have none, which is exactly what makes the 00152 CHECK, `(status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)`, hold for them; no other constraint on the table reads the column.

```sql
UPDATE coverage_gap_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL
```

An UPDATE that matches no row succeeds. On a populated table it writes one label of `outbox_dead_letter_reason` (00152) into rows that are `dead_lettered` and have none, which is exactly what makes the 00152 CHECK, `(status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)`, hold for them; no other constraint on the table reads the column.

```sql
UPDATE engagement_offer_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL
```

An UPDATE that matches no row succeeds. On a populated table it writes one label of `outbox_dead_letter_reason` (00152) into rows that are `dead_lettered` and have none, which is exactly what makes the 00152 CHECK, `(status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)`, hold for them; no other constraint on the table reads the column.

```sql
UPDATE engagement_request_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL
```

An UPDATE that matches no row succeeds. On a populated table it writes one label of `outbox_dead_letter_reason` (00152) into rows that are `dead_lettered` and have none, which is exactly what makes the 00152 CHECK, `(status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)`, hold for them; no other constraint on the table reads the column.

```sql
UPDATE feedback_issue_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL
```

An UPDATE that matches no row succeeds. On a populated table it writes one label of `outbox_dead_letter_reason` (00152) into rows that are `dead_lettered` and have none, which is exactly what makes the 00152 CHECK, `(status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)`, hold for them; no other constraint on the table reads the column.

```sql
UPDATE legal_notice_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL
```

An UPDATE that matches no row succeeds. On a populated table it writes one label of `outbox_dead_letter_reason` (00152) into rows that are `dead_lettered` and have none, which is exactly what makes the 00152 CHECK, `(status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)`, hold for them; no other constraint on the table reads the column.

```sql
UPDATE low_credit_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL
```

An UPDATE that matches no row succeeds. On a populated table it writes one label of `outbox_dead_letter_reason` (00152) into rows that are `dead_lettered` and have none, which is exactly what makes the 00152 CHECK, `(status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)`, hold for them; no other constraint on the table reads the column.

```sql
UPDATE payment_received_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL
```

An UPDATE that matches no row succeeds. On a populated table it writes one label of `outbox_dead_letter_reason` (00152) into rows that are `dead_lettered` and have none, which is exactly what makes the 00152 CHECK, `(status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)`, hold for them; no other constraint on the table reads the column.

```sql
UPDATE payout_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL
```

An UPDATE that matches no row succeeds. On a populated table it writes one label of `outbox_dead_letter_reason` (00152) into rows that are `dead_lettered` and have none, which is exactly what makes the 00152 CHECK, `(status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)`, hold for them; no other constraint on the table reads the column.

```sql
UPDATE portal_address_change_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL
```

An UPDATE that matches no row succeeds. On a populated table it writes one label of `outbox_dead_letter_reason` (00152) into rows that are `dead_lettered` and have none, which is exactly what makes the 00152 CHECK, `(status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)`, hold for them; no other constraint on the table reads the column.

```sql
UPDATE portal_invite_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL
```

An UPDATE that matches no row succeeds. On a populated table it writes one label of `outbox_dead_letter_reason` (00152) into rows that are `dead_lettered` and have none, which is exactly what makes the 00152 CHECK, `(status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)`, hold for them; no other constraint on the table reads the column.

```sql
UPDATE portal_magic_link_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL
```

An UPDATE that matches no row succeeds. On a populated table it writes one label of `outbox_dead_letter_reason` (00152) into rows that are `dead_lettered` and have none, which is exactly what makes the 00152 CHECK, `(status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)`, hold for them; no other constraint on the table reads the column.

```sql
UPDATE practice_deletion_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL
```

An UPDATE that matches no row succeeds. On a populated table it writes one label of `outbox_dead_letter_reason` (00152) into rows that are `dead_lettered` and have none, which is exactly what makes the 00152 CHECK, `(status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)`, hold for them; no other constraint on the table reads the column.

```sql
UPDATE session_notice_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL
```

An UPDATE that matches no row succeeds. On a populated table it writes one label of `outbox_dead_letter_reason` (00152) into rows that are `dead_lettered` and have none, which is exactly what makes the 00152 CHECK, `(status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)`, hold for them; no other constraint on the table reads the column.

```sql
UPDATE site_build_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL
```

An UPDATE that matches no row succeeds. On a populated table it writes one label of `outbox_dead_letter_reason` (00152) into rows that are `dead_lettered` and have none, which is exactly what makes the 00152 CHECK, `(status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)`, hold for them; no other constraint on the table reads the column.

```sql
UPDATE staff_email_change_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL
```

An UPDATE that matches no row succeeds. On a populated table it writes one label of `outbox_dead_letter_reason` (00152) into rows that are `dead_lettered` and have none, which is exactly what makes the 00152 CHECK, `(status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)`, hold for them; no other constraint on the table reads the column.

```sql
UPDATE staff_invite_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL
```

An UPDATE that matches no row succeeds. On a populated table it writes one label of `outbox_dead_letter_reason` (00152) into rows that are `dead_lettered` and have none, which is exactly what makes the 00152 CHECK, `(status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)`, hold for them; no other constraint on the table reads the column.

```sql
UPDATE staff_mfa_recovery_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL
```

An UPDATE that matches no row succeeds. On a populated table it writes one label of `outbox_dead_letter_reason` (00152) into rows that are `dead_lettered` and have none, which is exactly what makes the 00152 CHECK, `(status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)`, hold for them; no other constraint on the table reads the column.

```sql
UPDATE staff_notice_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL
```

An UPDATE that matches no row succeeds. On a populated table it writes one label of `outbox_dead_letter_reason` (00152) into rows that are `dead_lettered` and have none, which is exactly what makes the 00152 CHECK, `(status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)`, hold for them; no other constraint on the table reads the column.

```sql
UPDATE staff_token_mail_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL
```

An UPDATE that matches no row succeeds. On a populated table it writes one label of `outbox_dead_letter_reason` (00152) into rows that are `dead_lettered` and have none, which is exactly what makes the 00152 CHECK, `(status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)`, hold for them; no other constraint on the table reads the column.

```sql
UPDATE welcome_credits_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL
```

An UPDATE that matches no row succeeds. On a populated table it writes one label of `outbox_dead_letter_reason` (00152) into rows that are `dead_lettered` and have none, which is exactly what makes the 00152 CHECK, `(status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)`, hold for them; no other constraint on the table reads the column.

```sql
UPDATE engagement_offer_outbox SET invite_token = NULL, access_code = NULL
 WHERE status <> 'pending' AND (invite_token IS NOT NULL OR access_code IS NOT NULL)
```

An UPDATE that matches no row succeeds. On a populated table it nulls a secret column, which is nullable (every terminal transition has always nulled it), on rows that are no longer pending; no constraint on the table yet reads the column, and the CHECK below is added after it.

```sql
UPDATE portal_address_change_outbox SET token = NULL
 WHERE status <> 'pending' AND (token IS NOT NULL)
```

An UPDATE that matches no row succeeds. On a populated table it nulls a secret column, which is nullable (every terminal transition has always nulled it), on rows that are no longer pending; no constraint on the table yet reads the column, and the CHECK below is added after it.

```sql
UPDATE portal_magic_link_outbox SET token = NULL
 WHERE status <> 'pending' AND (token IS NOT NULL)
```

An UPDATE that matches no row succeeds. On a populated table it nulls a secret column, which is nullable (every terminal transition has always nulled it), on rows that are no longer pending; no constraint on the table yet reads the column, and the CHECK below is added after it.

```sql
UPDATE staff_invite_outbox SET invite_token = NULL
 WHERE status <> 'pending' AND (invite_token IS NOT NULL)
```

An UPDATE that matches no row succeeds. On a populated table it nulls a secret column, which is nullable (every terminal transition has always nulled it), on rows that are no longer pending; no constraint on the table yet reads the column, and the CHECK below is added after it.

```sql
UPDATE staff_mfa_recovery_outbox SET token = NULL
 WHERE status <> 'pending' AND (token IS NOT NULL)
```

An UPDATE that matches no row succeeds. On a populated table it nulls a secret column, which is nullable (every terminal transition has always nulled it), on rows that are no longer pending; no constraint on the table yet reads the column, and the CHECK below is added after it.

```sql
UPDATE staff_token_mail_outbox SET token = NULL
 WHERE status <> 'pending' AND (token IS NOT NULL)
```

An UPDATE that matches no row succeeds. On a populated table it nulls a secret column, which is nullable (every terminal transition has always nulled it), on rows that are no longer pending; no constraint on the table yet reads the column, and the CHECK below is added after it.

## VALIDATE CONSTRAINT

```sql
ALTER TABLE client_erasure_outbox VALIDATE CONSTRAINT client_erasure_outbox_dead_letter_reason_check
```

00152's CHECK has held every row written since it was added; a row older than it can only fail it by being `dead_lettered` with no reason, and the backfill just above gave every such row one. A non-dead-lettered row has no reason: none existed before 00152 added the column, and the CHECK refused one since.

```sql
ALTER TABLE connect_nudge_outbox VALIDATE CONSTRAINT connect_nudge_outbox_dead_letter_reason_check
```

00152's CHECK has held every row written since it was added; a row older than it can only fail it by being `dead_lettered` with no reason, and the backfill just above gave every such row one. A non-dead-lettered row has no reason: none existed before 00152 added the column, and the CHECK refused one since.

```sql
ALTER TABLE coverage_gap_outbox VALIDATE CONSTRAINT coverage_gap_outbox_dead_letter_reason_check
```

00152's CHECK has held every row written since it was added; a row older than it can only fail it by being `dead_lettered` with no reason, and the backfill just above gave every such row one. A non-dead-lettered row has no reason: none existed before 00152 added the column, and the CHECK refused one since.

```sql
ALTER TABLE engagement_offer_outbox VALIDATE CONSTRAINT engagement_offer_outbox_dead_letter_reason_check
```

00152's CHECK has held every row written since it was added; a row older than it can only fail it by being `dead_lettered` with no reason, and the backfill just above gave every such row one. A non-dead-lettered row has no reason: none existed before 00152 added the column, and the CHECK refused one since.

```sql
ALTER TABLE engagement_request_outbox VALIDATE CONSTRAINT engagement_request_outbox_dead_letter_reason_check
```

00152's CHECK has held every row written since it was added; a row older than it can only fail it by being `dead_lettered` with no reason, and the backfill just above gave every such row one. A non-dead-lettered row has no reason: none existed before 00152 added the column, and the CHECK refused one since.

```sql
ALTER TABLE feedback_issue_outbox VALIDATE CONSTRAINT feedback_issue_outbox_dead_letter_reason_check
```

00152's CHECK has held every row written since it was added; a row older than it can only fail it by being `dead_lettered` with no reason, and the backfill just above gave every such row one. A non-dead-lettered row has no reason: none existed before 00152 added the column, and the CHECK refused one since.

```sql
ALTER TABLE legal_notice_outbox VALIDATE CONSTRAINT legal_notice_outbox_dead_letter_reason_check
```

00152's CHECK has held every row written since it was added; a row older than it can only fail it by being `dead_lettered` with no reason, and the backfill just above gave every such row one. A non-dead-lettered row has no reason: none existed before 00152 added the column, and the CHECK refused one since.

```sql
ALTER TABLE low_credit_outbox VALIDATE CONSTRAINT low_credit_outbox_dead_letter_reason_check
```

00152's CHECK has held every row written since it was added; a row older than it can only fail it by being `dead_lettered` with no reason, and the backfill just above gave every such row one. A non-dead-lettered row has no reason: none existed before 00152 added the column, and the CHECK refused one since.

```sql
ALTER TABLE payment_received_outbox VALIDATE CONSTRAINT payment_received_outbox_dead_letter_reason_check
```

00152's CHECK has held every row written since it was added; a row older than it can only fail it by being `dead_lettered` with no reason, and the backfill just above gave every such row one. A non-dead-lettered row has no reason: none existed before 00152 added the column, and the CHECK refused one since.

```sql
ALTER TABLE payout_outbox VALIDATE CONSTRAINT payout_outbox_dead_letter_reason_check
```

00152's CHECK has held every row written since it was added; a row older than it can only fail it by being `dead_lettered` with no reason, and the backfill just above gave every such row one. A non-dead-lettered row has no reason: none existed before 00152 added the column, and the CHECK refused one since.

```sql
ALTER TABLE portal_address_change_outbox VALIDATE CONSTRAINT portal_address_change_outbox_dead_letter_reason_check
```

00152's CHECK has held every row written since it was added; a row older than it can only fail it by being `dead_lettered` with no reason, and the backfill just above gave every such row one. A non-dead-lettered row has no reason: none existed before 00152 added the column, and the CHECK refused one since.

```sql
ALTER TABLE portal_invite_outbox VALIDATE CONSTRAINT portal_invite_outbox_dead_letter_reason_check
```

00152's CHECK has held every row written since it was added; a row older than it can only fail it by being `dead_lettered` with no reason, and the backfill just above gave every such row one. A non-dead-lettered row has no reason: none existed before 00152 added the column, and the CHECK refused one since.

```sql
ALTER TABLE portal_magic_link_outbox VALIDATE CONSTRAINT portal_magic_link_outbox_dead_letter_reason_check
```

00152's CHECK has held every row written since it was added; a row older than it can only fail it by being `dead_lettered` with no reason, and the backfill just above gave every such row one. A non-dead-lettered row has no reason: none existed before 00152 added the column, and the CHECK refused one since.

```sql
ALTER TABLE practice_deletion_outbox VALIDATE CONSTRAINT practice_deletion_outbox_dead_letter_reason_check
```

00152's CHECK has held every row written since it was added; a row older than it can only fail it by being `dead_lettered` with no reason, and the backfill just above gave every such row one. A non-dead-lettered row has no reason: none existed before 00152 added the column, and the CHECK refused one since.

```sql
ALTER TABLE session_notice_outbox VALIDATE CONSTRAINT session_notice_outbox_dead_letter_reason_check
```

00152's CHECK has held every row written since it was added; a row older than it can only fail it by being `dead_lettered` with no reason, and the backfill just above gave every such row one. A non-dead-lettered row has no reason: none existed before 00152 added the column, and the CHECK refused one since.

```sql
ALTER TABLE site_build_outbox VALIDATE CONSTRAINT site_build_outbox_dead_letter_reason_check
```

00152's CHECK has held every row written since it was added; a row older than it can only fail it by being `dead_lettered` with no reason, and the backfill just above gave every such row one. A non-dead-lettered row has no reason: none existed before 00152 added the column, and the CHECK refused one since.

```sql
ALTER TABLE staff_email_change_outbox VALIDATE CONSTRAINT staff_email_change_outbox_dead_letter_reason_check
```

00152's CHECK has held every row written since it was added; a row older than it can only fail it by being `dead_lettered` with no reason, and the backfill just above gave every such row one. A non-dead-lettered row has no reason: none existed before 00152 added the column, and the CHECK refused one since.

```sql
ALTER TABLE staff_invite_outbox VALIDATE CONSTRAINT staff_invite_outbox_dead_letter_reason_check
```

00152's CHECK has held every row written since it was added; a row older than it can only fail it by being `dead_lettered` with no reason, and the backfill just above gave every such row one. A non-dead-lettered row has no reason: none existed before 00152 added the column, and the CHECK refused one since.

```sql
ALTER TABLE staff_mfa_recovery_outbox VALIDATE CONSTRAINT staff_mfa_recovery_outbox_dead_letter_reason_check
```

00152's CHECK has held every row written since it was added; a row older than it can only fail it by being `dead_lettered` with no reason, and the backfill just above gave every such row one. A non-dead-lettered row has no reason: none existed before 00152 added the column, and the CHECK refused one since.

```sql
ALTER TABLE staff_notice_outbox VALIDATE CONSTRAINT staff_notice_outbox_dead_letter_reason_check
```

00152's CHECK has held every row written since it was added; a row older than it can only fail it by being `dead_lettered` with no reason, and the backfill just above gave every such row one. A non-dead-lettered row has no reason: none existed before 00152 added the column, and the CHECK refused one since.

```sql
ALTER TABLE staff_token_mail_outbox VALIDATE CONSTRAINT staff_token_mail_outbox_dead_letter_reason_check
```

00152's CHECK has held every row written since it was added; a row older than it can only fail it by being `dead_lettered` with no reason, and the backfill just above gave every such row one. A non-dead-lettered row has no reason: none existed before 00152 added the column, and the CHECK refused one since.

## ADD CONSTRAINT ... CHECK

```sql
ALTER TABLE welcome_credits_outbox ADD CONSTRAINT welcome_credits_outbox_dead_letter_reason_check
    CHECK ((status = 'dead_lettered') = (dead_letter_reason IS NOT NULL))
```

The same predicate 00152 added NOT VALID, made again because its column changed type above. The backfill just above gave every `dead_lettered` row a reason, and no writer has ever set a reason on a row in any other state (00152's own CHECK held every row written since), so no row can violate it.

```sql
ALTER TABLE engagement_offer_outbox ADD CONSTRAINT engagement_offer_outbox_secret_cleared
    CHECK (status = 'pending' OR (invite_token IS NULL AND access_code IS NULL))
```

The UPDATE just above nulled the column on every row that is not pending, so every existing row is either pending or holds no secret.

```sql
ALTER TABLE portal_address_change_outbox ADD CONSTRAINT portal_address_change_outbox_secret_cleared
    CHECK (status = 'pending' OR (token IS NULL))
```

The UPDATE just above nulled the column on every row that is not pending, so every existing row is either pending or holds no secret.

```sql
ALTER TABLE portal_magic_link_outbox ADD CONSTRAINT portal_magic_link_outbox_secret_cleared
    CHECK (status = 'pending' OR (token IS NULL))
```

The UPDATE just above nulled the column on every row that is not pending, so every existing row is either pending or holds no secret.

```sql
ALTER TABLE staff_invite_outbox ADD CONSTRAINT staff_invite_outbox_secret_cleared
    CHECK (status = 'pending' OR (invite_token IS NULL))
```

The UPDATE just above nulled the column on every row that is not pending, so every existing row is either pending or holds no secret.

```sql
ALTER TABLE staff_mfa_recovery_outbox ADD CONSTRAINT staff_mfa_recovery_outbox_secret_cleared
    CHECK (status = 'pending' OR (token IS NULL))
```

The UPDATE just above nulled the column on every row that is not pending, so every existing row is either pending or holds no secret.

```sql
ALTER TABLE staff_token_mail_outbox ADD CONSTRAINT staff_token_mail_outbox_secret_cleared
    CHECK (status = 'pending' OR (token IS NULL))
```

The UPDATE just above nulled the column on every row that is not pending, so every existing row is either pending or holds no secret.

```sql
ALTER TABLE client_erasure_outbox ADD CONSTRAINT client_erasure_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
)
```

No row is `skipped`, the label being new in this migration, and `skip_reason` and `skipped_at` are new and NULL on every row, so the last two clauses hold on every existing row. For `sent_at`: every writer that has ever set this table's status to `sent` set `sent_at` in the same statement (`outbox.Worker.MarkSent`, and the login deletion's own UPDATE it replaces), no writer sets `sent_at` on any other status, and no path returns a row to `pending` from another state (every re-enqueue's `ON CONFLICT ... DO UPDATE` is on the partial index `WHERE status = 'pending'`).

```sql
ALTER TABLE connect_nudge_outbox ADD CONSTRAINT connect_nudge_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
)
```

No row is `skipped`, the label being new in this migration, and `skip_reason` and `skipped_at` are new and NULL on every row, so the last two clauses hold on every existing row. For `sent_at`: every writer that has ever set this table's status to `sent` set `sent_at` in the same statement (`outbox.Worker.MarkSent`, and the login deletion's own UPDATE it replaces), no writer sets `sent_at` on any other status, and no path returns a row to `pending` from another state (every re-enqueue's `ON CONFLICT ... DO UPDATE` is on the partial index `WHERE status = 'pending'`).

```sql
ALTER TABLE coverage_gap_outbox ADD CONSTRAINT coverage_gap_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
)
```

No row is `skipped`, the label being new in this migration, and `skip_reason` and `skipped_at` are new and NULL on every row, so the last two clauses hold on every existing row. For `sent_at`: every writer that has ever set this table's status to `sent` set `sent_at` in the same statement (`outbox.Worker.MarkSent`, and the login deletion's own UPDATE it replaces), no writer sets `sent_at` on any other status, and no path returns a row to `pending` from another state (every re-enqueue's `ON CONFLICT ... DO UPDATE` is on the partial index `WHERE status = 'pending'`).

```sql
ALTER TABLE engagement_offer_outbox ADD CONSTRAINT engagement_offer_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
)
```

No row is `skipped`, the label being new in this migration, and `skip_reason` and `skipped_at` are new and NULL on every row, so the last two clauses hold on every existing row. For `sent_at`: every writer that has ever set this table's status to `sent` set `sent_at` in the same statement (`outbox.Worker.MarkSent`, and the login deletion's own UPDATE it replaces), no writer sets `sent_at` on any other status, and no path returns a row to `pending` from another state (every re-enqueue's `ON CONFLICT ... DO UPDATE` is on the partial index `WHERE status = 'pending'`).

```sql
ALTER TABLE engagement_request_outbox ADD CONSTRAINT engagement_request_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
)
```

No row is `skipped`, the label being new in this migration, and `skip_reason` and `skipped_at` are new and NULL on every row, so the last two clauses hold on every existing row. For `sent_at`: every writer that has ever set this table's status to `sent` set `sent_at` in the same statement (`outbox.Worker.MarkSent`, and the login deletion's own UPDATE it replaces), no writer sets `sent_at` on any other status, and no path returns a row to `pending` from another state (every re-enqueue's `ON CONFLICT ... DO UPDATE` is on the partial index `WHERE status = 'pending'`).

```sql
ALTER TABLE feedback_issue_outbox ADD CONSTRAINT feedback_issue_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
)
```

No row is `skipped`, the label being new in this migration, and `skip_reason` and `skipped_at` are new and NULL on every row, so the last two clauses hold on every existing row. For `sent_at`: every writer that has ever set this table's status to `sent` set `sent_at` in the same statement (`outbox.Worker.MarkSent`, and the login deletion's own UPDATE it replaces), no writer sets `sent_at` on any other status, and no path returns a row to `pending` from another state (every re-enqueue's `ON CONFLICT ... DO UPDATE` is on the partial index `WHERE status = 'pending'`).

```sql
ALTER TABLE legal_notice_outbox ADD CONSTRAINT legal_notice_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
)
```

No row is `skipped`, the label being new in this migration, and `skip_reason` and `skipped_at` are new and NULL on every row, so the last two clauses hold on every existing row. For `sent_at`: every writer that has ever set this table's status to `sent` set `sent_at` in the same statement (`outbox.Worker.MarkSent`, and the login deletion's own UPDATE it replaces), no writer sets `sent_at` on any other status, and no path returns a row to `pending` from another state (every re-enqueue's `ON CONFLICT ... DO UPDATE` is on the partial index `WHERE status = 'pending'`).

```sql
ALTER TABLE low_credit_outbox ADD CONSTRAINT low_credit_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
)
```

No row is `skipped`, the label being new in this migration, and `skip_reason` and `skipped_at` are new and NULL on every row, so the last two clauses hold on every existing row. For `sent_at`: every writer that has ever set this table's status to `sent` set `sent_at` in the same statement (`outbox.Worker.MarkSent`, and the login deletion's own UPDATE it replaces), no writer sets `sent_at` on any other status, and no path returns a row to `pending` from another state (every re-enqueue's `ON CONFLICT ... DO UPDATE` is on the partial index `WHERE status = 'pending'`).

```sql
ALTER TABLE payment_received_outbox ADD CONSTRAINT payment_received_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
)
```

No row is `skipped`, the label being new in this migration, and `skip_reason` and `skipped_at` are new and NULL on every row, so the last two clauses hold on every existing row. For `sent_at`: every writer that has ever set this table's status to `sent` set `sent_at` in the same statement (`outbox.Worker.MarkSent`, and the login deletion's own UPDATE it replaces), no writer sets `sent_at` on any other status, and no path returns a row to `pending` from another state (every re-enqueue's `ON CONFLICT ... DO UPDATE` is on the partial index `WHERE status = 'pending'`).

```sql
ALTER TABLE payout_outbox ADD CONSTRAINT payout_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
)
```

No row is `skipped`, the label being new in this migration, and `skip_reason` and `skipped_at` are new and NULL on every row, so the last two clauses hold on every existing row. For `sent_at`: every writer that has ever set this table's status to `sent` set `sent_at` in the same statement (`outbox.Worker.MarkSent`, and the login deletion's own UPDATE it replaces), no writer sets `sent_at` on any other status, and no path returns a row to `pending` from another state (every re-enqueue's `ON CONFLICT ... DO UPDATE` is on the partial index `WHERE status = 'pending'`).

```sql
ALTER TABLE portal_address_change_outbox ADD CONSTRAINT portal_address_change_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
)
```

No row is `skipped`, the label being new in this migration, and `skip_reason` and `skipped_at` are new and NULL on every row, so the last two clauses hold on every existing row. For `sent_at`: every writer that has ever set this table's status to `sent` set `sent_at` in the same statement (`outbox.Worker.MarkSent`, and the login deletion's own UPDATE it replaces), no writer sets `sent_at` on any other status, and no path returns a row to `pending` from another state (every re-enqueue's `ON CONFLICT ... DO UPDATE` is on the partial index `WHERE status = 'pending'`).

```sql
ALTER TABLE portal_invite_outbox ADD CONSTRAINT portal_invite_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status IN ('sent', 'bounced', 'complained'))
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
)
```

No row is `skipped`, the label being new in this migration, and `skip_reason` and `skipped_at` are new and NULL on every row, so the last two clauses hold on every existing row. For `sent_at`: every writer that has ever set this table's status to `sent` (or moved a sent row to `bounced`/`complained`, which leaves `sent_at` as it was) set `sent_at` in the same statement (`outbox.Worker.MarkSent`, and the login deletion's own UPDATE it replaces), no writer sets `sent_at` on any other status, and no path returns a row to `pending` from another state (every re-enqueue's `ON CONFLICT ... DO UPDATE` is on the partial index `WHERE status = 'pending'`).

```sql
ALTER TABLE portal_magic_link_outbox ADD CONSTRAINT portal_magic_link_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
)
```

No row is `skipped`, the label being new in this migration, and `skip_reason` and `skipped_at` are new and NULL on every row, so the last two clauses hold on every existing row. For `sent_at`: every writer that has ever set this table's status to `sent` set `sent_at` in the same statement (`outbox.Worker.MarkSent`, and the login deletion's own UPDATE it replaces), no writer sets `sent_at` on any other status, and no path returns a row to `pending` from another state (every re-enqueue's `ON CONFLICT ... DO UPDATE` is on the partial index `WHERE status = 'pending'`).

```sql
ALTER TABLE practice_deletion_outbox ADD CONSTRAINT practice_deletion_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
)
```

No row is `skipped`, the label being new in this migration, and `skip_reason` and `skipped_at` are new and NULL on every row, so the last two clauses hold on every existing row. For `sent_at`: every writer that has ever set this table's status to `sent` set `sent_at` in the same statement (`outbox.Worker.MarkSent`, and the login deletion's own UPDATE it replaces), no writer sets `sent_at` on any other status, and no path returns a row to `pending` from another state (every re-enqueue's `ON CONFLICT ... DO UPDATE` is on the partial index `WHERE status = 'pending'`).

```sql
ALTER TABLE session_notice_outbox ADD CONSTRAINT session_notice_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
)
```

No row is `skipped`, the label being new in this migration, and `skip_reason` and `skipped_at` are new and NULL on every row, so the last two clauses hold on every existing row. For `sent_at`: every writer that has ever set this table's status to `sent` set `sent_at` in the same statement (`outbox.Worker.MarkSent`, and the login deletion's own UPDATE it replaces), no writer sets `sent_at` on any other status, and no path returns a row to `pending` from another state (every re-enqueue's `ON CONFLICT ... DO UPDATE` is on the partial index `WHERE status = 'pending'`).

```sql
ALTER TABLE staff_email_change_outbox ADD CONSTRAINT staff_email_change_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
)
```

No row is `skipped`, the label being new in this migration, and `skip_reason` and `skipped_at` are new and NULL on every row, so the last two clauses hold on every existing row. For `sent_at`: every writer that has ever set this table's status to `sent` set `sent_at` in the same statement (`outbox.Worker.MarkSent`, and the login deletion's own UPDATE it replaces), no writer sets `sent_at` on any other status, and no path returns a row to `pending` from another state (every re-enqueue's `ON CONFLICT ... DO UPDATE` is on the partial index `WHERE status = 'pending'`).

```sql
ALTER TABLE staff_invite_outbox ADD CONSTRAINT staff_invite_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
)
```

No row is `skipped`, the label being new in this migration, and `skip_reason` and `skipped_at` are new and NULL on every row, so the last two clauses hold on every existing row. For `sent_at`: every writer that has ever set this table's status to `sent` set `sent_at` in the same statement (`outbox.Worker.MarkSent`, and the login deletion's own UPDATE it replaces), no writer sets `sent_at` on any other status, and no path returns a row to `pending` from another state (every re-enqueue's `ON CONFLICT ... DO UPDATE` is on the partial index `WHERE status = 'pending'`).

```sql
ALTER TABLE staff_mfa_recovery_outbox ADD CONSTRAINT staff_mfa_recovery_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
)
```

No row is `skipped`, the label being new in this migration, and `skip_reason` and `skipped_at` are new and NULL on every row, so the last two clauses hold on every existing row. For `sent_at`: every writer that has ever set this table's status to `sent` set `sent_at` in the same statement (`outbox.Worker.MarkSent`, and the login deletion's own UPDATE it replaces), no writer sets `sent_at` on any other status, and no path returns a row to `pending` from another state (every re-enqueue's `ON CONFLICT ... DO UPDATE` is on the partial index `WHERE status = 'pending'`).

```sql
ALTER TABLE staff_notice_outbox ADD CONSTRAINT staff_notice_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
)
```

No row is `skipped`, the label being new in this migration, and `skip_reason` and `skipped_at` are new and NULL on every row, so the last two clauses hold on every existing row. For `sent_at`: every writer that has ever set this table's status to `sent` set `sent_at` in the same statement (`outbox.Worker.MarkSent`, and the login deletion's own UPDATE it replaces), no writer sets `sent_at` on any other status, and no path returns a row to `pending` from another state (every re-enqueue's `ON CONFLICT ... DO UPDATE` is on the partial index `WHERE status = 'pending'`).

```sql
ALTER TABLE staff_token_mail_outbox ADD CONSTRAINT staff_token_mail_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
)
```

No row is `skipped`, the label being new in this migration, and `skip_reason` and `skipped_at` are new and NULL on every row, so the last two clauses hold on every existing row. For `sent_at`: every writer that has ever set this table's status to `sent` set `sent_at` in the same statement (`outbox.Worker.MarkSent`, and the login deletion's own UPDATE it replaces), no writer sets `sent_at` on any other status, and no path returns a row to `pending` from another state (every re-enqueue's `ON CONFLICT ... DO UPDATE` is on the partial index `WHERE status = 'pending'`).

```sql
ALTER TABLE welcome_credits_outbox ADD CONSTRAINT welcome_credits_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
)
```

No row is `skipped` yet except those the welcome backfill above moved, which it gave both a `skip_reason` and a `skipped_at`; every other row has both NULL, the columns being new in this migration. For `sent_at`: every writer that has ever set this table's status to `sent` set `sent_at` in the same statement (`outbox.Worker.MarkSent`, and the login deletion's own UPDATE it replaces), no writer sets `sent_at` on any other status, and no path returns a row to `pending` from another state (every re-enqueue's `ON CONFLICT ... DO UPDATE` is on the partial index `WHERE status = 'pending'`), and the welcome backfill above cleared `sent_at` on the rows it moved off `sent`.

```sql
ALTER TABLE site_build_outbox ADD CONSTRAINT site_build_outbox_dispatched_at_check
    CHECK ((status = 'dispatched') = (dispatched_at IS NOT NULL))
```

`sitebuild.Worker.ProcessPending` is the only writer of `dispatched` and sets `dispatched_at` in the same statement; `recordFailure`, the only other status writer, touches only pending rows, which have no `dispatched_at`, and nothing moves a row out of `dispatched`.
