-- +goose Up
-- #1533 (ADR-0026's amendment for #1492): the product recommends a
-- second factor to an Owner with none, after three acts of hers -- First
-- Value, her first Staff Invitation, and Stripe connected. She can close
-- the notice, and it shows again once at each new act on the list.
--
-- One row per act the notice was closed for: who closed it (staff_id),
-- at which Practice, for which act, and when. Closing records every act
-- reached at that moment, so a later act is the only thing that brings
-- the notice back. The primary key makes a repeated close a no-op.
--
-- Append-only: no UPDATE and no DELETE. closed_at comes from
-- clock.Now(ctx) (#773), never now().
CREATE TABLE second_factor_recommendation_closures (
    practice_id uuid NOT NULL REFERENCES practices (id),
    staff_id    uuid NOT NULL REFERENCES staff (id),
    act         text NOT NULL
                CONSTRAINT second_factor_recommendation_closures_act_known
                CHECK (act IN ('first_value', 'first_invitation', 'stripe_connected')),
    closed_at   timestamptz NOT NULL,
    PRIMARY KEY (practice_id, staff_id, act)
);

GRANT SELECT, INSERT ON second_factor_recommendation_closures TO app_runtime;

ALTER TABLE second_factor_recommendation_closures ENABLE ROW LEVEL SECURITY;

-- A plain practice_id comparison, the shape practice_invitations (00030)
-- uses.
CREATE POLICY second_factor_recommendation_closures_practice_visibility
    ON second_factor_recommendation_closures
    USING (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid);

-- +goose Down
DROP TABLE second_factor_recommendation_closures;
