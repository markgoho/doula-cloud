-- +goose Up
-- #264 (RA-G6)'s Clients-list rollup reads "the latest Invoice on this
-- Engagement's Contract" per open Engagement, LATERAL-joined against
-- contracts.id directly (api/internal/client/list.go) -- invoices
-- carries no index on contract_id at all (00024_invoices.sql only added
-- the primary key and stripe_invoice_id's own unique constraint, because
-- the only reader then was the per-Engagement Invoice history
-- (payments/invoice.go's listInvoicesQuery), which joins invoices to
-- contracts the other way -- filtering on contracts.engagement_id, never
-- on invoices.contract_id alone). Without this index, the rollup's
-- LATERAL join sequential-scans invoices once per open Engagement on the
-- page. created_at DESC matches the "newest Invoice" ORDER BY ... LIMIT 1
-- the rollup runs, so this is a bounded index scan rather than a
-- scan-then-sort.
CREATE INDEX invoices_contract_created_idx
    ON invoices (contract_id, created_at DESC);

-- +goose Down
DROP INDEX invoices_contract_created_idx;
