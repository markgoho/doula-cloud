-- +goose Up
-- The outbox row lifecycle, declared (api/internal/outbox/lifecycle.go):
-- pending -> sent | skipped | dead_lettered, a retry staying pending, and
-- only a sent portal invite moving again, to bounced or complained. Until
-- now a row resolved with nothing sent -- a send-time recheck finding
-- nothing left to do, a deleted login, a restored Practice -- was marked
-- 'sent', and only welcome_credits_outbox recorded why. Each of the
-- twenty-one outboxes on that machine gains 'skipped', with skip_reason
-- and skipped_at, and a CHECK per table ties each state to the columns it
-- carries. site_build_outbox is its own machine (pending -> dispatched |
-- dead_lettered, lifecycle.go says why) and only gains the CHECK tying
-- 'dispatched' to dispatched_at.
--
-- The per-table enums stay. One shared outbox_status type was weighed:
-- moving to it rebuilds every partial index and CHECK naming the old
-- types, about thirty, and every enum already holds the same labels.
--
-- Postgres refuses to use an enum label in the transaction that added it
-- ("unsafe use of new value"), and this migration stays one transaction.
-- So on the twenty enums gaining 'skipped' by ADD VALUE, this file's own
-- CHECKs and function compare status::text = 'skipped'; code that runs
-- after the commit names the label directly. welcome_credits_outbox_status
-- is rebuilt instead, because its skipped rows have to be moved to the new
-- label here.

-- Why a row resolved with nothing sent, one value set for every outbox.
-- The last four are welcome_credits_outbox's own (00133), kept; its fifth,
-- no_owners, is no_recipients under the name every other outbox shares.
CREATE TYPE outbox_skip_reason AS ENUM (
    'already_resolved',
    'no_recipients',
    'login_deleted',
    'practice_restored',
    'engagement_started',
    'founding_grant',
    'practice_deleting',
    'owners_suppressed'
);

ALTER TYPE client_erasure_outbox_status ADD VALUE IF NOT EXISTS 'skipped' AFTER 'sent';
ALTER TYPE connect_nudge_outbox_status ADD VALUE IF NOT EXISTS 'skipped' AFTER 'sent';
ALTER TYPE coverage_gap_outbox_status ADD VALUE IF NOT EXISTS 'skipped' AFTER 'sent';
ALTER TYPE engagement_offer_outbox_status ADD VALUE IF NOT EXISTS 'skipped' AFTER 'sent';
ALTER TYPE engagement_request_outbox_status ADD VALUE IF NOT EXISTS 'skipped' AFTER 'sent';
ALTER TYPE feedback_issue_outbox_status ADD VALUE IF NOT EXISTS 'skipped' AFTER 'sent';
ALTER TYPE legal_notice_outbox_status ADD VALUE IF NOT EXISTS 'skipped' AFTER 'sent';
ALTER TYPE low_credit_outbox_status ADD VALUE IF NOT EXISTS 'skipped' AFTER 'sent';
ALTER TYPE payment_received_outbox_status ADD VALUE IF NOT EXISTS 'skipped' AFTER 'sent';
ALTER TYPE payout_outbox_status ADD VALUE IF NOT EXISTS 'skipped' AFTER 'sent';
ALTER TYPE portal_address_change_outbox_status ADD VALUE IF NOT EXISTS 'skipped' AFTER 'sent';
ALTER TYPE portal_invite_outbox_status ADD VALUE IF NOT EXISTS 'skipped' AFTER 'sent';
ALTER TYPE portal_magic_link_outbox_status ADD VALUE IF NOT EXISTS 'skipped' AFTER 'sent';
ALTER TYPE practice_deletion_outbox_status ADD VALUE IF NOT EXISTS 'skipped' AFTER 'sent';
ALTER TYPE session_notice_outbox_status ADD VALUE IF NOT EXISTS 'skipped' AFTER 'sent';
ALTER TYPE staff_email_change_outbox_status ADD VALUE IF NOT EXISTS 'skipped' AFTER 'sent';
ALTER TYPE staff_invite_outbox_status ADD VALUE IF NOT EXISTS 'skipped' AFTER 'sent';
ALTER TYPE staff_mfa_recovery_outbox_status ADD VALUE IF NOT EXISTS 'skipped' AFTER 'sent';
ALTER TYPE staff_notice_outbox_status ADD VALUE IF NOT EXISTS 'skipped' AFTER 'sent';
ALTER TYPE staff_token_mail_outbox_status ADD VALUE IF NOT EXISTS 'skipped' AFTER 'sent';

