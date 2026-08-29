-- +goose Up
-- ADR-0017's Client Field Template settings screen (#399): the field list
-- an Owner or Admin defines for a Practice's Clients, plus the audit of
-- the field list itself -- a separate table from client_events, because
-- these rows audit a Practice's settings, not a Client
-- ("Why does this Client hold a value in a field that is not on the
-- form?" has no other answer).
--
-- 00042 gave client_field_templates a bare, all-commands, any-staff RLS
-- policy -- fine while nothing wrote to the table. Now that PUT is
-- Owner/Admin-only at the endpoint, the policy is split so a Doula's
-- session is refused at the database seam too, not just the Go one --
-- the same INSERT/UPDATE role check clients_insert (00042) already uses.

DROP POLICY client_field_templates_practice_visibility ON client_field_templates;

-- +goose StatementBegin
CREATE POLICY client_field_templates_select ON client_field_templates
    FOR SELECT
    USING (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE POLICY client_field_templates_insert ON client_field_templates
    FOR INSERT
    WITH CHECK (
        practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        AND EXISTS (
            SELECT 1 FROM practice_memberships pm
            WHERE pm.staff_id = NULLIF(current_setting('app.current_staff_id', true), '')::uuid
              AND pm.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
              AND pm.roles && ARRAY['owner', 'admin']::practice_role[]
        )
    );
-- +goose StatementEnd

-- +goose StatementBegin
CREATE POLICY client_field_templates_update ON client_field_templates
    FOR UPDATE
    USING (
        practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        AND EXISTS (
            SELECT 1 FROM practice_memberships pm
            WHERE pm.staff_id = NULLIF(current_setting('app.current_staff_id', true), '')::uuid
              AND pm.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
              AND pm.roles && ARRAY['owner', 'admin']::practice_role[]
        )
    );
-- +goose StatementEnd

-- One row per PUT that actually changes the field list, holding a
-- before/after diff of the whole array -- the same act-becomes-the-row
-- shape as client_events, but simpler: a Client Field Template has one
-- act (replace the list), not an open set of individually-diffable
-- facts, so a per-field diff engine buys nothing. actor_staff_id is
-- NOT NULL -- unlike client_events, nothing ever writes this table as
-- 'system', so there is no actor_kind to check.
CREATE TABLE client_field_template_events (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    practice_id    uuid NOT NULL REFERENCES practices (id),
    diff           jsonb NOT NULL,
    actor_staff_id uuid NOT NULL REFERENCES staff (id),
    created_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX client_field_template_events_practice ON client_field_template_events (practice_id, created_at);

GRANT SELECT, INSERT ON client_field_template_events TO app_runtime;   -- no UPDATE, no DELETE -- append-only

ALTER TABLE client_field_template_events ENABLE ROW LEVEL SECURITY;

-- +goose StatementBegin
CREATE POLICY client_field_template_events_select ON client_field_template_events
    FOR SELECT
    USING (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid);
-- +goose StatementEnd

-- actor_staff_id must be the caller's own id, and the caller must hold
-- the owner or admin role -- the same belt-and-braces as the template
-- table's own INSERT/UPDATE policies above, so an audit row can never be
-- forged with someone else's name on it.
-- +goose StatementBegin
CREATE POLICY client_field_template_events_insert ON client_field_template_events
    FOR INSERT
    WITH CHECK (
        practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        AND actor_staff_id = NULLIF(current_setting('app.current_staff_id', true), '')::uuid
        AND EXISTS (
            SELECT 1 FROM practice_memberships pm
            WHERE pm.staff_id = actor_staff_id
              AND pm.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
              AND pm.roles && ARRAY['owner', 'admin']::practice_role[]
        )
    );
-- +goose StatementEnd

-- +goose Down
DROP POLICY client_field_template_events_insert ON client_field_template_events;
DROP POLICY client_field_template_events_select ON client_field_template_events;
DROP TABLE client_field_template_events;

DROP POLICY client_field_templates_update ON client_field_templates;
DROP POLICY client_field_templates_insert ON client_field_templates;
DROP POLICY client_field_templates_select ON client_field_templates;

-- +goose StatementBegin
CREATE POLICY client_field_templates_practice_visibility ON client_field_templates
    USING (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid);
-- +goose StatementEnd
