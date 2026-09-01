-- +goose Up
-- The Practice-wide Invoice list (#265, gap RA-G7) reads
--   WHERE practice_id = $1 ORDER BY created_at DESC, id DESC LIMIT 31
-- which without this index is a sequential scan of the Practice's whole
-- book followed by a sort, to return thirty rows. 00024_invoices.sql
-- created no index beyond the primary key and the stripe_invoice_id
-- unique constraint, because the only reader then was the per-Engagement
-- history, which joins from contracts.
--
-- Column order matches the query exactly -- practice_id equality first,
-- then the (created_at, id) tuple the cursor compares and the ORDER BY
-- sorts by -- so both the first page and every cursor page are a bounded
-- index scan. The DESC direction is written out rather than left to
-- Postgres's backwards scan so the two-column cursor comparison
-- ((created_at, id) < ($2, $3)) can be satisfied as a single range seek.
--
-- The whole-book totals query (SUM ... FILTER on status) still reads
-- every row of the Practice, which this index also serves as a covering
-- prefix: it is scanned by practice_id rather than the table.
CREATE INDEX invoices_practice_created_idx
    ON invoices (practice_id, created_at DESC, id DESC);

-- +goose Down
DROP INDEX invoices_practice_created_idx;