ALTER TABLE client_erasure_outbox ADD COLUMN skip_reason outbox_skip_reason;
ALTER TABLE client_erasure_outbox ADD COLUMN skipped_at timestamptz;
ALTER TABLE connect_nudge_outbox ADD COLUMN skip_reason outbox_skip_reason;
ALTER TABLE connect_nudge_outbox ADD COLUMN skipped_at timestamptz;
ALTER TABLE coverage_gap_outbox ADD COLUMN skip_reason outbox_skip_reason;
ALTER TABLE coverage_gap_outbox ADD COLUMN skipped_at timestamptz;
ALTER TABLE engagement_offer_outbox ADD COLUMN skip_reason outbox_skip_reason;
ALTER TABLE engagement_offer_outbox ADD COLUMN skipped_at timestamptz;
ALTER TABLE engagement_request_outbox ADD COLUMN skip_reason outbox_skip_reason;
ALTER TABLE engagement_request_outbox ADD COLUMN skipped_at timestamptz;
ALTER TABLE feedback_issue_outbox ADD COLUMN skip_reason outbox_skip_reason;
ALTER TABLE feedback_issue_outbox ADD COLUMN skipped_at timestamptz;
ALTER TABLE legal_notice_outbox ADD COLUMN skip_reason outbox_skip_reason;
ALTER TABLE legal_notice_outbox ADD COLUMN skipped_at timestamptz;
ALTER TABLE low_credit_outbox ADD COLUMN skip_reason outbox_skip_reason;
ALTER TABLE low_credit_outbox ADD COLUMN skipped_at timestamptz;
ALTER TABLE payment_received_outbox ADD COLUMN skip_reason outbox_skip_reason;
ALTER TABLE payment_received_outbox ADD COLUMN skipped_at timestamptz;
ALTER TABLE payout_outbox ADD COLUMN skip_reason outbox_skip_reason;
ALTER TABLE payout_outbox ADD COLUMN skipped_at timestamptz;
ALTER TABLE portal_address_change_outbox ADD COLUMN skip_reason outbox_skip_reason;
ALTER TABLE portal_address_change_outbox ADD COLUMN skipped_at timestamptz;
ALTER TABLE portal_invite_outbox ADD COLUMN skip_reason outbox_skip_reason;
ALTER TABLE portal_invite_outbox ADD COLUMN skipped_at timestamptz;
ALTER TABLE portal_magic_link_outbox ADD COLUMN skip_reason outbox_skip_reason;
ALTER TABLE portal_magic_link_outbox ADD COLUMN skipped_at timestamptz;
ALTER TABLE practice_deletion_outbox ADD COLUMN skip_reason outbox_skip_reason;
ALTER TABLE practice_deletion_outbox ADD COLUMN skipped_at timestamptz;
ALTER TABLE session_notice_outbox ADD COLUMN skip_reason outbox_skip_reason;
ALTER TABLE session_notice_outbox ADD COLUMN skipped_at timestamptz;
ALTER TABLE staff_email_change_outbox ADD COLUMN skip_reason outbox_skip_reason;
ALTER TABLE staff_email_change_outbox ADD COLUMN skipped_at timestamptz;
ALTER TABLE staff_invite_outbox ADD COLUMN skip_reason outbox_skip_reason;
ALTER TABLE staff_invite_outbox ADD COLUMN skipped_at timestamptz;
ALTER TABLE staff_mfa_recovery_outbox ADD COLUMN skip_reason outbox_skip_reason;
ALTER TABLE staff_mfa_recovery_outbox ADD COLUMN skipped_at timestamptz;
ALTER TABLE staff_notice_outbox ADD COLUMN skip_reason outbox_skip_reason;
ALTER TABLE staff_notice_outbox ADD COLUMN skipped_at timestamptz;
ALTER TABLE staff_token_mail_outbox ADD COLUMN skip_reason outbox_skip_reason;
ALTER TABLE staff_token_mail_outbox ADD COLUMN skipped_at timestamptz;

-- welcome_credits_outbox: its status type is rebuilt with 'skipped' (a type
-- created in this transaction may use its labels at once), its text
-- skip_reason becomes the shared type, and every row it marked sent with a
-- skip_reason moves to skipped, the time it acted moving with it. The
-- claim index and the 00152 CHECK name the old type, so they are dropped
-- and made again around the column change.
ALTER TYPE welcome_credits_outbox_status RENAME TO welcome_credits_outbox_status_old;
CREATE TYPE welcome_credits_outbox_status AS ENUM ('pending', 'sent', 'skipped', 'dead_lettered');
DROP INDEX welcome_credits_outbox_claim;
ALTER TABLE welcome_credits_outbox DROP CONSTRAINT welcome_credits_outbox_dead_letter_reason_check;
ALTER TABLE welcome_credits_outbox ALTER COLUMN status DROP DEFAULT;
ALTER TABLE welcome_credits_outbox ALTER COLUMN status TYPE welcome_credits_outbox_status USING status::text::welcome_credits_outbox_status;
ALTER TABLE welcome_credits_outbox ALTER COLUMN status SET DEFAULT 'pending';
DROP TYPE welcome_credits_outbox_status_old;
CREATE INDEX welcome_credits_outbox_claim ON welcome_credits_outbox (next_attempt_at) WHERE status = 'pending';
ALTER TABLE welcome_credits_outbox DROP CONSTRAINT welcome_credits_outbox_skip_reason_check;
ALTER TABLE welcome_credits_outbox ALTER COLUMN skip_reason TYPE outbox_skip_reason
    USING (CASE skip_reason WHEN 'no_owners' THEN 'no_recipients' ELSE skip_reason END)::outbox_skip_reason;
