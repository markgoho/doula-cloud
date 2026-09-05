-- +goose Up
-- #617's hand-off from #616: acceptInvite no longer stores an Identity
-- Platform uid anywhere, so client_portal_users.identity_uid always names
-- a Portal Account (#616) from here on, and 'identity_account_delete'
-- (00064) has nothing left to delete -- Clients have no Identity Platform
-- account for client.ErasureWorker to reach. Retired rather than left as
-- dead code queuing a no-op against a uid Identity Platform would report
-- as already-not-found.
--
-- Postgres has no ALTER TYPE ... DROP VALUE, so the enum is rebuilt: any
-- row still carrying the retired value is deleted first (pre-launch, no
-- users, no production data -- CLAUDE.md), then the column is cast
-- through the new type and the old one dropped.
DELETE FROM client_erasure_outbox WHERE act = 'identity_account_delete';

ALTER TYPE client_erasure_act RENAME TO client_erasure_act_old;
CREATE TYPE client_erasure_act AS ENUM ('stripe_customer_delete', 'stripe_redaction_job');
ALTER TABLE client_erasure_outbox
    ALTER COLUMN act TYPE client_erasure_act USING act::text::client_erasure_act;
DROP TYPE client_erasure_act_old;

-- +goose Down
ALTER TYPE client_erasure_act RENAME TO client_erasure_act_new;
CREATE TYPE client_erasure_act AS ENUM
    ('stripe_customer_delete', 'stripe_redaction_job', 'identity_account_delete');
ALTER TABLE client_erasure_outbox
    ALTER COLUMN act TYPE client_erasure_act USING act::text::client_erasure_act;
DROP TYPE client_erasure_act_new;
