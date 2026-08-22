-- +goose Up
-- Redesigns a Practice's Stripe Connect linkage for Accounts v2 (#247).
--
-- 00023 stored what v1's Account object reported: three booleans,
-- charges_enabled / payouts_enabled / details_submitted. v2 reports
-- neither the same facts nor the same shape:
--
--   charges_enabled   -> configuration.merchant.capabilities.card_payments.status
--   payouts_enabled   -> configuration.merchant.capabilities.stripe_balance.payouts.status
--   details_submitted -> no equivalent; `requirements.entries` carries what
--                        is still outstanding
--
-- Both capability statuses are four-valued (active / pending / restricted /
-- unsupported), and "pending" is the case a boolean cannot express at all --
-- Stripe is reviewing, the Owner has nothing left to do, and the account is
-- neither connected nor not-connected. So these are new columns, not renamed
-- ones. Pre-launch (CLAUDE.md): no rows to migrate, so the old columns are
-- dropped outright rather than backfilled.
--
-- The status columns default to 'unsupported' -- the v2 value meaning the
-- capability was never granted -- so a freshly-created account reads as
-- not-yet-usable before its first webhook delivery, the same intent 00023's
-- `DEFAULT false` had.
--
-- stripe_connect_requirements_due holds the `description` of every
-- requirements entry still awaiting the account holder (dotted field paths
-- like "configuration.merchant.mcc"). It is what replaces details_submitted:
-- empty means nothing outstanding. These are Stripe field names, never
-- Client data, so the no-PHI rule (#30/#78) is unaffected.
ALTER TABLE practices DROP COLUMN stripe_connect_charges_enabled;
ALTER TABLE practices DROP COLUMN stripe_connect_payouts_enabled;
ALTER TABLE practices DROP COLUMN stripe_connect_details_submitted;

ALTER TABLE practices ADD COLUMN stripe_connect_card_payments_status text NOT NULL DEFAULT 'unsupported'
	CHECK (stripe_connect_card_payments_status IN ('active', 'pending', 'restricted', 'unsupported'));
ALTER TABLE practices ADD COLUMN stripe_connect_payouts_status text NOT NULL DEFAULT 'unsupported'
	CHECK (stripe_connect_payouts_status IN ('active', 'pending', 'restricted', 'unsupported'));
ALTER TABLE practices ADD COLUMN stripe_connect_requirements_due text[] NOT NULL DEFAULT '{}';

-- Who last moved this Practice's Connect state, and when. Every write is the
-- Connect account webhook applying a capability_status_updated event, so the
-- actor is Stripe rather than a Staff member -- the event id is what makes
-- the change traceable back to a specific delivery (CLAUDE.md's audit-trail
-- expectation).
ALTER TABLE practices ADD COLUMN stripe_connect_status_event_id text;
ALTER TABLE practices ADD COLUMN stripe_connect_status_updated_at timestamptz;

-- +goose Down
ALTER TABLE practices DROP COLUMN stripe_connect_status_updated_at;
ALTER TABLE practices DROP COLUMN stripe_connect_status_event_id;
ALTER TABLE practices DROP COLUMN stripe_connect_requirements_due;
ALTER TABLE practices DROP COLUMN stripe_connect_payouts_status;
ALTER TABLE practices DROP COLUMN stripe_connect_card_payments_status;

ALTER TABLE practices ADD COLUMN stripe_connect_charges_enabled boolean NOT NULL DEFAULT false;
ALTER TABLE practices ADD COLUMN stripe_connect_payouts_enabled boolean NOT NULL DEFAULT false;
ALTER TABLE practices ADD COLUMN stripe_connect_details_submitted boolean NOT NULL DEFAULT false;
