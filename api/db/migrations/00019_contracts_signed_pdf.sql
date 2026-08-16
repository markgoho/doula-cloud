-- +goose Up
-- Adds the object-store reference to the Signed PDF a Contract's sent ->
-- signed transition (#70, sign.go) renders and persists (#71): the
-- permanent record of what was actually agreed to, never re-rendered
-- after signing even if the Contract Template or rendering code later
-- changes. Nullable -- every pre-signed row (draft/sent) has none, and a
-- signed row always gets one atomically in the same UPDATE that sets
-- status = 'signed', so a NULL here on a signed row would mean the write
-- half-failed. No new RLS policy: the existing practice/client SELECT
-- policies (00016_contracts.sql, 00017_contracts_client_visibility.sql)
-- already scope which rows are visible; this is just another column on
-- an already-visible row.

ALTER TABLE contracts
    ADD COLUMN signed_pdf_object_path text;

-- +goose Down
ALTER TABLE contracts
    DROP COLUMN signed_pdf_object_path;
