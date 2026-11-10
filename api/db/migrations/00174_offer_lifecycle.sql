-- +goose Up
-- An Offer's lifecycle, made explicit (offer's lifecycle.go is the Go
-- half).
--
-- An Offer is open while it is 'offered' and its expires_at is still
-- ahead. Until this migration that was restated by hand in a dozen
-- places, some with the expires_at half and some without it, and the
-- ones without it were right only if expireOpen's lazy sweep had already
-- flipped the row: completion's cascade stamped a lapsed Offer
-- 'withdrawn', and an acceptance stamped a lapsed sibling 'superseded',
-- because neither looked at the clock.
--
-- 'expired' stays a stored state, unlike a Staff Invitation's (00170):
-- an Offer's history is read -- by her inbox, by the Practice's list of
-- who was asked, by the export -- and every closed Offer answers "when
-- did this stop" from decided_at. What changes is that the stored value
-- is a record of a fact the clock already decided, never the decision
-- itself:
--
--   * offer_state_at and offer_open_at below are the one definition of
--     what an Offer is at an instant. Every reader and every guard asks
--     them, so none depends on the sweep having run.
--   * a lapsed Offer is recorded with decided_at = expires_at, the
--     instant it lapsed, not the instant somebody happened to look; so a
--     row flipped late and a row flipped on time read the same.
--
-- Both take the instant as a parameter, never now(): the Go side passes
-- clock.Now(ctx), the clock expires_at was written from (#1992, #773).

CREATE FUNCTION offer_state_at(p_state offer_state, p_expires_at timestamptz, p_at timestamptz)
RETURNS offer_state
LANGUAGE sql IMMUTABLE PARALLEL SAFE
RETURN CASE
    WHEN p_state = 'offered' AND p_expires_at <= p_at THEN 'expired'::offer_state
    ELSE p_state
END;

CREATE FUNCTION offer_open_at(p_state offer_state, p_expires_at timestamptz, p_at timestamptz)
RETURNS boolean
LANGUAGE sql IMMUTABLE PARALLEL SAFE
RETURN offer_state_at(p_state, p_expires_at, p_at) = 'offered';

-- Who closed an Offer. decided_by names a Staff row, and three of the
-- ways an Offer closes have none to name -- ADR-0008 forbids inventing
-- one -- so until now a NULL decided_by meant any of three things and
-- the row could not say which:
--
--   staff       a Staff member: she accepted or declined, an Owner or an
--               Admin withdrew it, or her acceptance superseded it.
--   invitee     the holder of the emailed link and code declined it
--               before she had an account (DeclineByTokenHandler).
--   completion  the Engagement completed, and its cascade withdrew every
--               Offer still open (CloseOnCompletion).
--   lapse       nobody: expires_at passed.
CREATE TYPE offer_decider_kind AS ENUM ('staff', 'invitee', 'completion', 'lapse');

ALTER TABLE engagement_offers ADD COLUMN decided_by_kind offer_decider_kind;

-- Each state carries exactly the who and when it means:
--
--   offered     no decided_at, no decider.
--   expired     decided_at is expires_at, and the decider is the lapse.
--   the rest    decided_at before expires_at -- a person cannot decide an
--               Offer that has already run out -- and a decider the
--               state admits: a Staff member for any of them, the invitee
--               for a decline only, the completion cascade for a
--               withdrawal only.
--
-- Every term is written so it is never NULL, because a CHECK passes on
-- NULL.
--
-- NOT VALID, as 00170's is: the constraint holds every row written from
-- here on without scanning the rows trunk's database already has, none
-- of which carries decided_by_kind and some of which were flipped to
-- 'expired' at the moment of a read rather than at expires_at.
ALTER TABLE engagement_offers ADD CONSTRAINT engagement_offers_lifecycle CHECK (
    (state = 'offered'
        AND decided_at IS NULL AND decided_by IS NULL AND decided_by_kind IS NULL)
 OR (state = 'expired'
        AND decided_by_kind IS NOT DISTINCT FROM 'lapse'
        AND decided_by IS NULL
        AND decided_at IS NOT DISTINCT FROM expires_at)
 OR (state IN ('accepted', 'declined', 'withdrawn', 'superseded')
        AND decided_at IS NOT NULL AND decided_at < expires_at
        AND ((decided_by_kind IS NOT DISTINCT FROM 'staff' AND decided_by IS NOT NULL)
          OR (decided_by_kind IS NOT DISTINCT FROM 'invitee' AND state = 'declined' AND decided_by IS NULL)
          OR (decided_by_kind IS NOT DISTINCT FROM 'completion' AND state = 'withdrawn' AND decided_by IS NULL)))
) NOT VALID;

-- +goose Down
ALTER TABLE engagement_offers DROP CONSTRAINT engagement_offers_lifecycle;
ALTER TABLE engagement_offers DROP COLUMN decided_by_kind;
DROP TYPE offer_decider_kind;
DROP FUNCTION offer_open_at(offer_state, timestamptz, timestamptz);
DROP FUNCTION offer_state_at(offer_state, timestamptz, timestamptz);
