-- +goose Up
-- The Practice-wide Invoice list's unpaid and overdue views (#2746) read
--   WHERE practice_id = $1 AND status = 'open' ... ORDER BY due_at ASC, id ASC
-- with a (due_at, id) > ($2, $3) cursor. invoices_practice_created_idx
-- (00056) serves only the whole book's newest-first order, so these views
-- sorted every open Invoice of the Practice to return thirty.
--
-- Partial on status = 'open': both views hold open Invoices only (the
-- overdue test repeats it), and a paid book never grows this index.
CREATE INDEX invoices_practice_open_due_idx
    ON invoices (practice_id, due_at, id)
    WHERE status = 'open';

-- +goose Down
DROP INDEX invoices_practice_open_due_idx;
