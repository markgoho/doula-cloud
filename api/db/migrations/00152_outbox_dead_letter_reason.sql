-- +goose Up
-- #2474, ADR-0062: a dead-lettered row records why. retries_exhausted is
-- the alert's; own_data (the row's own data meant no send could succeed)
-- is only shown. No live data, so no backfill: the CHECK is NOT VALID and
-- holds every row written from here on.
CREATE TYPE outbox_dead_letter_reason AS ENUM ('retries_exhausted', 'own_data');

ALTER TABLE client_erasure_outbox ADD COLUMN dead_letter_reason outbox_dead_letter_reason;
ALTER TABLE client_erasure_outbox ADD CONSTRAINT client_erasure_outbox_dead_letter_reason_check
    CHECK ((status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)) NOT VALID;

ALTER TABLE connect_nudge_outbox ADD COLUMN dead_letter_reason outbox_dead_letter_reason;
ALTER TABLE connect_nudge_outbox ADD CONSTRAINT connect_nudge_outbox_dead_letter_reason_check
    CHECK ((status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)) NOT VALID;

ALTER TABLE coverage_gap_outbox ADD COLUMN dead_letter_reason outbox_dead_letter_reason;
ALTER TABLE coverage_gap_outbox ADD CONSTRAINT coverage_gap_outbox_dead_letter_reason_check
    CHECK ((status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)) NOT VALID;

ALTER TABLE engagement_offer_outbox ADD COLUMN dead_letter_reason outbox_dead_letter_reason;
ALTER TABLE engagement_offer_outbox ADD CONSTRAINT engagement_offer_outbox_dead_letter_reason_check
    CHECK ((status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)) NOT VALID;

ALTER TABLE engagement_request_outbox ADD COLUMN dead_letter_reason outbox_dead_letter_reason;
ALTER TABLE engagement_request_outbox ADD CONSTRAINT engagement_request_outbox_dead_letter_reason_check
    CHECK ((status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)) NOT VALID;

ALTER TABLE feedback_issue_outbox ADD COLUMN dead_letter_reason outbox_dead_letter_reason;
ALTER TABLE feedback_issue_outbox ADD CONSTRAINT feedback_issue_outbox_dead_letter_reason_check
    CHECK ((status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)) NOT VALID;

ALTER TABLE legal_notice_outbox ADD COLUMN dead_letter_reason outbox_dead_letter_reason;
ALTER TABLE legal_notice_outbox ADD CONSTRAINT legal_notice_outbox_dead_letter_reason_check
    CHECK ((status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)) NOT VALID;

ALTER TABLE low_credit_outbox ADD COLUMN dead_letter_reason outbox_dead_letter_reason;
ALTER TABLE low_credit_outbox ADD CONSTRAINT low_credit_outbox_dead_letter_reason_check
    CHECK ((status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)) NOT VALID;

ALTER TABLE payment_received_outbox ADD COLUMN dead_letter_reason outbox_dead_letter_reason;
ALTER TABLE payment_received_outbox ADD CONSTRAINT payment_received_outbox_dead_letter_reason_check
    CHECK ((status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)) NOT VALID;

ALTER TABLE payout_outbox ADD COLUMN dead_letter_reason outbox_dead_letter_reason;
ALTER TABLE payout_outbox ADD CONSTRAINT payout_outbox_dead_letter_reason_check
    CHECK ((status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)) NOT VALID;

ALTER TABLE portal_address_change_outbox ADD COLUMN dead_letter_reason outbox_dead_letter_reason;
ALTER TABLE portal_address_change_outbox ADD CONSTRAINT portal_address_change_outbox_dead_letter_reason_check
    CHECK ((status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)) NOT VALID;

ALTER TABLE portal_invite_outbox ADD COLUMN dead_letter_reason outbox_dead_letter_reason;
ALTER TABLE portal_invite_outbox ADD CONSTRAINT portal_invite_outbox_dead_letter_reason_check
    CHECK ((status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)) NOT VALID;

ALTER TABLE portal_magic_link_outbox ADD COLUMN dead_letter_reason outbox_dead_letter_reason;
ALTER TABLE portal_magic_link_outbox ADD CONSTRAINT portal_magic_link_outbox_dead_letter_reason_check
    CHECK ((status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)) NOT VALID;

ALTER TABLE practice_deletion_outbox ADD COLUMN dead_letter_reason outbox_dead_letter_reason;
ALTER TABLE practice_deletion_outbox ADD CONSTRAINT practice_deletion_outbox_dead_letter_reason_check
    CHECK ((status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)) NOT VALID;

