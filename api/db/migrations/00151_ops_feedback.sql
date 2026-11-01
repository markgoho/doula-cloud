-- +goose Up
-- #2478, ADR-0062: Feedback is read in ops. Two ops_runtime-only definer
-- functions: the list, which names nobody, and one piece, which names its
-- sender and is the one ops function that reads clients (the Feedback
-- exception). Opening a piece writes a feedback_read row. app/'s founder
-- read page goes, and with it 00122's door for a Staff session.

ALTER TABLE operator_acts DROP CONSTRAINT operator_acts_action_check;
ALTER TABLE operator_acts ADD CONSTRAINT operator_acts_action_check
    CHECK (action IN ('sign_in', 'sign_out', 'step_up_refused', 'refund', 'founding_grant', 'second_factor_cleared', 'owner_contact_read', 'feedback_read'));

-- What the list's paging reads.
CREATE INDEX feedback_sent_at_id ON feedback (sent_at DESC, id DESC);

-- One page of the list, newest first, after the cursor (p_after_at,
-- p_after_id), both NULL for the first page. p_unopened narrows it to the
-- pieces whose issue did not open and whose open job has failed:
-- dead-lettered, or pending after a failed attempt. Each row carries the
-- piece's newest open job.
-- +goose StatementBegin
CREATE FUNCTION ops_feedback_list(p_unopened boolean, p_after_at timestamptz, p_after_id uuid, p_limit integer)
RETURNS TABLE (
    id uuid, kind text, route_id text, sent_at timestamptz, issue_number integer,
    job_status text, attempt_count integer, last_error text, dead_letter_reason text
)
    LANGUAGE sql SECURITY DEFINER STABLE
    SET search_path = public, pg_temp
AS $$
    SELECT f.id, f.kind::text, f.route_id, f.sent_at, f.issue_number,
           o.status::text, COALESCE(o.attempt_count, 0), o.last_error, o.dead_letter_reason::text
      FROM feedback f
      LEFT JOIN LATERAL (
           SELECT j.status, j.attempt_count, j.last_error, j.dead_letter_reason
             FROM feedback_issue_outbox j
            WHERE j.feedback_id = f.id AND j.act = 'open'
            ORDER BY j.created_at DESC
            LIMIT 1
      ) o ON true
     WHERE (p_after_at IS NULL OR (f.sent_at, f.id) < (p_after_at, p_after_id))
       AND (NOT p_unopened OR (f.issue_number IS NULL
            AND (o.status = 'dead_lettered' OR (o.status = 'pending' AND o.attempt_count > 0))))
     ORDER BY f.sent_at DESC, f.id DESC
     LIMIT p_limit
$$;
-- +goose StatementEnd

-- One piece with every stored field but client_id, its sender's name and
-- email, its Practice's name and its newest open job. A Staff sender is
-- named from staff; a Client sender by her own name on clients and her
-- Portal Account's sign-in address.
-- +goose StatementBegin
CREATE FUNCTION ops_feedback_piece(p_feedback_id uuid)
RETURNS TABLE (
    id uuid, kind text, text text, page_url text, route_id text, app_build text, screen_width integer,
    browser text, sent_at timestamptz, roles text, from_portal boolean, issue_number integer,
    sender_name text, sender_email text, practice_name text,
    job_status text, attempt_count integer, last_error text, dead_letter_reason text
)
    LANGUAGE sql SECURITY DEFINER STABLE
    SET search_path = public, pg_temp
