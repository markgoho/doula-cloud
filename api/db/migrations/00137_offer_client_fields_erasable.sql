-- +goose Up
-- #2385: a Client's erasure (ADR-0027) redacts an Offer's copy of her
-- initial, area and due date in place, so the three columns 00030 made
-- NOT NULL now take NULL. The handlers still require all three at send.
ALTER TABLE engagement_offers ALTER COLUMN client_first_initial DROP NOT NULL;
ALTER TABLE engagement_offers ALTER COLUMN client_area DROP NOT NULL;
ALTER TABLE engagement_offers ALTER COLUMN due_date DROP NOT NULL;

-- +goose Down
ALTER TABLE engagement_offers ALTER COLUMN due_date SET NOT NULL;
ALTER TABLE engagement_offers ALTER COLUMN client_area SET NOT NULL;
ALTER TABLE engagement_offers ALTER COLUMN client_first_initial SET NOT NULL;