ALTER TABLE session_notice_outbox ADD COLUMN dead_letter_reason outbox_dead_letter_reason;
ALTER TABLE session_notice_outbox ADD CONSTRAINT session_notice_outbox_dead_letter_reason_check
    CHECK ((status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)) NOT VALID;

ALTER TABLE site_build_outbox ADD COLUMN dead_letter_reason outbox_dead_letter_reason;
ALTER TABLE site_build_outbox ADD CONSTRAINT site_build_outbox_dead_letter_reason_check
    CHECK ((status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)) NOT VALID;

ALTER TABLE staff_email_change_outbox ADD COLUMN dead_letter_reason outbox_dead_letter_reason;
ALTER TABLE staff_email_change_outbox ADD CONSTRAINT staff_email_change_outbox_dead_letter_reason_check
    CHECK ((status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)) NOT VALID;

ALTER TABLE staff_invite_outbox ADD COLUMN dead_letter_reason outbox_dead_letter_reason;
ALTER TABLE staff_invite_outbox ADD CONSTRAINT staff_invite_outbox_dead_letter_reason_check
    CHECK ((status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)) NOT VALID;

ALTER TABLE staff_mfa_recovery_outbox ADD COLUMN dead_letter_reason outbox_dead_letter_reason;
ALTER TABLE staff_mfa_recovery_outbox ADD CONSTRAINT staff_mfa_recovery_outbox_dead_letter_reason_check
    CHECK ((status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)) NOT VALID;

ALTER TABLE staff_notice_outbox ADD COLUMN dead_letter_reason outbox_dead_letter_reason;
ALTER TABLE staff_notice_outbox ADD CONSTRAINT staff_notice_outbox_dead_letter_reason_check
    CHECK ((status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)) NOT VALID;

ALTER TABLE staff_token_mail_outbox ADD COLUMN dead_letter_reason outbox_dead_letter_reason;
ALTER TABLE staff_token_mail_outbox ADD CONSTRAINT staff_token_mail_outbox_dead_letter_reason_check
    CHECK ((status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)) NOT VALID;

ALTER TABLE welcome_credits_outbox ADD COLUMN dead_letter_reason outbox_dead_letter_reason;
ALTER TABLE welcome_credits_outbox ADD CONSTRAINT welcome_credits_outbox_dead_letter_reason_check
    CHECK ((status = 'dead_lettered') = (dead_letter_reason IS NOT NULL)) NOT VALID;

-- +goose Down
ALTER TABLE client_erasure_outbox DROP COLUMN dead_letter_reason;
ALTER TABLE connect_nudge_outbox DROP COLUMN dead_letter_reason;
ALTER TABLE coverage_gap_outbox DROP COLUMN dead_letter_reason;
ALTER TABLE engagement_offer_outbox DROP COLUMN dead_letter_reason;
ALTER TABLE engagement_request_outbox DROP COLUMN dead_letter_reason;
ALTER TABLE feedback_issue_outbox DROP COLUMN dead_letter_reason;
ALTER TABLE legal_notice_outbox DROP COLUMN dead_letter_reason;
ALTER TABLE low_credit_outbox DROP COLUMN dead_letter_reason;
ALTER TABLE payment_received_outbox DROP COLUMN dead_letter_reason;
ALTER TABLE payout_outbox DROP COLUMN dead_letter_reason;
ALTER TABLE portal_address_change_outbox DROP COLUMN dead_letter_reason;
ALTER TABLE portal_invite_outbox DROP COLUMN dead_letter_reason;
ALTER TABLE portal_magic_link_outbox DROP COLUMN dead_letter_reason;
ALTER TABLE practice_deletion_outbox DROP COLUMN dead_letter_reason;
ALTER TABLE session_notice_outbox DROP COLUMN dead_letter_reason;
ALTER TABLE site_build_outbox DROP COLUMN dead_letter_reason;
ALTER TABLE staff_email_change_outbox DROP COLUMN dead_letter_reason;
ALTER TABLE staff_invite_outbox DROP COLUMN dead_letter_reason;
ALTER TABLE staff_mfa_recovery_outbox DROP COLUMN dead_letter_reason;
ALTER TABLE staff_notice_outbox DROP COLUMN dead_letter_reason;
ALTER TABLE staff_token_mail_outbox DROP COLUMN dead_letter_reason;
ALTER TABLE welcome_credits_outbox DROP COLUMN dead_letter_reason;
DROP TYPE outbox_dead_letter_reason;
