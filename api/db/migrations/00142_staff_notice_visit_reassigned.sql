-- +goose Up
-- Two more staff_notice_outbox (00138) kinds, for #2436: an Owner or an
-- Admin moved a Visit onto a Doula, or off her, and she is told. Each
-- subject_id is the visits id.
ALTER TYPE staff_notice_kind ADD VALUE 'visit_reassigned_to';
ALTER TYPE staff_notice_kind ADD VALUE 'visit_reassigned_from';

-- The worker asks, before it sends, whether the Visit is still hers (or
-- still not hers), and which Engagement it is on, for the link. Its
-- transaction sets no app.current_practice_id, so 00007's practice-tier
-- policy shows it no Visit; this is the notification_worker_trusted door
-- 00139 opened on engagement_attachments.
-- +goose StatementBegin
CREATE POLICY visits_notification_worker ON visits
    FOR SELECT
    USING (current_setting('app.notification_worker_trusted', true) = 'true');
-- +goose StatementEnd

-- +goose Down
DROP POLICY visits_notification_worker ON visits;
DELETE FROM staff_notice_outbox WHERE kind IN ('visit_reassigned_to', 'visit_reassigned_from');
ALTER TYPE staff_notice_kind RENAME TO staff_notice_kind_old;
CREATE TYPE staff_notice_kind AS ENUM ('offer_received', 'put_on_engagement');
ALTER TABLE staff_notice_outbox
    ALTER COLUMN kind TYPE staff_notice_kind
    USING kind::text::staff_notice_kind;
DROP TYPE staff_notice_kind_old;
