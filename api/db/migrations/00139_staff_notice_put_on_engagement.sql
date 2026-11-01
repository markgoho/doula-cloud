-- +goose Up
-- A second staff_notice_outbox (00138) kind, for #2403: an Owner or an
-- Admin put a Doula on an Engagement, and she is told. Its subject_id is
-- the engagements id.
ALTER TYPE staff_notice_kind ADD VALUE 'put_on_engagement';

-- The worker asks, before it sends, whether she is still on the
-- Engagement. The worker's transaction sets no app.current_practice_id,
-- so 00030's practice-tier policy shows it no attachment; this is the
-- notification_worker_trusted door 00041 opened on engagement_offers.
-- +goose StatementBegin
CREATE POLICY engagement_attachments_notification_worker ON engagement_attachments
    FOR SELECT
    USING (current_setting('app.notification_worker_trusted', true) = 'true');
-- +goose StatementEnd

-- +goose Down
DROP POLICY engagement_attachments_notification_worker ON engagement_attachments;
DELETE FROM staff_notice_outbox WHERE kind = 'put_on_engagement';
ALTER TYPE staff_notice_kind RENAME TO staff_notice_kind_old;
CREATE TYPE staff_notice_kind AS ENUM ('offer_received');
ALTER TABLE staff_notice_outbox
    ALTER COLUMN kind TYPE staff_notice_kind
    USING kind::text::staff_notice_kind;
DROP TYPE staff_notice_kind_old;
