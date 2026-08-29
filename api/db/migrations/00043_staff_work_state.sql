-- +goose Up
-- Where each Staff member works (#415), and the audit trail for it.
--
-- New York sources a sale of remotely accessed software to "the location
-- from which the purchaser uses or directs the use of the software"
-- (TB-ST-128), and where a Practice's people are split across states it
-- says to "collect tax based on the portion of the receipt attributable
-- to the users located in New York." So the taxable share of a Credit
-- purchase is a headcount -- New York-located Staff over all Staff --
-- and nothing in this schema could answer it: every existing "address"
-- column is an email address.
--
-- This is a fact about a person, not about a Membership, so it lives on
-- staff. A person works from one place at a time; a per-Membership copy
-- could only ever be the same value repeated, or two Practices
-- disagreeing about where the same contractor doula sits.
--
-- Deliberately a state and nothing more. A doula's home address would
-- serve payroll and contractor payment (#391, both out of this map's
-- scope) and would be the wrong fact anyway -- what is wanted is where
-- she *works*, which for a doula who commutes is not where she lives.
-- Clients carry a full structured address (00042) because a doula
-- travels to them; nothing travels to the doula.
--
-- The value is self-reported and nothing verifies it -- not against the
-- Client addresses 00042 stores, not against anything. The apportionment
-- New York is owed therefore rests on an unverified assertion, and this
-- table plus the price and tax recorded on the purchase row (#420) are
-- its whole substantiation. Stated here so an audit meets a documented
-- weakness rather than an undiscovered one.
-- ---------------------------------------------------------------------

ALTER TABLE staff ADD COLUMN work_state text;

-- Every row that exists today is a development fixture -- Doula Cloud
-- has no production data (CLAUDE.md) -- so this value is arbitrary
-- rather than wrong. It is backfilled only so the column can be NOT
-- NULL from here on: a nullable one would force every apportionment
-- query to answer "and what if we don't know?", which is exactly the
-- silently-wrong-tax failure the column exists to prevent.
UPDATE staff SET work_state = 'NY' WHERE work_state IS NULL;

ALTER TABLE staff ALTER COLUMN work_state SET NOT NULL;

-- The 50 states and the District of Columbia, as USPS two-letter
-- abbreviations. A Staff member working outside the US, or from Puerto
-- Rico, cannot be recorded and is refused at onboarding rather than
-- mis-taxed -- the safe direction, but a real refusal, and the first
-- pilot Practice to hit it reopens this list.
ALTER TABLE staff ADD CONSTRAINT staff_work_state_usps CHECK (
    work_state IN (
        'AL','AK','AZ','AR','CA','CO','CT','DE','DC','FL',
        'GA','HI','ID','IL','IN','IA','KS','KY','LA','ME',
        'MD','MA','MI','MN','MS','MO','MT','NE','NV','NH',
        'NJ','NM','NY','NC','ND','OH','OK','OR','PA','RI',
        'SC','SD','TN','TX','UT','VT','VA','WA','WV','WI','WY'
    )
);

-- When the current value was last asserted, and by whom it is read. The
-- roster prints it as "New York -- self-reported 28 Aug 2026", which is
-- the whole answer to "how did this get set?" for a Practice that
-- inherited the value from an earlier one: only the person herself may
-- write it, so the actor is never in doubt, and an untouched value shows
-- its own age without any staleness machinery.
ALTER TABLE staff ADD COLUMN work_state_reported_at timestamptz NOT NULL DEFAULT now();

COMMENT ON COLUMN staff.work_state IS
    'The US state this person works from -- the fact NY sales tax apportionment is computed on (TB-ST-128). Not a mailing address, and not necessarily where she lives.';

-- ---------------------------------------------------------------------
-- staff_work_state_events: how the current value came to be. The same
-- shape practice_membership_events (00039) has, for the same reason --
-- CLAUDE.md's audit-trail expectation -- and, unlike that table, one
-- whose rows a tax return may need years later: an ST-100 is
-- substantiated from what was true on the purchase date, not from what
-- is true when the auditor asks.
--
-- previous_work_state is NULL for the first event, which is the one
-- written at onboarding. The founding Owner gets that row too, the same
-- way 00039 gives her a 'joined' Membership event -- she is her own
-- actor.
--
-- actor_staff_id exists rather than being assumed to equal staff_id
-- because "who changed this?" must stay answerable if a future ticket
-- ever widens the write. Today only the person herself may write, so the
-- two are always equal, and a row where they differ is a signal.
--
-- No UPDATE, no DELETE grant: an event is a fact about the past.
-- ---------------------------------------------------------------------
CREATE TABLE staff_work_state_events (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    staff_id            uuid NOT NULL REFERENCES staff (id),
    previous_work_state text,
    work_state          text NOT NULL,
    actor_staff_id      uuid NOT NULL REFERENCES staff (id),
    created_at          timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX staff_work_state_events_staff
    ON staff_work_state_events (staff_id, created_at);

GRANT SELECT, INSERT ON staff_work_state_events TO app_runtime;

ALTER TABLE staff_work_state_events ENABLE ROW LEVEL SECURITY;

-- The table carries no practice_id -- it is a fact about a person, like
-- staff itself -- so its practice-tier visibility is the same EXISTS
-- subquery staff_practice_visibility (00002) uses: an event is visible
-- if the person it is about holds a Membership at the current Practice.
CREATE POLICY staff_work_state_events_practice_visibility ON staff_work_state_events
    USING (
        EXISTS (
            SELECT 1 FROM practice_memberships pm
            WHERE pm.staff_id = staff_work_state_events.staff_id
              AND pm.practice_id = NULLIF(current_setting('app.current_practice_id', true), '')::uuid
        )
    );

-- +goose Down
DROP TABLE staff_work_state_events;
ALTER TABLE staff DROP COLUMN work_state_reported_at;
ALTER TABLE staff DROP CONSTRAINT staff_work_state_usps;
ALTER TABLE staff DROP COLUMN work_state;