ALTER TABLE welcome_credits_outbox ADD COLUMN skipped_at timestamptz;
UPDATE welcome_credits_outbox SET status = 'skipped', skipped_at = sent_at, sent_at = NULL
 WHERE status = 'sent' AND skip_reason IS NOT NULL;

-- 00152's CHECKs tying dead_letter_reason to 'dead_lettered' were added
-- NOT VALID. A row dead-lettered before 00152 has no reason; it is given
-- the one that is alerted on, so an operator looks at it once rather than
-- never. Then each is validated -- welcome_credits_outbox's is made again,
-- validated, since its column changed type.
UPDATE client_erasure_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL;
UPDATE connect_nudge_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL;
UPDATE coverage_gap_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL;
UPDATE engagement_offer_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL;
UPDATE engagement_request_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL;
UPDATE feedback_issue_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL;
UPDATE legal_notice_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL;
UPDATE low_credit_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL;
UPDATE payment_received_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL;
UPDATE payout_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL;
UPDATE portal_address_change_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL;
UPDATE portal_invite_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL;
UPDATE portal_magic_link_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL;
UPDATE practice_deletion_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL;
UPDATE session_notice_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL;
UPDATE site_build_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL;
UPDATE staff_email_change_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL;
UPDATE staff_invite_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL;
UPDATE staff_mfa_recovery_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL;
UPDATE staff_notice_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL;
UPDATE staff_token_mail_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL;
UPDATE welcome_credits_outbox SET dead_letter_reason = 'retries_exhausted'
 WHERE status = 'dead_lettered' AND dead_letter_reason IS NULL;
ALTER TABLE client_erasure_outbox VALIDATE CONSTRAINT client_erasure_outbox_dead_letter_reason_check;
ALTER TABLE connect_nudge_outbox VALIDATE CONSTRAINT connect_nudge_outbox_dead_letter_reason_check;
ALTER TABLE coverage_gap_outbox VALIDATE CONSTRAINT coverage_gap_outbox_dead_letter_reason_check;
ALTER TABLE engagement_offer_outbox VALIDATE CONSTRAINT engagement_offer_outbox_dead_letter_reason_check;
ALTER TABLE engagement_request_outbox VALIDATE CONSTRAINT engagement_request_outbox_dead_letter_reason_check;
ALTER TABLE feedback_issue_outbox VALIDATE CONSTRAINT feedback_issue_outbox_dead_letter_reason_check;
ALTER TABLE legal_notice_outbox VALIDATE CONSTRAINT legal_notice_outbox_dead_letter_reason_check;
ALTER TABLE low_credit_outbox VALIDATE CONSTRAINT low_credit_outbox_dead_letter_reason_check;
ALTER TABLE payment_received_outbox VALIDATE CONSTRAINT payment_received_outbox_dead_letter_reason_check;
ALTER TABLE payout_outbox VALIDATE CONSTRAINT payout_outbox_dead_letter_reason_check;
ALTER TABLE portal_address_change_outbox VALIDATE CONSTRAINT portal_address_change_outbox_dead_letter_reason_check;
ALTER TABLE portal_invite_outbox VALIDATE CONSTRAINT portal_invite_outbox_dead_letter_reason_check;
ALTER TABLE portal_magic_link_outbox VALIDATE CONSTRAINT portal_magic_link_outbox_dead_letter_reason_check;
ALTER TABLE practice_deletion_outbox VALIDATE CONSTRAINT practice_deletion_outbox_dead_letter_reason_check;
ALTER TABLE session_notice_outbox VALIDATE CONSTRAINT session_notice_outbox_dead_letter_reason_check;
ALTER TABLE site_build_outbox VALIDATE CONSTRAINT site_build_outbox_dead_letter_reason_check;
ALTER TABLE staff_email_change_outbox VALIDATE CONSTRAINT staff_email_change_outbox_dead_letter_reason_check;
ALTER TABLE staff_invite_outbox VALIDATE CONSTRAINT staff_invite_outbox_dead_letter_reason_check;
ALTER TABLE staff_mfa_recovery_outbox VALIDATE CONSTRAINT staff_mfa_recovery_outbox_dead_letter_reason_check;
ALTER TABLE staff_notice_outbox VALIDATE CONSTRAINT staff_notice_outbox_dead_letter_reason_check;
ALTER TABLE staff_token_mail_outbox VALIDATE CONSTRAINT staff_token_mail_outbox_dead_letter_reason_check;
ALTER TABLE welcome_credits_outbox ADD CONSTRAINT welcome_credits_outbox_dead_letter_reason_check
    CHECK ((status = 'dead_lettered') = (dead_letter_reason IS NOT NULL));

