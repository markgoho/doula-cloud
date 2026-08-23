-- +goose Up
-- ADR-0008 (docs/adr/0008-employment-type-gates-the-practice-attachment-
-- gates-the-engagement.md): employment type gates ambient reach over a
-- Practice, Attachment gates a Doula's reach over one Engagement, and an
-- Offer is how a contractor's Attachment is granted through her own
-- agreement. This migration builds the ADR's Concrete Schema section
-- exactly -- every table, enum, column, constraint, and index below is
-- named there; nothing here re-decides a column. It is schema only: no
-- handler in this codebase yet writes practice_invitations,
-- engagement_attachments, or engagement_offers (#316, #317 build those).

-- ---------------------------------------------------------------------
-- staff: identity_uid returns to NOT NULL. UNIQUE already held (00002)
-- and was never the bug (#226) -- it returns to NOT NULL because the
-- pending, identity-less row this nullability existed for no longer
-- exists once practice_invitations replaces it below. No staff row
-- exists at invite time from here on; the three RLS policies 00004
-- added for that pending-row flow (staff_invite_insert,
-- staff_accept_invite_select, staff_accept_invite_update) are replaced,
-- not extended, so they are dropped rather than kept dead.
-- ---------------------------------------------------------------------
DROP POLICY staff_accept_invite_update ON staff;
DROP POLICY staff_accept_invite_select ON staff;
DROP POLICY staff_invite_insert ON staff;
ALTER TABLE staff ALTER COLUMN identity_uid SET NOT NULL;

-- ---------------------------------------------------------------------
-- employment_type: not a role. A role says what a person does; this
-- says what they are to the business. Every Membership carries one, the
-- founding Owner's included (#227) -- NOT NULL is free, since the
-- half-formed zero-identity membership this nullability would have
-- needed to accommodate no longer exists once the block above lands.
-- ---------------------------------------------------------------------
CREATE TYPE employment_type AS ENUM ('employee', 'contractor');

ALTER TABLE practice_memberships
    ADD COLUMN employment_type employment_type NOT NULL;

-- ---------------------------------------------------------------------
-- practice_invitations: replaces the pending staff row as how joining
-- works. No staff row exists until accept resolves the caller's
-- verified identity to a new or existing one (#226). roles and
-- employment_type ride the Invitation, not a later PATCH -- the
-- zero-role membership stays abolished (RA-G8). token_digest is a
-- SHA-256 digest, following the precedent 00028_sessions.sql set: a
-- leaked read of this table hands nobody a usable credential.
-- ---------------------------------------------------------------------
CREATE TYPE invitation_status AS ENUM ('pending', 'accepted', 'revoked', 'expired');

CREATE TABLE practice_invitations (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    practice_id       uuid NOT NULL REFERENCES practices (id),
    address           text NOT NULL,
    roles             practice_role[] NOT NULL,
    employment_type   employment_type NOT NULL,
    token_digest      text NOT NULL,
    status            invitation_status NOT NULL DEFAULT 'pending',
    invited_by        uuid NOT NULL REFERENCES staff (id),
    created_at        timestamptz NOT NULL DEFAULT now(),
    expires_at        timestamptz NOT NULL,
    revoked_by        uuid REFERENCES staff (id),
    revoked_at        timestamptz,
    accepted_staff_id uuid REFERENCES staff (id)
);

-- No DELETE: an Invitation is revoked or expires, never erased -- it is
-- part of the audit record of who was invited and when, the same
-- reasoning invoices (00024) applies to a financial row.
GRANT SELECT, INSERT, UPDATE ON practice_invitations TO app_runtime;

ALTER TABLE practice_invitations ENABLE ROW LEVEL SECURITY;

-- practice_invitations carries practice_id directly, so its policy is a
-- plain column comparison, the same shape practice_memberships_practice_visibility
-- (00002) and invoices_practice_visibility (00024) use.
CREATE POLICY practice_invitations_practice_visibility ON practice_invitations
    USING (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid);

-- ---------------------------------------------------------------------
-- engagement_attachments: one row per (Engagement, Doula), universal,
-- several per Engagement, Doula-only (#228). origin upgrades accrued ->
-- granted and never the reverse; only a granted attachment reaches, an
-- accrued one is a record of work, never a key. Ending is ended_at,
-- never a delete -- "on this from February to May" is more of the
-- record, not less. fee_amount_cents/fee_terms are this ADR's own
-- addition: NULL for an accrued attachment and for any granted
-- attachment opened without an Offer, copied from the Offer at
-- acceptance otherwise (#229).
-- ---------------------------------------------------------------------
CREATE TYPE attachment_origin AS ENUM ('accrued', 'granted');

CREATE TABLE engagement_attachments (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    engagement_id    uuid NOT NULL REFERENCES engagements (id),
    staff_id         uuid NOT NULL REFERENCES staff (id),
    origin           attachment_origin NOT NULL,
    attached_by      uuid NOT NULL REFERENCES staff (id),
    attached_at      timestamptz NOT NULL DEFAULT now(),
    ended_at         timestamptz,
    ended_by         uuid REFERENCES staff (id),
    fee_amount_cents bigint,
    fee_terms        text
);

CREATE UNIQUE INDEX engagement_attachments_one_open
    ON engagement_attachments (engagement_id, staff_id)
    WHERE ended_at IS NULL;

-- No DELETE: an attachment is ended, never erased (#228).
GRANT SELECT, INSERT, UPDATE ON engagement_attachments TO app_runtime;

ALTER TABLE engagement_attachments ENABLE ROW LEVEL SECURITY;

-- engagement_attachments has no practice_id column, so its
-- practice-tier visibility is an EXISTS subquery against engagements,
-- the same shape visits_practice_visibility (00007_visit.sql) sets.
-- No chicken-and-egg case: an attachment is always created under an
-- Engagement that already exists.
CREATE POLICY engagement_attachments_practice_visibility ON engagement_attachments
    USING (
        EXISTS (
            SELECT 1 FROM engagements e
            WHERE e.id = engagement_attachments.engagement_id
              AND e.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        )
    );

-- ---------------------------------------------------------------------
-- engagement_offers: the event that grants a contractor's attachment
-- through her own agreement (an employee may also be offered work, for
-- the claim rather than the reach). staff_id and invitation_id are both
-- nullable, exactly one required -- an Offer to an email address sets
-- invitation_id and staff_id stays NULL until accept backfills it.
-- employment_type is snapshotted onto the row, not read live off the
-- target's Membership (which may not exist yet for the email path); the
-- fee CHECK runs entirely inside the row because of it. due_date,
-- client_area, and client_first_initial are typed in by the sender at
-- send time, not derived from any Engagement or Client column -- the
-- Offer is a copy, never refreshed after it is sent. decided_by is NULL
-- exactly once: the system-driven withdrawn state on Engagement
-- completion, which by construction has no human actor. access_code_*
-- are populated only for the invitation_id path -- an existing Staff
-- member reads her Offer through her session and needs no code. (#229,
-- #230, and this document's own calls -- see ADR-0008.)
-- ---------------------------------------------------------------------
CREATE TYPE offer_state AS ENUM
    ('offered', 'accepted', 'declined', 'withdrawn', 'superseded', 'expired');

CREATE TABLE engagement_offers (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    engagement_id         uuid NOT NULL REFERENCES engagements (id),
    staff_id              uuid REFERENCES staff (id),
    invitation_id         uuid REFERENCES practice_invitations (id),
    employment_type       employment_type NOT NULL,
    amount_cents          bigint,
    terms                 text,
    client_first_initial  text NOT NULL,
    client_area           text NOT NULL,
    due_date              date NOT NULL,
    state                 offer_state NOT NULL DEFAULT 'offered',
    offered_by            uuid NOT NULL REFERENCES staff (id),
    offered_at            timestamptz NOT NULL DEFAULT now(),
    expires_at            timestamptz NOT NULL,
    decided_at            timestamptz,
    decided_by            uuid REFERENCES staff (id),
    access_code_digest    text,
    access_code_sent_at   timestamptz,

    CONSTRAINT offer_target_named
        CHECK (staff_id IS NOT NULL OR invitation_id IS NOT NULL),
    CONSTRAINT offer_fee_matches_employment_type
        CHECK (
            (employment_type = 'contractor' AND amount_cents IS NOT NULL)
            OR (employment_type = 'employee' AND amount_cents IS NULL)
        )
);

-- Fan-out is uncapped, first yes wins: at most one open Offer per
-- (Engagement, staff) and per (Engagement, invitation) at a time. The
-- accept handler owns the actual race (#229) -- these two partial
-- indexes only half enforce it, which is why "at most one open Offer"
-- is a database constraint but "first accept wins the race" is not.
CREATE UNIQUE INDEX engagement_offers_one_open_per_staff
    ON engagement_offers (engagement_id, staff_id)
    WHERE state = 'offered' AND staff_id IS NOT NULL;

CREATE UNIQUE INDEX engagement_offers_one_open_per_invitation
    ON engagement_offers (engagement_id, invitation_id)
    WHERE state = 'offered' AND invitation_id IS NOT NULL;

-- No DELETE: an Offer is withdrawn, declined, or expires, never erased
-- -- Renata reading her own history must be able to tell those apart.
GRANT SELECT, INSERT, UPDATE ON engagement_offers TO app_runtime;

ALTER TABLE engagement_offers ENABLE ROW LEVEL SECURITY;

-- Same EXISTS shape as engagement_attachments above -- no
-- chicken-and-egg case, an Offer is always created under an Engagement
-- that already exists.
CREATE POLICY engagement_offers_practice_visibility ON engagement_offers
    USING (
        EXISTS (
            SELECT 1 FROM engagements e
            WHERE e.id = engagement_offers.engagement_id
              AND e.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        )
    );

-- +goose Down
-- Dropping each table takes its policy with it, the same convention
-- 00002/00004/00007 follow.
DROP TABLE engagement_offers;
DROP TYPE offer_state;

DROP TABLE engagement_attachments;
DROP TYPE attachment_origin;

DROP TABLE practice_invitations;
DROP TYPE invitation_status;

ALTER TABLE practice_memberships DROP COLUMN employment_type;
DROP TYPE employment_type;

ALTER TABLE staff ALTER COLUMN identity_uid DROP NOT NULL;

-- +goose StatementBegin
CREATE POLICY staff_invite_insert ON staff
    FOR INSERT
    WITH CHECK (
        identity_uid IS NULL
        AND EXISTS (
            SELECT 1 FROM practice_memberships pm
            WHERE pm.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
              AND pm.staff_id = current_staff_id()
              AND 'owner' = ANY (pm.roles)
        )
    );
-- +goose StatementEnd

-- +goose StatementBegin
CREATE POLICY staff_accept_invite_select ON staff
    FOR SELECT
    USING (
        identity_uid IS NULL
        AND invite_token = NULLIF(current_setting('app.invite_token', true), '')::uuid
    );

CREATE POLICY staff_accept_invite_update ON staff
    FOR UPDATE
    USING (
        identity_uid IS NULL
        AND invite_token = NULLIF(current_setting('app.invite_token', true), '')::uuid
    )
    WITH CHECK (identity_uid = NULLIF(current_setting('app.current_identity_uid', true), ''));
-- +goose StatementEnd
