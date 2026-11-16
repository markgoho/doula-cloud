-- +goose Up
-- contracts.reminded_at (00182) was set when a reminder was queued, so a
-- reminder whose mail never went out still read as "reminded" and held the
-- Client's day (#2978). When and whether a reminder was sent is the
-- contract_notice_outbox row's: its created_at, its status and
-- triggered_by_staff_id. The column is a second, wrong record of it.
ALTER TABLE contracts DROP COLUMN reminded_at;

-- +goose Down
ALTER TABLE contracts ADD COLUMN reminded_at timestamptz;
