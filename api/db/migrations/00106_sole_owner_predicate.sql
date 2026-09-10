-- +goose Up
-- #694: "is this person the sole Owner of a Practice?" as a SECURITY
-- DEFINER predicate, so a caller may ask it about herself without being
-- given sight of anyone else's Membership row.
--
-- The question cannot be answered from the asker's own rows alone: it
-- turns on whether *another* Owner holds a Membership at the same
-- Practice. Before this, staffauth's Go-side isSoleOwnerAnywhere ran
-- that NOT EXISTS as a plain query, which meant every caller had to
-- already hold practice-wide visibility -- RotateSavedCodesHandler
-- reaches it by setting app.notification_worker_trusted, and a
-- pre-Practice read like GET /api/staff/session has neither that flag
-- nor an app.current_practice_id. Under
-- practice_memberships_self_visibility (00003/00006) the subquery would
-- have seen no other Owner's row at all and answered "sole" for every
-- Owner alive, silently, with no error to notice.
--
-- SECURITY DEFINER is the same instrument current_staff_id() (00003)
-- already uses, for the same reason: it runs as the table's owner, which
-- bypasses RLS, and it returns one boolean rather than a row. The
-- widening is exactly the width of the answer.
--
-- deleted_at is deliberately not filtered, matching what
-- isSoleOwnerAnywhere has always meant: saved-recovery-code eligibility
-- counts the sole Owner of a Practice pending deletion as its Owner,
-- because ADR-0031 promises a restore and a restored Practice with no
-- reachable Owner is a Practice nobody can act for. logindeletion.go's
-- soleOwnerPractices asks a different question and keeps its own filter;
-- the divergence is recorded there.
CREATE FUNCTION staff_is_sole_owner(subject_staff_id uuid) RETURNS boolean
    LANGUAGE sql SECURITY DEFINER STABLE
    SET search_path = public, pg_temp
    AS $$
        SELECT EXISTS (
            SELECT 1 FROM practice_memberships pm
            WHERE pm.staff_id = subject_staff_id AND 'owner' = ANY(pm.roles)
              AND NOT EXISTS (
                  SELECT 1 FROM practice_memberships other
                  WHERE other.practice_id = pm.practice_id
                    AND other.staff_id <> pm.staff_id
                    AND 'owner' = ANY(other.roles)
              )
        )
    $$;

-- +goose Down
DROP FUNCTION staff_is_sole_owner(uuid);
