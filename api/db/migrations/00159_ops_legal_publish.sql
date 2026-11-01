-- +goose Up
-- #2486, ADR-0062: publishing a version of the Terms or the Privacy
-- Policy moves from doula-api's /api/internal to ops. A publication keys
-- the operator_acts row that records who published it in place of the
-- free-text published_by. app_runtime keeps reading publications and
-- sending the emails, but no longer writes either: a publication and its
-- emails are written together by ops_publish_legal_version.

ALTER TABLE operator_acts DROP CONSTRAINT operator_acts_action_check;
ALTER TABLE operator_acts ADD CONSTRAINT operator_acts_action_check
    CHECK (action IN ('sign_in', 'sign_out', 'step_up_refused', 'refund', 'founding_grant', 'second_factor_cleared', 'owner_contact_read', 'feedback_read', 'pilot_address_added', 'pilot_gate_opened', 'legal_version_published'));

-- No backfill: a version published through doula-api named its
-- publisher in published_by, and the actor key replaces that column.
-- Every publication from here on keys the act that published it.
ALTER TABLE legal_version_publications DROP CONSTRAINT legal_version_publications_published_by_named;
ALTER TABLE legal_version_publications DROP COLUMN published_by;
ALTER TABLE legal_version_publications ADD COLUMN operator_act_id uuid REFERENCES operator_acts (id);
ALTER TABLE legal_version_publications ADD CONSTRAINT legal_version_publications_operator_act
    CHECK (operator_act_id IS NOT NULL) NOT VALID;

REVOKE INSERT ON legal_version_publications FROM app_runtime;
REVOKE INSERT ON legal_notice_outbox FROM app_runtime;

-- The Owners a material version emails: each Owner of each Practice that
-- is not deleted and not in its deletion window (ADR-0031), whose login
-- is not deleted (ADR-0033). An Owner of two Practices is two rows. The
-- one rule both the check page's count and the emails read, so the two
-- cannot disagree. Granted to no role; the definer functions below call
-- it with their owner's rights.
-- +goose StatementBegin
CREATE FUNCTION legal_notice_owners()
RETURNS TABLE (practice_id uuid, staff_id uuid)
    LANGUAGE sql STABLE
    SET search_path = public, pg_temp
AS $$
    SELECT pm.practice_id, pm.staff_id
      FROM practice_memberships pm
      JOIN practices p ON p.id = pm.practice_id
      JOIN staff s ON s.id = pm.staff_id
     WHERE 'owner' = ANY(pm.roles)
       AND p.deleted_at IS NULL
       AND p.deletion_requested_at IS NULL
       AND s.deleted_at IS NULL
$$;
-- +goose StatementEnd

-- How many emails a material version published now would queue.
-- +goose StatementBegin
CREATE FUNCTION ops_legal_notice_owner_count()
RETURNS integer
    LANGUAGE sql SECURITY DEFINER STABLE
    SET search_path = public, pg_temp
AS $$
    SELECT count(*)::integer FROM legal_notice_owners()
$$;
-- +goose StatementEnd

-- Every published version, with the act that published it and how many
-- emails it queued.
-- +goose StatementBegin
CREATE FUNCTION ops_legal_publications()
RETURNS TABLE (document text, version text, material boolean, published_at timestamptz, operator_act_id uuid, queued integer)
    LANGUAGE sql SECURITY DEFINER STABLE
    SET search_path = public, pg_temp
AS $$
    SELECT pub.document, to_char(pub.version, 'YYYY-MM-DD'), pub.material, pub.published_at, pub.operator_act_id,
           (SELECT count(*)::integer FROM legal_notice_outbox o WHERE o.document = pub.document AND o.version = pub.version)
      FROM legal_version_publications pub
$$;
-- +goose StatementEnd

-- The publication itself, keyed to the act that made it, and for a
-- material version one email to each Owner legal_notice_owners names, in
-- the same transaction. Returns how many emails it queued, or NULL when
-- the version was already published, which keeps its first record.
-- +goose StatementBegin
CREATE FUNCTION ops_publish_legal_version(p_operator_act_id uuid, p_document text, p_version date, p_material boolean, p_published_at timestamptz)
RETURNS integer
    LANGUAGE plpgsql SECURITY DEFINER
    SET search_path = public, pg_temp
AS $$
DECLARE
    v_queued integer;
BEGIN
    INSERT INTO legal_version_publications (document, version, material, operator_act_id, published_at)
    VALUES (p_document, p_version, p_material, p_operator_act_id, p_published_at)
    ON CONFLICT (document, version) DO NOTHING;
    IF NOT FOUND THEN
        RETURN NULL;
    END IF;
    IF NOT p_material THEN
        RETURN 0;
    END IF;
    INSERT INTO legal_notice_outbox (document, version, practice_id, staff_id, created_at)
    SELECT p_document, p_version, o.practice_id, o.staff_id, p_published_at
      FROM legal_notice_owners() o;
    GET DIAGNOSTICS v_queued = ROW_COUNT;
    RETURN v_queued;
END
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION legal_notice_owners() FROM PUBLIC;
REVOKE EXECUTE ON FUNCTION ops_legal_notice_owner_count() FROM PUBLIC;
REVOKE EXECUTE ON FUNCTION ops_legal_publications() FROM PUBLIC;
REVOKE EXECUTE ON FUNCTION ops_publish_legal_version(uuid, text, date, boolean, timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ops_legal_notice_owner_count() TO ops_runtime;
GRANT EXECUTE ON FUNCTION ops_legal_publications() TO ops_runtime;
GRANT EXECUTE ON FUNCTION ops_publish_legal_version(uuid, text, date, boolean, timestamptz) TO ops_runtime;

-- +goose Down
DROP FUNCTION ops_publish_legal_version(uuid, text, date, boolean, timestamptz);
DROP FUNCTION ops_legal_publications();
DROP FUNCTION ops_legal_notice_owner_count();
DROP FUNCTION legal_notice_owners();
GRANT INSERT ON legal_notice_outbox TO app_runtime;
GRANT INSERT ON legal_version_publications TO app_runtime;
ALTER TABLE legal_version_publications DROP CONSTRAINT legal_version_publications_operator_act;
ALTER TABLE legal_version_publications DROP COLUMN operator_act_id;
ALTER TABLE legal_version_publications ADD COLUMN published_by text;
ALTER TABLE legal_version_publications ADD CONSTRAINT legal_version_publications_published_by_named
    CHECK (btrim(published_by) <> '') NOT VALID;
ALTER TABLE operator_acts DROP CONSTRAINT operator_acts_action_check;
ALTER TABLE operator_acts ADD CONSTRAINT operator_acts_action_check
    CHECK (action IN ('sign_in', 'sign_out', 'step_up_refused', 'refund', 'founding_grant', 'second_factor_cleared', 'owner_contact_read', 'feedback_read', 'pilot_address_added', 'pilot_gate_opened')) NOT VALID;
