-- +goose Up
-- The write half of ADR-0008's Offer/Attachment model (#317). 00030 built
-- engagement_offers and engagement_attachments as schema only; this
-- migration adds the three things the handlers need that the schema
-- section did not name: a brute-force bound on the emailed six-digit
-- code, the pre-account read door the token-authenticated Offer read
-- looks a row up through, and the outbox that mails the Offer.

-- ---------------------------------------------------------------------
-- access_code_attempts: 00030 gave the Offer an access_code_digest but
-- nothing bounding how many guesses it will accept. The pre-account read
-- sits outside staffauth entirely, so a six-digit code is a 10^6 space in
-- front of an unauthenticated endpoint -- CLAUDE.md's Security
-- expectation ("refuses what it should refuse, at the boundary that can
-- actually enforce it"). The counter lives on the row rather than in a
-- rate limiter because the row is what the guesser is attacking: a
-- burned Offer stays burned across processes, restarts, and source
-- addresses.
-- ---------------------------------------------------------------------
ALTER TABLE engagement_offers ADD COLUMN access_code_attempts int NOT NULL DEFAULT 0;

-- ---------------------------------------------------------------------
-- The pre-account read runs before any Practice is known -- the reader
-- holds no membership anywhere yet -- so 00030's
-- engagement_offers_practice_visibility (practice-tier) matches nothing
-- and the lookup would return zero rows. This policy is the narrow door
-- for exactly that read, keyed on the digest of the Invitation token the
-- caller presented, the same shape practice_invitations_accept_lookup
-- (00039) already opened on the Invitation itself.
--
-- Unlike that one this is an ALL policy, not SELECT-only: the same
-- caller bumps access_code_attempts on a wrong guess and flips state to
-- 'declined' on a decline, and neither write has a Practice context to
-- go through the practice-tier policy with. It reaches only Offers whose
-- invitation_id resolves to the presented token -- an Offer to an
-- existing Staff member has no invitation_id at all and stays
-- unreachable here.
-- ---------------------------------------------------------------------
-- +goose StatementBegin
CREATE POLICY engagement_offers_token_lookup ON engagement_offers
    USING (
        EXISTS (
            SELECT 1 FROM practice_invitations pi
            WHERE pi.id = engagement_offers.invitation_id
              AND pi.token_digest = NULLIF(current_setting('app.invite_token_digest', true), '')
        )
    );
-- +goose StatementEnd

-- ---------------------------------------------------------------------
-- engagement_offer_outbox: the Offer's own Notification (ADR-0010),
-- mirroring staff_invite_outbox (00038) row for row. A separate table
-- rather than a reuse of that one because the email is a different
-- Notification with different content: one link that both joins the
-- Practice and opens the Offer, plus the six-digit code that opens it.
-- Sending the Invitation's own email as well would mail the same person
-- two links to the same token for the same reason.
--
-- Like staff_invite_outbox this table carries secrets in the clear --
-- engagement_offers holds only access_code_digest, practice_invitations
-- only token_digest -- so there is nowhere else for the worker to read a
-- mailable token or code from at send time. Both are nulled out the
-- moment the row leaves 'pending', sent or dead-lettered alike.
-- ---------------------------------------------------------------------
CREATE TYPE engagement_offer_outbox_status AS ENUM ('pending', 'sent', 'dead_lettered');

CREATE TABLE engagement_offer_outbox (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    offer_id uuid NOT NULL REFERENCES engagement_offers (id),
    invite_token uuid,
    access_code text,
    status engagement_offer_outbox_status NOT NULL DEFAULT 'pending',
    attempt_count int NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    sent_at timestamptz,
    last_error text
);

-- At most one pending row per Offer, the same partial-index idiom
-- staff_invite_outbox_one_pending (00038) uses.
CREATE UNIQUE INDEX engagement_offer_outbox_one_pending
    ON engagement_offer_outbox (offer_id)
    WHERE status = 'pending';

GRANT SELECT, INSERT, UPDATE ON engagement_offer_outbox TO app_runtime;

-- No RLS -- platform-level like every other outbox table: the worker runs
-- with no Practice or Client session context.

-- The worker needs the Offer row (its state and expiry at send time, so
-- an Offer withdrawn or accepted before this row was sent is never
-- mailed) and its Invitation (the recipient's address). 00038 already
-- opened the app.notification_worker_trusted door on
-- practice_invitations; this is the same door on engagement_offers.
-- +goose StatementBegin
CREATE POLICY engagement_offers_notification_worker ON engagement_offers
    FOR SELECT
    USING (current_setting('app.notification_worker_trusted', true) = 'true');
-- +goose StatementEnd

-- The worker stamps access_code_sent_at on the Offer once the code is
-- actually in the post, which is a write and so needs its own door --
-- the SELECT policy above does not cover it.
-- +goose StatementBegin
CREATE POLICY engagement_offers_notification_worker_stamp ON engagement_offers
    FOR UPDATE
    USING (current_setting('app.notification_worker_trusted', true) = 'true');
-- +goose StatementEnd

-- +goose Down
DROP POLICY engagement_offers_notification_worker_stamp ON engagement_offers;
DROP POLICY engagement_offers_notification_worker ON engagement_offers;
DROP TABLE engagement_offer_outbox;
DROP TYPE engagement_offer_outbox_status;
DROP POLICY engagement_offers_token_lookup ON engagement_offers;
ALTER TABLE engagement_offers DROP COLUMN access_code_attempts;
