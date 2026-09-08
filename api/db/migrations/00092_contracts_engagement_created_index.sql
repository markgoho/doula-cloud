-- +goose Up
-- Indexes the "latest Contract for this Engagement" lookup that both
-- fetchContract (contract.go) and serveSignedPDF (signed_pdf.go) do.
--
-- Until #299, every such read carried a status predicate, so the planner
-- could reach contracts_engagement_id_active_key
-- (00020_contracts_recreate_after_void.sql). That index is partial --
-- WHERE status <> 'voided' -- and #299 removed the status comparison from
-- the Signed PDF read on purpose: a voided Contract's PDF is preserved
-- evidence and must still be served. So the rows the fix exists to reach
-- are exactly the rows that index excludes, and the lookup had nothing
-- left to use.
--
-- (engagement_id, created_at DESC, id DESC) matches both reads' ORDER BY
-- exactly, so the row comes back from an index scan with no sort. The id
-- column is in the key rather than only in the heap because it is the
-- tie-break both reads carry: two Contracts created in the same instant
-- must resolve the same way for the JSON read and for the PDF, or an
-- Engagement's Signed PDF could belong to a different Contract than the
-- one its Contract screen shows.
CREATE INDEX contracts_engagement_created_idx
    ON contracts (engagement_id, created_at DESC, id DESC);

-- +goose Down
DROP INDEX contracts_engagement_created_idx;
