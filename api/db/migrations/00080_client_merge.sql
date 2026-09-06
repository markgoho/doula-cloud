-- +goose Up
-- ADR-0017's amendment (docs/adr/0017-twelve-columns-a-practice-defined-
-- layer-and-an-engagement-that-is-asked-for.md), decided on #727 and
-- built on #814: an unattached Client record can be absorbed into a
-- matched one. The absorbed row keeps its id (ADR-0027: a clients row is
-- never deleted) and gains merged_into, pointing at the survivor.

-- =====================================================================
-- clients.merged_into
-- =====================================================================

-- Nullable: "not merged" is the normal state and has no target. Points
-- at another clients row, never at itself -- enforced below, alongside
-- the same practice, no chain, and no erased target.
ALTER TABLE clients ADD COLUMN merged_into uuid REFERENCES clients (id);

-- Backs both the Clients-list/search exclusion ("WHERE merged_into IS
-- NULL", run on every list/search/collision query) and the detail
-- redirect's own lookup. Partial: a merge is rare, so the index only
-- ever needs to hold the small minority of rows it is for.
CREATE INDEX clients_merged_into ON clients (merged_into) WHERE merged_into IS NOT NULL;

-- =====================================================================
-- clients_update: the tombstone can never be written to again
-- =====================================================================

-- USING gains merged_into IS NULL -- a pre-image check, so the write
-- that sets merged_into (tombstoning the row) still passes (the row's
-- state *before* that write has merged_into IS NULL), and every write
-- after it is refused. clients_select is untouched: the tombstone stays
-- readable, which is what the detail redirect needs.
--
-- WITH CHECK gains the one fact about a merge that is a property of the
-- row being written on its own, with no other row to consult: never a
-- self-merge. No chain (the target is not itself already absorbed) and
-- never into an erased Client are the two more facts ADR-0017's
-- amendment names, and both are enforced at the endpoint
-- (MergeHandler's own reads of the target before it writes) rather than
-- here -- a WITH CHECK subquery that reads clients from inside a policy
-- defined on clients itself was tried and, empirically, always evaluates
-- false under this Postgres version, however trivially true the
-- subquery's own condition is (confirmed with EXISTS (SELECT 1 FROM
-- clients s WHERE s.id = merged_into), rewritten and re-tested rather
-- than assumed). ADR-0017's "enforced at both seams" already has a
-- precedent for a gate that is endpoint-only: EraseHandler's own
-- erased_at refusal on edit is not mirrored in clients_update either.
DROP POLICY clients_update ON clients;
-- +goose StatementBegin
CREATE POLICY clients_update ON clients
    FOR UPDATE
    USING (
        practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        AND merged_into IS NULL
    )
    WITH CHECK (
        merged_into IS NULL
        OR merged_into <> id
    );
-- +goose StatementEnd

-- +goose Down
DROP POLICY clients_update ON clients;
-- +goose StatementBegin
CREATE POLICY clients_update ON clients
    FOR UPDATE
    USING (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid);
-- +goose StatementEnd

DROP INDEX clients_merged_into;
ALTER TABLE clients DROP COLUMN merged_into;
