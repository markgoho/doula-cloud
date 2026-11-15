-- +goose Up
-- contract_notice_outbox: a Practice Notification (ADR-0009) to a Client
-- about her Contract (#2951) -- one when it is sent, one each time Staff
-- remind her. Content-free: no name, no Engagement detail. One table, told
-- apart by kind, shaped like staff_notice_outbox (00138) and born on the
-- lifecycle 00152 and 00169 gave every outbox before it.
--
-- The row is the audit trail of the mail: what (kind), about which Contract,
-- to whom (sent_to_address at send time, since her address can change), when
-- (sent_at) and who caused it (triggered_by_staff_id). The reminder's own
-- Activity entry names who sent it; this row says whether it arrived.
CREATE TYPE contract_notice_outbox_status AS ENUM ('pending', 'sent', 'skipped', 'dead_lettered');

CREATE TYPE contract_notice_kind AS ENUM ('sent', 'reminder');

CREATE TABLE contract_notice_outbox (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    practice_id           uuid NOT NULL REFERENCES practices (id),
    contract_id           uuid NOT NULL REFERENCES contracts (id),
    kind                  contract_notice_kind NOT NULL,
    triggered_by_staff_id uuid REFERENCES staff (id),
    status                contract_notice_outbox_status NOT NULL DEFAULT 'pending',
    attempt_count         int NOT NULL DEFAULT 0,
    next_attempt_at       timestamptz NOT NULL,
    created_at            timestamptz NOT NULL,
    sent_at               timestamptz,
    last_error            text,
    sent_to_address       text,
    skip_reason           outbox_skip_reason,
    skipped_at            timestamptz,
    dead_letter_reason    outbox_dead_letter_reason,
    CONSTRAINT contract_notice_outbox_status_shape CHECK (
        (sent_at IS NOT NULL) = (status = 'sent')
        AND (skip_reason IS NOT NULL) = (status::text = 'skipped')
        AND (skipped_at IS NOT NULL) = (status::text = 'skipped')
    ),
    CONSTRAINT contract_notice_outbox_dead_letter_reason_check
        CHECK ((status = 'dead_lettered') = (dead_letter_reason IS NOT NULL))
);

-- At most one pending notice of a kind per Contract: a replayed send or
-- remind queues nothing twice.
CREATE UNIQUE INDEX contract_notice_outbox_one_pending
    ON contract_notice_outbox (kind, contract_id)
    WHERE status = 'pending';

CREATE INDEX contract_notice_outbox_claim
    ON contract_notice_outbox (next_attempt_at)
    WHERE status = 'pending';

GRANT SELECT, INSERT, UPDATE ON contract_notice_outbox TO app_runtime;

-- No RLS on the outbox itself, platform-level like every other outbox
-- table. The worker, which sets no app.current_practice_id, rechecks the
-- Contract and the Engagement it is about before it mails anyone, through
-- the notification_worker_trusted door 00032 opened on clients and
-- client_portal_users.
-- +goose StatementBegin
CREATE POLICY contracts_notification_worker ON contracts
    FOR SELECT
    USING (current_setting('app.notification_worker_trusted', true) = 'true');

CREATE POLICY engagements_notification_worker ON engagements
    FOR SELECT
    USING (current_setting('app.notification_worker_trusted', true) = 'true');
-- +goose StatementEnd

-- When a Client was last reminded of a Sent Contract. The reminder's
-- Activity entry is the record and names who sent it; this lets the
-- Contracts list say it on the row and the BFF refuse a second reminder in a
-- day without reading Activity.
ALTER TABLE contracts ADD COLUMN reminded_at timestamptz;

-- ops_outbox_status (00169), restated in full with an arm for the new table.
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
        SELECT 'contract_notice_outbox'::text AS outbox,
               count(*) FILTER (WHERE status = 'pending') AS pending,
               count(*) FILTER (WHERE status = 'sent') AS sent,
               count(*) FILTER (WHERE status::text = 'skipped') AS skipped,
               count(*) FILTER (WHERE status = 'dead_lettered') AS dead_lettered,
               0 AS bounced,
               0 AS complained,
               min(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= p_now) AS oldest_due_at
          FROM contract_notice_outbox
        UNION ALL
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

ALTER TABLE contracts DROP COLUMN reminded_at;
DROP POLICY engagements_notification_worker ON engagements;
DROP POLICY contracts_notification_worker ON contracts;
DROP TABLE contract_notice_outbox;
DROP TYPE contract_notice_kind;
DROP TYPE contract_notice_outbox_status;
