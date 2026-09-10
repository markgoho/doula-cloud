-- +goose Up
-- #813: two Client records that both carry history are the same woman,
-- and the product now merges them for real. 00080 (#814) could only
-- absorb a record nothing pointed at; this migration is what lets the
-- things that point at one Client point at the other instead.
--
-- The reversal this carries is deliberate and recorded: ADR-0040
-- supersedes the part of ADR-0015/ADR-0017 that said an Engagement
-- belongs to one Client for life. See
-- docs/adr/0040-a-client-record-merges-and-the-engagement-moves-with-her.md.
--
-- 00093's own comment says "Nothing in the product updates client_id
-- today -- a Client with Engagements cannot even be merged away
-- (client/merge.go)". That sentence is now false. It is not edited
-- there: goose checksums an applied file by content, so the correction
-- lives here instead.
--
-- ---------------------------------------------------------------------
-- Every foreign key into clients, and what the merge does with it.
-- Enumerated here rather than discovered in a failing test, because a
-- table missed by the merge is a woman's history stranded on a
-- tombstone that no screen will ever show again.
--
--   engagements            (00005) -- MOVES. The reversal itself.
--   engagement_requests    (00042) -- MOVES. An Engagement only exists
--                                     because a Request was approved, so
--                                     the Request follows it or the
--                                     approval loses its subject.
--   client_portal_users    (00006) -- MOVES, through
--                                     merge_client_portal_links below:
--                                     no Staff-facing UPDATE policy
--                                     admits an accepted row (00042's
--                                     invite_update demands
--                                     identity_uid IS NULL), so a plain
--                                     UPDATE moves nothing and says so
--                                     with neither an error nor a row.
--   client_stripe_customers(00076) -- STAYS. The table carries SELECT
--                                     and INSERT for app_runtime and no
--                                     UPDATE at all: 00076 states that
--                                     repointing a Stripe Customer is
--                                     not an operation this product has.
--                                     The rows stay on the absorbed
--                                     record and are reached by erasure
--                                     (redact_absorbed_client below,
--                                     plus client.eraseAbsorbedRecords).
--   client_data_keys       (00064) -- STAYS, and this is a constraint
--                                     rather than an omission. ADR-0027
--                                     seals every diff under the
--                                     Client's own key and ADR-0022's
--                                     activity table is append-only, so
--                                     re-sealing the absorbed woman's
--                                     diffs under the survivor's key
--                                     would rewrite rows that may never
--                                     be rewritten. Both keys survive
--                                     the merge; erasing the merged
--                                     Client must shred both.
--   activity               (00051) -- STAYS, by the same rule. The
--                                     merged Client carries two audit
--                                     trails under one name, and the
--                                     screen says so rather than
--                                     implying one history.
--   client_erasure_outbox  (00064) -- STAYS. A row records what was done
--                                     to one record's Stripe presence,
--                                     and that record is the absorbed
--                                     one.
--   clients.merged_into    (00080) -- the tombstone itself, written
--                                     below with its plaintext audit.
-- ---------------------------------------------------------------------

-- =====================================================================
-- The plaintext audit of the merge
-- =====================================================================

-- Who merged which two records, and when. Deliberately plaintext
-- columns on clients rather than only a sealed activity diff: the
-- 'merged' and 'absorbed' diffs are sealed under two client keys, and an
-- erasure shreds both, so a Practice asking "how did this record come to
-- be?" after an erasure would otherwise have merged_into pointing
-- somewhere and no readable answer to who did it or when. This is the
-- same rule erasureScope already follows -- describe the act, never a
-- value that was erased.
ALTER TABLE clients ADD COLUMN merged_at timestamptz;
ALTER TABLE clients ADD COLUMN merged_by_staff_id uuid REFERENCES staff (id);

-- Backfilled before the CHECK below, so a database that already ran
-- 00080's absorb still satisfies it. now() is honest about what is
-- known: the row was tombstoned at some point before this migration and
-- the exact moment was never recorded. merged_by_staff_id stays NULL for
-- those rows and the CHECK deliberately does not tie it -- inventing a
-- Staff member would be worse than an absent one.
UPDATE clients SET merged_at = now() WHERE merged_into IS NOT NULL AND merged_at IS NULL;

-- The biconditional keeps the tombstone and its timestamp from ever
-- drifting apart. Both write sites (client.setMergedInto is the only
-- one) set the pair in a single UPDATE, which they must: clients_update's
-- USING clause carries merged_into IS NULL, so the write that tombstones
-- a row is the last write app_runtime will ever make to it.
ALTER TABLE clients ADD CONSTRAINT clients_merge_is_dated CHECK (
    (merged_into IS NULL) = (merged_at IS NULL)
);

-- =====================================================================
-- The freeze, narrowed rather than dropped
-- =====================================================================

-- 00093's engagements_freeze_outcome refuses any change to client_id
-- outright. The refusal stays -- ADR-0015's abuse case is unchanged, and
-- serving a second baby on a Credit already spent is still what a free
-- client_id would buy -- but it now admits the one write a merge makes.
--
-- The condition is a durable tombstone, not a session flag of the
-- app.allow_outcome_correction kind. That is the whole point of writing
-- merged_into BEFORE moving anything: the database verifies from a row
-- that survives the transaction that this Engagement's old Client was
-- absorbed into its new one, rather than trusting a caller that says so.
-- A session variable would let any writer that can SET one move an
-- Engagement anywhere; a tombstone can only be written by the merge
-- endpoint, once, and it names exactly one destination.
--
-- One hop, deliberately not a chain. A merge moves an Engagement from
-- the record being absorbed to the record absorbing it and no further,
-- so a chained tombstone (A into B, later B into C) still has to move
-- A's Engagements one merge at a time, each with its own tombstone.
--
-- The EXISTS reads clients as the invoking role, so clients_select's RLS
-- applies inside it. Both rows are at the caller's own Practice -- a
-- merge across Practices is refused at the endpoint and the two rows
-- could not have collided in the first place -- so the row is visible.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION engagements_freeze_outcome() RETURNS trigger
    LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.client_id IS DISTINCT FROM OLD.client_id
       AND NOT EXISTS (
           SELECT 1 FROM clients
            WHERE id = OLD.client_id
              AND merged_into = NEW.client_id
       ) THEN
        RAISE EXCEPTION 'an Engagement moves between Clients only when the record it belongs to has been merged into the other';
    END IF;
    IF OLD.birth_outcome IS NOT NULL
       AND (NEW.birth_outcome, NEW.pregnancy_ended_on)
           IS DISTINCT FROM (OLD.birth_outcome, OLD.pregnancy_ended_on)
       AND coalesce(current_setting('app.allow_outcome_correction', true), '') <> 'on' THEN
        RAISE EXCEPTION 'a recorded birth outcome is frozen; only a Practice Owner may correct it';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- =====================================================================
-- merge_client_portal_links: the one move no policy admits
-- =====================================================================

-- client_portal_users has three Staff-facing write policies and not one
-- of them admits an accepted row. invite_insert and invite_update (00042)
-- both demand identity_uid IS NULL, and erasure_update (00064) demands
-- the Client already be erased and ends with identity_uid NULL. That is
-- deliberate: an accepted portal account is immutable through the Staff
-- door, because it is the person's own login and not the Practice's to
-- edit. A merge is the one act that has to change which Client record it
-- points at, so it gets a purpose-built door rather than a widened
-- policy -- the same shape portal_account_reuse_for_accept (00081) and
-- portal_account_reaches_a_live_client (00108) already take.
--
-- It answers to the tombstone, exactly as the trigger above does, and it
-- re-derives the Practice itself rather than believing the caller: a
-- SECURITY DEFINER function sees every row, so every tenancy fact it
-- relies on has to be checked inside it.
--
-- Only accepted rows move. A pending invitation on the absorbed record
-- is revoked by the caller instead of moved: an invitation is
-- re-sendable, moving one would collide with
-- client_portal_users_one_pending_per_client (00026) whenever the
-- survivor already has her own, and the merge endpoint already revokes
-- the survivor's pending invite when the folded email differs from the
-- one on file.
--
-- It returns the number of rows it moved so the caller can refuse a
-- silent zero. Under RLS a move that matches nothing is indistinguishable
-- from a move that worked, and that is the failure this whole function
-- exists because of.
-- +goose StatementBegin
CREATE FUNCTION merge_client_portal_links(p_absorbed uuid, p_survivor uuid)
RETURNS integer
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
DECLARE
    moved integer;
BEGIN
    IF NOT EXISTS (
        SELECT 1
          FROM clients a
          JOIN clients s ON s.id = a.merged_into
         WHERE a.id = p_absorbed
           AND a.merged_into = p_survivor
           AND a.practice_id = s.practice_id
           AND a.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
    ) THEN
        RAISE EXCEPTION 'a portal link moves only from a record already merged into the destination, at the caller''s own Practice';
    END IF;

    UPDATE client_portal_users
       SET client_id = p_survivor
     WHERE client_id = p_absorbed
       AND identity_uid IS NOT NULL;
    GET DIAGNOSTICS moved = ROW_COUNT;
    RETURN moved;
END;
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION merge_client_portal_links(uuid, uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION merge_client_portal_links(uuid, uuid) TO app_runtime;

-- =====================================================================
-- redact_absorbed_client: erasure reaches the tombstone
-- =====================================================================

-- clients_update's USING clause (00080) carries merged_into IS NULL, so
-- app_runtime can never write to a tombstone again -- which is correct
-- for every act except the one that is supposed to destroy what it
-- still holds. An absorbed record keeps her name, her email, her
-- address and her own data key, and erasing the woman has to reach all
-- of it.
--
-- Preconditioned on the destination already being erased, so this opens
-- no door that an erasure has not already opened. A chain (A into B,
-- later B into C) is walked outward from the erased record: erasing C
-- admits B, and redacting B stamps its own erased_at, which then admits
-- A. The caller enumerates that order; this function only ever checks
-- one hop.
--
-- merged_into, merged_at and merged_by_staff_id are deliberately left
-- standing. They are the plaintext audit of the merge, and they are
-- exactly what has to outlive the shredding of both keys: after this
-- runs, "who merged which two records, and when" still has an answer
-- while nothing about the woman herself does.
-- +goose StatementBegin
-- It takes the record to redact and nothing else. An earlier shape let
-- the caller pass the replacement name and the timestamp, which is a
-- door onto the one write app_runtime is otherwise forbidden to make:
-- anything holding that role could have stamped a chosen name and a
-- chosen date onto a tombstone. There is exactly one legal end state for
-- this function, so it owns both values. 'Erased Client' is
-- client.ErasedGivenName, and client/merge_moves_test.go asserts the two
-- still agree.
CREATE FUNCTION redact_absorbed_client(p_absorbed uuid)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
          FROM clients a
          JOIN clients s ON s.id = a.merged_into
         WHERE a.id = p_absorbed
           AND a.erased_at IS NULL
           AND s.erased_at IS NOT NULL
           AND a.practice_id = s.practice_id
           AND a.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
    ) THEN
        RAISE EXCEPTION 'an absorbed record is redacted only once, and only after the record it was merged into has been erased';
    END IF;

    UPDATE clients SET
        given_name = 'Erased Client', family_name = NULL, preferred_name = NULL,
        email = NULL, phone = NULL,
        address_line1 = NULL, address_line2 = NULL, address_locality = NULL,
        address_region = NULL, address_postal_code = NULL, date_of_birth = NULL,
        field_values = '{}'::jsonb, erased_at = now()
     WHERE id = p_absorbed;
END;
$$;
-- +goose StatementEnd

REVOKE EXECUTE ON FUNCTION redact_absorbed_client(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION redact_absorbed_client(uuid) TO app_runtime;

-- +goose Down
REVOKE EXECUTE ON FUNCTION redact_absorbed_client(uuid) FROM app_runtime;
DROP FUNCTION redact_absorbed_client(uuid);

REVOKE EXECUTE ON FUNCTION merge_client_portal_links(uuid, uuid) FROM app_runtime;
DROP FUNCTION merge_client_portal_links(uuid, uuid);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION engagements_freeze_outcome() RETURNS trigger
    LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.client_id IS DISTINCT FROM OLD.client_id THEN
        RAISE EXCEPTION 'an Engagement belongs to one Client for life; client_id cannot be changed';
    END IF;
    IF OLD.birth_outcome IS NOT NULL
       AND (NEW.birth_outcome, NEW.pregnancy_ended_on)
           IS DISTINCT FROM (OLD.birth_outcome, OLD.pregnancy_ended_on)
       AND coalesce(current_setting('app.allow_outcome_correction', true), '') <> 'on' THEN
        RAISE EXCEPTION 'a recorded birth outcome is frozen; only a Practice Owner may correct it';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

ALTER TABLE clients DROP CONSTRAINT clients_merge_is_dated;
ALTER TABLE clients DROP COLUMN merged_by_staff_id;
ALTER TABLE clients DROP COLUMN merged_at;
