-- +goose Up
-- One more staff_notice_outbox (00138) kind, for #2596: an Owner or an
-- Admin took a Doula off an Engagement, and she is told. Its subject_id is
-- the engagements id. The worker's reads are 00139's engagement_attachments
-- door, so no policy is added.
ALTER TYPE staff_notice_kind ADD VALUE 'taken_off_engagement';

-- +goose Down
DELETE FROM staff_notice_outbox WHERE kind = 'taken_off_engagement';
ALTER TYPE staff_notice_kind RENAME TO staff_notice_kind_old;
CREATE TYPE staff_notice_kind AS ENUM ('offer_received', 'put_on_engagement', 'visit_reassigned_to', 'visit_reassigned_from');
ALTER TABLE staff_notice_outbox
    ALTER COLUMN kind TYPE staff_notice_kind
    USING kind::text::staff_notice_kind;
DROP TYPE staff_notice_kind_old;
