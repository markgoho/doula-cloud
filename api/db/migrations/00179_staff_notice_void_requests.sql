-- +goose Up
-- Three more staff_notice_outbox (00138) kinds, for #2953: a Doula asked for
-- a Contract to be voided, and each Owner and Admin is told; the Owner or
-- Admin declined it or voided the Contract, and the Doula who asked is
-- told. Each subject_id is the contract_void_requests id.
ALTER TYPE staff_notice_kind ADD VALUE 'void_requested';
ALTER TYPE staff_notice_kind ADD VALUE 'void_declined';
ALTER TYPE staff_notice_kind ADD VALUE 'void_granted';

-- The worker asks, before it sends, whether the request is still open (or
-- how it was decided) and which Engagement it is on, for the link. Its
-- transaction sets no app.current_practice_id, so 00099's practice-tier
-- policy shows it no request; this is the notification_worker_trusted door
-- 00139 opened on engagement_attachments.
-- +goose StatementBegin
CREATE POLICY contract_void_requests_notification_worker ON contract_void_requests
    FOR SELECT
    USING (current_setting('app.notification_worker_trusted', true) = 'true');
-- +goose StatementEnd

-- +goose Down
DROP POLICY contract_void_requests_notification_worker ON contract_void_requests;
DELETE FROM staff_notice_outbox WHERE kind IN ('void_requested', 'void_declined', 'void_granted');
ALTER TYPE staff_notice_kind RENAME TO staff_notice_kind_old;
CREATE TYPE staff_notice_kind AS ENUM ('offer_received', 'put_on_engagement', 'visit_reassigned_to', 'visit_reassigned_from', 'taken_off_engagement');
ALTER TABLE staff_notice_outbox
    ALTER COLUMN kind TYPE staff_notice_kind
    USING kind::text::staff_notice_kind;
DROP TYPE staff_notice_kind_old;
