-- +goose Up
-- The write half of ADR-0008's Invitation/Membership model (#316).
-- 00030 built practice_invitations as schema only; this migration adds
-- the three things a handler needs that the schema section did not name:
-- an accepted_at timestamp, the uniqueness a re-invite rotates against,
-- and the pre-Practice-context read door accept looks an Invitation up
-- through. It also adds the Membership audit table CLAUDE.md's
-- audit-trail expectation asks for.

-- ---------------------------------------------------------------------
-- accepted_at: 00030 gave the Invitation an accepted_staff_id but no
-- "when". invited_by/created_at and revoked_by/revoked_at already pair
-- an actor with a timestamp; acceptance is the third lifecycle event and
-- gets the same pair (accepted_staff_id is both -- the accepter acts on
-- her own behalf).
-- ---------------------------------------------------------------------
ALTER TABLE practice_invitations ADD COLUMN accepted_at timestamptz;

-- ---------------------------------------------------------------------
-- At most one pending Invitation per address per Practice. This is what
-- makes InviteHandler's "rotate the existing pending row" branch
-- race-safe rather than a read-then-write that two concurrent invites
-- can both win -- the same partial-index idiom 00030's
-- engagement_attachments_one_open and 00038's
-- staff_invite_outbox_one_pending use. lower(address) because the
-- address is matched case-insensitively at accept (an Identity Platform
-- account's verified address is not guaranteed to preserve the case the
-- Owner typed), so two rows differing only in case would be one
-- Invitation as far as accept is concerned.
-- ---------------------------------------------------------------------
CREATE UNIQUE INDEX practice_invitations_one_pending
    ON practice_invitations (practice_id, lower(address))
    WHERE status = 'pending';

-- ---------------------------------------------------------------------
-- The accept path runs before any Practice is known: the caller holds no
-- membership anywhere yet, so 00030's
-- practice_invitations_practice_visibility (practice-tier) matches
-- nothing and the lookup would return zero rows. This policy is the
-- narrow door for exactly that lookup -- keyed on the digest of the
-- token the caller presented, so it opens one row and only to someone
-- already holding its token, the same shape 00004's dropped
-- staff_accept_invite_select had. SELECT only: once accept has read the
-- row it sets app.current_practice_id to that Invitation's Practice,
-- from which point the practice-tier policy governs the UPDATE that
-- flips status.
-- ---------------------------------------------------------------------
CREATE POLICY practice_invitations_accept_lookup ON practice_invitations
    FOR SELECT
    USING (token_digest = NULLIF(current_setting('app.invite_token_digest', true), ''));

-- ---------------------------------------------------------------------
-- practice_membership_events: how a Membership came to hold what it
-- holds. practice_invitations already answers "who invited this person,
-- when, and who accepted" for the joining half; nothing answered "who
-- changed her roles" or "who moved her from employee to contractor",
-- which ADR-0008 makes a consequential change (employment type gates
-- ambient reach over the whole Practice). One row per change, both sides
-- recorded, so the answer is readable without replaying anything.
--
-- No UPDATE, no DELETE grant: an event is a fact about the past.
-- ---------------------------------------------------------------------
CREATE TYPE membership_event_type AS ENUM
    ('joined', 'roles_changed', 'employment_type_changed');

CREATE TABLE practice_membership_events (
    id                       uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    practice_id              uuid NOT NULL REFERENCES practices (id),
    staff_id                 uuid NOT NULL REFERENCES staff (id),
    event_type               membership_event_type NOT NULL,
    previous_roles           practice_role[],
    roles                    practice_role[],
    previous_employment_type employment_type,
    employment_type          employment_type,
    actor_staff_id           uuid NOT NULL REFERENCES staff (id),
    created_at               timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX practice_membership_events_membership
    ON practice_membership_events (practice_id, staff_id, created_at);

GRANT SELECT, INSERT ON practice_membership_events TO app_runtime;

ALTER TABLE practice_membership_events ENABLE ROW LEVEL SECURITY;

-- Carries practice_id directly, so a plain column comparison, the same
-- shape practice_invitations_practice_visibility (00030) uses.
CREATE POLICY practice_membership_events_practice_visibility ON practice_membership_events
    USING (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid);

-- +goose Down
DROP TABLE practice_membership_events;
DROP TYPE membership_event_type;

DROP POLICY practice_invitations_accept_lookup ON practice_invitations;
DROP INDEX practice_invitations_one_pending;
ALTER TABLE practice_invitations DROP COLUMN accepted_at;