-- A plaintext token or access code is held only while the row could still
-- mail it (00061: "nulled out the moment a row leaves 'pending'"). Until
-- now a deleted login's resolved rows kept theirs; those are cleared here,
-- and the CHECK refuses a finished row that keeps one.
UPDATE engagement_offer_outbox SET invite_token = NULL, access_code = NULL
 WHERE status <> 'pending' AND (invite_token IS NOT NULL OR access_code IS NOT NULL);
ALTER TABLE engagement_offer_outbox ADD CONSTRAINT engagement_offer_outbox_secret_cleared
    CHECK (status = 'pending' OR (invite_token IS NULL AND access_code IS NULL));
UPDATE portal_address_change_outbox SET token = NULL
 WHERE status <> 'pending' AND (token IS NOT NULL);
ALTER TABLE portal_address_change_outbox ADD CONSTRAINT portal_address_change_outbox_secret_cleared
    CHECK (status = 'pending' OR (token IS NULL));
UPDATE portal_magic_link_outbox SET token = NULL
 WHERE status <> 'pending' AND (token IS NOT NULL);
ALTER TABLE portal_magic_link_outbox ADD CONSTRAINT portal_magic_link_outbox_secret_cleared
    CHECK (status = 'pending' OR (token IS NULL));
UPDATE staff_invite_outbox SET invite_token = NULL
 WHERE status <> 'pending' AND (invite_token IS NOT NULL);
ALTER TABLE staff_invite_outbox ADD CONSTRAINT staff_invite_outbox_secret_cleared
    CHECK (status = 'pending' OR (invite_token IS NULL));
UPDATE staff_mfa_recovery_outbox SET token = NULL
 WHERE status <> 'pending' AND (token IS NOT NULL);
ALTER TABLE staff_mfa_recovery_outbox ADD CONSTRAINT staff_mfa_recovery_outbox_secret_cleared
    CHECK (status = 'pending' OR (token IS NULL));
UPDATE staff_token_mail_outbox SET token = NULL
 WHERE status <> 'pending' AND (token IS NOT NULL);
ALTER TABLE staff_token_mail_outbox ADD CONSTRAINT staff_token_mail_outbox_secret_cleared
    CHECK (status = 'pending' OR (token IS NULL));

-- Each state carries exactly its own columns: sent_at only on a row that
-- was sent (a portal invite keeps it once bounced or complained), and
-- skip_reason and skipped_at only on a skipped one.
ALTER TABLE client_erasure_outbox ADD CONSTRAINT client_erasure_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
);
ALTER TABLE connect_nudge_outbox ADD CONSTRAINT connect_nudge_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
);
ALTER TABLE coverage_gap_outbox ADD CONSTRAINT coverage_gap_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
);
ALTER TABLE engagement_offer_outbox ADD CONSTRAINT engagement_offer_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
);
ALTER TABLE engagement_request_outbox ADD CONSTRAINT engagement_request_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
);
ALTER TABLE feedback_issue_outbox ADD CONSTRAINT feedback_issue_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
);
ALTER TABLE legal_notice_outbox ADD CONSTRAINT legal_notice_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
);
ALTER TABLE low_credit_outbox ADD CONSTRAINT low_credit_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
);
ALTER TABLE payment_received_outbox ADD CONSTRAINT payment_received_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
);
ALTER TABLE payout_outbox ADD CONSTRAINT payout_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
);
ALTER TABLE portal_address_change_outbox ADD CONSTRAINT portal_address_change_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
);
ALTER TABLE portal_invite_outbox ADD CONSTRAINT portal_invite_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status IN ('sent', 'bounced', 'complained'))
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
);
ALTER TABLE portal_magic_link_outbox ADD CONSTRAINT portal_magic_link_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
);
ALTER TABLE practice_deletion_outbox ADD CONSTRAINT practice_deletion_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
);
ALTER TABLE session_notice_outbox ADD CONSTRAINT session_notice_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
);
ALTER TABLE staff_email_change_outbox ADD CONSTRAINT staff_email_change_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
);
ALTER TABLE staff_invite_outbox ADD CONSTRAINT staff_invite_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
);
ALTER TABLE staff_mfa_recovery_outbox ADD CONSTRAINT staff_mfa_recovery_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
);
ALTER TABLE staff_notice_outbox ADD CONSTRAINT staff_notice_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
);
ALTER TABLE staff_token_mail_outbox ADD CONSTRAINT staff_token_mail_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
);
ALTER TABLE welcome_credits_outbox ADD CONSTRAINT welcome_credits_outbox_status_shape CHECK (
    (sent_at IS NOT NULL) = (status = 'sent')
    AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
    AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
);
ALTER TABLE site_build_outbox ADD CONSTRAINT site_build_outbox_dispatched_at_check
    CHECK ((status = 'dispatched') = (dispatched_at IS NOT NULL));

