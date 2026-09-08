-- +goose Up
-- #971: a Doula who needs a signed Contract voided cannot void it herself
-- (#970/#282 -- void is Owner and Admin only, refused to every Doula
-- regardless of employment type or attachment). This gives her a way to
-- ask instead of only messaging someone and hoping it lands: a request
-- naming who asked, when, and why, an Owner or Admin acts on it (voids
-- the Contract) or declines it with a reason of their own, and the
-- requester can tell which outcome she got.
--
-- A separate table rather than a status on contracts: a request is a
-- fact about *asking*, not about the Contract's own lifecycle
-- (contract_status, 00016_contracts.sql/00018), and it must survive
-- being declined without moving the Contract at all -- a status column
-- on contracts would need a fourth "void requested" value that isn't a
-- real lifecycle stage and would collide with a second concurrent
-- request from a different Doula.
CREATE TYPE contract_void_request_status AS ENUM ('open', 'voided', 'declined');

CREATE TABLE contract_void_requests (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    contract_id    uuid NOT NULL REFERENCES contracts (id),
    engagement_id  uuid NOT NULL REFERENCES engagements (id),
    practice_id    uuid NOT NULL REFERENCES practices (id),
    requested_by   uuid NOT NULL REFERENCES staff (id),
    reason         text NOT NULL,
    status         contract_void_request_status NOT NULL DEFAULT 'open',
    decided_by     uuid REFERENCES staff (id),
    decided_at     timestamptz,
    decline_reason text,
    created_at     timestamptz NOT NULL DEFAULT now()
);

-- engagement_id and practice_id are both denormalized off contract_id
-- (reachable via contracts.engagement_id and engagements.practice_id)
-- rather than joined for every read: engagement_id is what
-- PostVoidRequestHandler already has in hand from the path and what a
-- decline needs to re-verify against, and practice_id is what the
-- Owner/Admin practice-wide roll-up (mirroring contracts' own
-- awaiting-signature list, #426) filters and RLS-scopes on without a
-- three-table join.
--
-- The decision columns move together: open carries none of them,
-- resolved carries decided_by/decided_at always, and decline_reason only
-- on the declined branch -- the same "decision matches status" shape
-- engagement_events leaves to its writer, made a CHECK here instead
-- since this table has exactly one writer path per status rather than a
-- generic event log.
ALTER TABLE contract_void_requests ADD CONSTRAINT contract_void_requests_decision_matches_status CHECK (
    (status = 'open' AND decided_by IS NULL AND decided_at IS NULL AND decline_reason IS NULL) OR
    (status = 'voided' AND decided_by IS NOT NULL AND decided_at IS NOT NULL AND decline_reason IS NULL) OR
    (status = 'declined' AND decided_by IS NOT NULL AND decided_at IS NOT NULL AND decline_reason IS NOT NULL)
);

-- "A Contract may not accumulate duplicate open requests from the same
-- person" (#971's own AC): at most one open request per (contract,
-- requester). Two different Doulas may each have one open at once --
-- nothing in the ticket asks for a single practice-wide slot, and an
-- employed Doula and a contractor can both reach the same Engagement.
CREATE UNIQUE INDEX contract_void_requests_one_open_per_requester
    ON contract_void_requests (contract_id, requested_by)
    WHERE status = 'open';

-- Backs the practice-wide "void requests waiting on you" roll-up
-- (Owner/Admin), oldest first, the same shape contracts'
-- awaiting-signature index reasoning follows.
CREATE INDEX contract_void_requests_practice_open
    ON contract_void_requests (practice_id, created_at)
    WHERE status = 'open';

-- Backs listing every request (any status) against the current Contract
-- row, newest first -- what ContractResponse embeds so the person who
-- asked can tell a decline from a void (#971's AC).
CREATE INDEX contract_void_requests_contract
    ON contract_void_requests (contract_id, created_at);

GRANT SELECT, INSERT, UPDATE ON contract_void_requests TO app_runtime;
-- No DELETE: a request, like a Contract itself, is a record of what was
-- asked and decided, never erased once made.

ALTER TABLE contract_void_requests ENABLE ROW LEVEL SECURITY;

-- Practice-tier only, the same shape contracts_practice_visibility
-- (00016_contracts.sql) uses -- practice_id lives directly on this row,
-- so no EXISTS join through engagements is needed the way that one
-- needs. Staff-only: a Client never reaches this.
CREATE POLICY contract_void_requests_practice_visibility ON contract_void_requests
    USING (practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid);

-- +goose Down
DROP TABLE contract_void_requests;
DROP TYPE contract_void_request_status;