AS $$
    SELECT f.id, f.kind::text, f.text, f.page_url, f.route_id, f.app_build, f.screen_width,
           f.browser, f.sent_at, array_to_string(f.roles, ','), f.portal_account IS NOT NULL, f.issue_number,
           CASE WHEN f.portal_account IS NULL THEN btrim(s.first_name || ' ' || s.last_name)
                ELSE NULLIF(trim(concat_ws(' ', COALESCE(c.preferred_name, c.given_name), c.family_name)), '') END,
           COALESCE(s.email, pa.sign_in_address), p.name,
           o.status::text, COALESCE(o.attempt_count, 0), o.last_error, o.dead_letter_reason::text
      FROM feedback f
      LEFT JOIN staff s ON s.id = f.staff_id
      LEFT JOIN portal_accounts pa ON pa.identifier = f.portal_account
      LEFT JOIN clients c ON c.id = f.client_id
      LEFT JOIN practices p ON p.id = f.practice_id
      LEFT JOIN LATERAL (
           SELECT j.status, j.attempt_count, j.last_error, j.dead_letter_reason
             FROM feedback_issue_outbox j
            WHERE j.feedback_id = f.id AND j.act = 'open'
            ORDER BY j.created_at DESC
            LIMIT 1
      ) o ON true
     WHERE f.id = p_feedback_id
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION ops_feedback_list(boolean, timestamptz, uuid, integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ops_feedback_list(boolean, timestamptz, uuid, integer) TO ops_runtime;
REVOKE EXECUTE ON FUNCTION ops_feedback_piece(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ops_feedback_piece(uuid) TO ops_runtime;

-- 00122's door for the founder's Staff session, and its read record:
-- reads are now operator_acts rows.
DROP TABLE feedback_reads;
REVOKE EXECUTE ON FUNCTION feedback_sender(uuid) FROM app_runtime;
DROP FUNCTION feedback_sender(uuid);
DROP POLICY feedback_reader_select ON feedback;
DROP FUNCTION feedback_reader();

-- +goose Down
CREATE FUNCTION feedback_reader() RETURNS uuid
    LANGUAGE sql STABLE
    AS $$
        SELECT NULLIF(current_setting('app.feedback_reader', true), '')::uuid
    $$;

-- +goose StatementBegin
CREATE POLICY feedback_reader_select ON feedback
    FOR SELECT
    USING (feedback_reader() = (SELECT current_staff_id()));
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION feedback_sender(p_feedback_id uuid)
RETURNS TABLE (sender_name text, sender_email text, practice_name text)
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = public, pg_temp
AS $$
    SELECT btrim(s.first_name || ' ' || s.last_name), COALESCE(s.email, pa.sign_in_address), p.name
      FROM feedback f
      LEFT JOIN staff s ON s.id = f.staff_id
      LEFT JOIN portal_accounts pa ON pa.identifier = f.portal_account
      LEFT JOIN practices p ON p.id = f.practice_id
     WHERE f.id = p_feedback_id
       AND feedback_reader() = current_staff_id();
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION feedback_sender(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION feedback_sender(uuid) TO app_runtime;

CREATE TABLE feedback_reads (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    feedback_id uuid NOT NULL REFERENCES feedback (id) ON DELETE CASCADE,
    staff_id    uuid NOT NULL REFERENCES staff (id),
    read_at     timestamptz NOT NULL
);
CREATE INDEX feedback_reads_feedback_id ON feedback_reads (feedback_id);
GRANT INSERT ON feedback_reads TO app_runtime;
ALTER TABLE feedback_reads ENABLE ROW LEVEL SECURITY;

-- +goose StatementBegin
CREATE POLICY feedback_reads_reader_insert ON feedback_reads
    FOR INSERT
    WITH CHECK (staff_id = feedback_reader() AND staff_id = current_staff_id());
-- +goose StatementEnd

DROP FUNCTION ops_feedback_piece(uuid);
DROP FUNCTION ops_feedback_list(boolean, timestamptz, uuid, integer);
DROP INDEX feedback_sent_at_id;
ALTER TABLE operator_acts DROP CONSTRAINT operator_acts_action_check;
ALTER TABLE operator_acts ADD CONSTRAINT operator_acts_action_check
    CHECK (action IN ('sign_in', 'sign_out', 'step_up_refused', 'refund', 'founding_grant', 'second_factor_cleared', 'owner_contact_read')) NOT VALID;