-- ops_outbox_status (00163), restated in full with a count for every state
-- so the System page can tell a send from a skip, and see a bounce or a
-- complaint. Its columns change, so it is dropped and made again, and its
-- grant with it. site_build_outbox's 'dispatched' is its sent. Still one
-- arm per table: one query over all of them would need a view or dynamic
-- SQL across twenty-two tables that are kept apart on purpose.
DROP FUNCTION ops_outbox_status(timestamptz);

-- +goose StatementBegin
CREATE FUNCTION ops_outbox_status(p_now timestamptz)
RETURNS TABLE (outbox text, pending integer, sent integer, skipped integer, dead_lettered integer,
               bounced integer, complained integer, oldest_due_at timestamptz)
    LANGUAGE sql SECURITY DEFINER STABLE
    SET search_path = public, pg_temp
AS $$
    SELECT s.outbox, s.pending::integer, s.sent::integer, s.skipped::integer, s.dead_lettered::integer,
           s.bounced::integer, s.complained::integer, s.oldest_due_at FROM (
        SELECT 'client_erasure_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'sent') AS sent,
               count(*) FILTER (WHERE status::text = 'skipped') AS skipped,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               0 AS bounced,
               0 AS complained,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM client_erasure_outbox
        UNION ALL
        SELECT 'connect_nudge_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'sent') AS sent,
               count(*) FILTER (WHERE status::text = 'skipped') AS skipped,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               0 AS bounced,
               0 AS complained,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM connect_nudge_outbox
        UNION ALL
        SELECT 'coverage_gap_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'sent') AS sent,
               count(*) FILTER (WHERE status::text = 'skipped') AS skipped,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               0 AS bounced,
               0 AS complained,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM coverage_gap_outbox
        UNION ALL
        SELECT 'engagement_offer_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'sent') AS sent,
               count(*) FILTER (WHERE status::text = 'skipped') AS skipped,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               0 AS bounced,
               0 AS complained,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM engagement_offer_outbox
        UNION ALL
        SELECT 'engagement_request_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'sent') AS sent,
               count(*) FILTER (WHERE status::text = 'skipped') AS skipped,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               0 AS bounced,
               0 AS complained,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM engagement_request_outbox
        UNION ALL
        SELECT 'feedback_issue_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'sent') AS sent,
               count(*) FILTER (WHERE status::text = 'skipped') AS skipped,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               0 AS bounced,
               0 AS complained,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM feedback_issue_outbox
        UNION ALL
        SELECT 'legal_notice_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'sent') AS sent,
               count(*) FILTER (WHERE status::text = 'skipped') AS skipped,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               0 AS bounced,
               0 AS complained,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM legal_notice_outbox
        UNION ALL
        SELECT 'low_credit_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'sent') AS sent,
               count(*) FILTER (WHERE status::text = 'skipped') AS skipped,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               0 AS bounced,
               0 AS complained,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM low_credit_outbox
        UNION ALL
        SELECT 'payment_received_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'sent') AS sent,
               count(*) FILTER (WHERE status::text = 'skipped') AS skipped,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               0 AS bounced,
               0 AS complained,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM payment_received_outbox
        UNION ALL
        SELECT 'payout_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'sent') AS sent,
               count(*) FILTER (WHERE status::text = 'skipped') AS skipped,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               0 AS bounced,
               0 AS complained,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM payout_outbox
        UNION ALL
        SELECT 'portal_address_change_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'sent') AS sent,
               count(*) FILTER (WHERE status::text = 'skipped') AS skipped,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               0 AS bounced,
               0 AS complained,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM portal_address_change_outbox
        UNION ALL
        SELECT 'portal_invite_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'sent') AS sent,
               count(*) FILTER (WHERE status::text = 'skipped') AS skipped,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               count(*) FILTER (WHERE status = 'bounced') AS bounced,
               count(*) FILTER (WHERE status = 'complained') AS complained,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM portal_invite_outbox
        UNION ALL
        SELECT 'portal_magic_link_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'sent') AS sent,
               count(*) FILTER (WHERE status::text = 'skipped') AS skipped,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               0 AS bounced,
               0 AS complained,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM portal_magic_link_outbox
        UNION ALL
        SELECT 'practice_deletion_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'sent') AS sent,
               count(*) FILTER (WHERE status::text = 'skipped') AS skipped,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               0 AS bounced,
               0 AS complained,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM practice_deletion_outbox
        UNION ALL
        SELECT 'session_notice_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'sent') AS sent,
               count(*) FILTER (WHERE status::text = 'skipped') AS skipped,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               0 AS bounced,
               0 AS complained,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM session_notice_outbox
        UNION ALL
        SELECT 'site_build_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dispatched') AS sent,
               0 AS skipped,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               0 AS bounced,
               0 AS complained,
               min(created_at) FILTER (WHERE status = 'pending' AND created_at <= p_now) AS oldest_due_at
          FROM site_build_outbox
        UNION ALL
        SELECT 'staff_email_change_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'sent') AS sent,
               count(*) FILTER (WHERE status::text = 'skipped') AS skipped,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               0 AS bounced,
               0 AS complained,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM staff_email_change_outbox
        UNION ALL
        SELECT 'staff_invite_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'sent') AS sent,
               count(*) FILTER (WHERE status::text = 'skipped') AS skipped,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               0 AS bounced,
               0 AS complained,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM staff_invite_outbox
        UNION ALL
        SELECT 'staff_mfa_recovery_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'sent') AS sent,
               count(*) FILTER (WHERE status::text = 'skipped') AS skipped,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               0 AS bounced,
               0 AS complained,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM staff_mfa_recovery_outbox
        UNION ALL
        SELECT 'staff_notice_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'sent') AS sent,
               count(*) FILTER (WHERE status::text = 'skipped') AS skipped,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               0 AS bounced,
               0 AS complained,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM staff_notice_outbox
        UNION ALL
        SELECT 'staff_token_mail_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'sent') AS sent,
               count(*) FILTER (WHERE status::text = 'skipped') AS skipped,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               0 AS bounced,
               0 AS complained,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM staff_token_mail_outbox
        UNION ALL
        SELECT 'welcome_credits_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'sent') AS sent,
               count(*) FILTER (WHERE status::text = 'skipped') AS skipped,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               0 AS bounced,
               0 AS complained,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM welcome_credits_outbox
    ) s
     ORDER BY s.outbox
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION ops_outbox_status(timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ops_outbox_status(timestamptz) TO ops_runtime;

-- +goose Down
DROP FUNCTION ops_outbox_status(timestamptz);

-- +goose StatementBegin
CREATE FUNCTION ops_outbox_status(p_now timestamptz)
RETURNS TABLE (outbox text, pending integer, dead_lettered integer, oldest_due_at timestamptz)
    LANGUAGE sql SECURITY DEFINER STABLE
    SET search_path = public, pg_temp
AS $$
    SELECT s.outbox, s.pending::integer, s.dead_lettered::integer, s.oldest_due_at FROM (
        SELECT 'client_erasure_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM client_erasure_outbox
        UNION ALL
        SELECT 'connect_nudge_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM connect_nudge_outbox
        UNION ALL
        SELECT 'coverage_gap_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM coverage_gap_outbox
        UNION ALL
        SELECT 'engagement_offer_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM engagement_offer_outbox
        UNION ALL
        SELECT 'engagement_request_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM engagement_request_outbox
        UNION ALL
        SELECT 'feedback_issue_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM feedback_issue_outbox
        UNION ALL
        SELECT 'legal_notice_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM legal_notice_outbox
        UNION ALL
        SELECT 'low_credit_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM low_credit_outbox
        UNION ALL
        SELECT 'payment_received_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM payment_received_outbox
        UNION ALL
        SELECT 'payout_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM payout_outbox
        UNION ALL
        SELECT 'portal_address_change_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM portal_address_change_outbox
        UNION ALL
        SELECT 'portal_invite_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM portal_invite_outbox
        UNION ALL
        SELECT 'portal_magic_link_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM portal_magic_link_outbox
        UNION ALL
        SELECT 'practice_deletion_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM practice_deletion_outbox
        UNION ALL
        SELECT 'session_notice_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM session_notice_outbox
        UNION ALL
        SELECT 'site_build_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(created_at) FILTER (WHERE status = 'pending' AND created_at <= p_now) AS oldest_due_at
          FROM site_build_outbox
        UNION ALL
        SELECT 'staff_email_change_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM staff_email_change_outbox
        UNION ALL
        SELECT 'staff_invite_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM staff_invite_outbox
        UNION ALL
        SELECT 'staff_mfa_recovery_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM staff_mfa_recovery_outbox
        UNION ALL
        SELECT 'staff_notice_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM staff_notice_outbox
        UNION ALL
        SELECT 'staff_token_mail_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM staff_token_mail_outbox
        UNION ALL
        SELECT 'welcome_credits_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM welcome_credits_outbox
    ) s
     ORDER BY s.outbox
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION ops_outbox_status(timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ops_outbox_status(timestamptz) TO ops_runtime;

ALTER TABLE site_build_outbox DROP CONSTRAINT site_build_outbox_dispatched_at_check;
ALTER TABLE client_erasure_outbox DROP CONSTRAINT client_erasure_outbox_status_shape;
ALTER TABLE connect_nudge_outbox DROP CONSTRAINT connect_nudge_outbox_status_shape;
ALTER TABLE coverage_gap_outbox DROP CONSTRAINT coverage_gap_outbox_status_shape;
ALTER TABLE engagement_offer_outbox DROP CONSTRAINT engagement_offer_outbox_status_shape;
ALTER TABLE engagement_request_outbox DROP CONSTRAINT engagement_request_outbox_status_shape;
ALTER TABLE feedback_issue_outbox DROP CONSTRAINT feedback_issue_outbox_status_shape;
ALTER TABLE legal_notice_outbox DROP CONSTRAINT legal_notice_outbox_status_shape;
ALTER TABLE low_credit_outbox DROP CONSTRAINT low_credit_outbox_status_shape;
ALTER TABLE payment_received_outbox DROP CONSTRAINT payment_received_outbox_status_shape;
ALTER TABLE payout_outbox DROP CONSTRAINT payout_outbox_status_shape;
ALTER TABLE portal_address_change_outbox DROP CONSTRAINT portal_address_change_outbox_status_shape;
ALTER TABLE portal_invite_outbox DROP CONSTRAINT portal_invite_outbox_status_shape;
ALTER TABLE portal_magic_link_outbox DROP CONSTRAINT portal_magic_link_outbox_status_shape;
ALTER TABLE practice_deletion_outbox DROP CONSTRAINT practice_deletion_outbox_status_shape;
ALTER TABLE session_notice_outbox DROP CONSTRAINT session_notice_outbox_status_shape;
ALTER TABLE staff_email_change_outbox DROP CONSTRAINT staff_email_change_outbox_status_shape;
ALTER TABLE staff_invite_outbox DROP CONSTRAINT staff_invite_outbox_status_shape;
ALTER TABLE staff_mfa_recovery_outbox DROP CONSTRAINT staff_mfa_recovery_outbox_status_shape;
ALTER TABLE staff_notice_outbox DROP CONSTRAINT staff_notice_outbox_status_shape;
ALTER TABLE staff_token_mail_outbox DROP CONSTRAINT staff_token_mail_outbox_status_shape;
ALTER TABLE welcome_credits_outbox DROP CONSTRAINT welcome_credits_outbox_status_shape;
ALTER TABLE engagement_offer_outbox DROP CONSTRAINT engagement_offer_outbox_secret_cleared;
ALTER TABLE portal_address_change_outbox DROP CONSTRAINT portal_address_change_outbox_secret_cleared;
ALTER TABLE portal_magic_link_outbox DROP CONSTRAINT portal_magic_link_outbox_secret_cleared;
ALTER TABLE staff_invite_outbox DROP CONSTRAINT staff_invite_outbox_secret_cleared;
ALTER TABLE staff_mfa_recovery_outbox DROP CONSTRAINT staff_mfa_recovery_outbox_secret_cleared;
ALTER TABLE staff_token_mail_outbox DROP CONSTRAINT staff_token_mail_outbox_secret_cleared;

-- Postgres has no ALTER TYPE ... DROP VALUE, so 'skipped' stays on each of
-- the twenty enums, unused: every skipped row goes back to sent, as it was
-- marked before this migration.
UPDATE client_erasure_outbox SET status = 'sent', sent_at = skipped_at WHERE status = 'skipped';
ALTER TABLE client_erasure_outbox DROP COLUMN skipped_at;
ALTER TABLE client_erasure_outbox DROP COLUMN skip_reason;
UPDATE connect_nudge_outbox SET status = 'sent', sent_at = skipped_at WHERE status = 'skipped';
ALTER TABLE connect_nudge_outbox DROP COLUMN skipped_at;
ALTER TABLE connect_nudge_outbox DROP COLUMN skip_reason;
UPDATE coverage_gap_outbox SET status = 'sent', sent_at = skipped_at WHERE status = 'skipped';
ALTER TABLE coverage_gap_outbox DROP COLUMN skipped_at;
ALTER TABLE coverage_gap_outbox DROP COLUMN skip_reason;
UPDATE engagement_offer_outbox SET status = 'sent', sent_at = skipped_at WHERE status = 'skipped';
ALTER TABLE engagement_offer_outbox DROP COLUMN skipped_at;
ALTER TABLE engagement_offer_outbox DROP COLUMN skip_reason;
UPDATE engagement_request_outbox SET status = 'sent', sent_at = skipped_at WHERE status = 'skipped';
ALTER TABLE engagement_request_outbox DROP COLUMN skipped_at;
ALTER TABLE engagement_request_outbox DROP COLUMN skip_reason;
UPDATE feedback_issue_outbox SET status = 'sent', sent_at = skipped_at WHERE status = 'skipped';
ALTER TABLE feedback_issue_outbox DROP COLUMN skipped_at;
ALTER TABLE feedback_issue_outbox DROP COLUMN skip_reason;
UPDATE legal_notice_outbox SET status = 'sent', sent_at = skipped_at WHERE status = 'skipped';
ALTER TABLE legal_notice_outbox DROP COLUMN skipped_at;
ALTER TABLE legal_notice_outbox DROP COLUMN skip_reason;
UPDATE low_credit_outbox SET status = 'sent', sent_at = skipped_at WHERE status = 'skipped';
ALTER TABLE low_credit_outbox DROP COLUMN skipped_at;
ALTER TABLE low_credit_outbox DROP COLUMN skip_reason;
UPDATE payment_received_outbox SET status = 'sent', sent_at = skipped_at WHERE status = 'skipped';
ALTER TABLE payment_received_outbox DROP COLUMN skipped_at;
ALTER TABLE payment_received_outbox DROP COLUMN skip_reason;
UPDATE payout_outbox SET status = 'sent', sent_at = skipped_at WHERE status = 'skipped';
ALTER TABLE payout_outbox DROP COLUMN skipped_at;
ALTER TABLE payout_outbox DROP COLUMN skip_reason;
UPDATE portal_address_change_outbox SET status = 'sent', sent_at = skipped_at WHERE status = 'skipped';
ALTER TABLE portal_address_change_outbox DROP COLUMN skipped_at;
ALTER TABLE portal_address_change_outbox DROP COLUMN skip_reason;
UPDATE portal_invite_outbox SET status = 'sent', sent_at = skipped_at WHERE status = 'skipped';
ALTER TABLE portal_invite_outbox DROP COLUMN skipped_at;
ALTER TABLE portal_invite_outbox DROP COLUMN skip_reason;
UPDATE portal_magic_link_outbox SET status = 'sent', sent_at = skipped_at WHERE status = 'skipped';
ALTER TABLE portal_magic_link_outbox DROP COLUMN skipped_at;
ALTER TABLE portal_magic_link_outbox DROP COLUMN skip_reason;
UPDATE practice_deletion_outbox SET status = 'sent', sent_at = skipped_at WHERE status = 'skipped';
ALTER TABLE practice_deletion_outbox DROP COLUMN skipped_at;
ALTER TABLE practice_deletion_outbox DROP COLUMN skip_reason;
UPDATE session_notice_outbox SET status = 'sent', sent_at = skipped_at WHERE status = 'skipped';
ALTER TABLE session_notice_outbox DROP COLUMN skipped_at;
ALTER TABLE session_notice_outbox DROP COLUMN skip_reason;
UPDATE staff_email_change_outbox SET status = 'sent', sent_at = skipped_at WHERE status = 'skipped';
ALTER TABLE staff_email_change_outbox DROP COLUMN skipped_at;
ALTER TABLE staff_email_change_outbox DROP COLUMN skip_reason;
UPDATE staff_invite_outbox SET status = 'sent', sent_at = skipped_at WHERE status = 'skipped';
ALTER TABLE staff_invite_outbox DROP COLUMN skipped_at;
ALTER TABLE staff_invite_outbox DROP COLUMN skip_reason;
UPDATE staff_mfa_recovery_outbox SET status = 'sent', sent_at = skipped_at WHERE status = 'skipped';
ALTER TABLE staff_mfa_recovery_outbox DROP COLUMN skipped_at;
ALTER TABLE staff_mfa_recovery_outbox DROP COLUMN skip_reason;
UPDATE staff_notice_outbox SET status = 'sent', sent_at = skipped_at WHERE status = 'skipped';
ALTER TABLE staff_notice_outbox DROP COLUMN skipped_at;
ALTER TABLE staff_notice_outbox DROP COLUMN skip_reason;
UPDATE staff_token_mail_outbox SET status = 'sent', sent_at = skipped_at WHERE status = 'skipped';
ALTER TABLE staff_token_mail_outbox DROP COLUMN skipped_at;
ALTER TABLE staff_token_mail_outbox DROP COLUMN skip_reason;
UPDATE welcome_credits_outbox SET status = 'sent', sent_at = skipped_at WHERE status = 'skipped';
ALTER TABLE welcome_credits_outbox DROP COLUMN skipped_at;
ALTER TABLE welcome_credits_outbox ALTER COLUMN skip_reason TYPE text
    USING (CASE skip_reason WHEN 'no_recipients' THEN 'no_owners' ELSE skip_reason::text END);
ALTER TABLE welcome_credits_outbox ADD CONSTRAINT welcome_credits_outbox_skip_reason_check CHECK (skip_reason IN (
    'engagement_started',
    'founding_grant',
    'practice_deleting',
    'owners_suppressed',
    'no_owners'
));
DROP TYPE outbox_skip_reason;
