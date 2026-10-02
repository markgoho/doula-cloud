-- +goose Up
-- #1596, ADR-0017's amendment on #1515: an Engagement Request states a
-- third fact beside the kind and the due date. The asker names the one
-- employee Doula who is on the work, or says there is no Doula yet, and
-- approval attaches the named Doula in the transaction that creates the
-- Engagement.
--
-- NULL is "No Doula yet". It is an answer and not a gap, so the column
-- has no default and no NOT NULL: every Request that exists already was
-- asked before the question existed, named nobody, and reads as "No
-- Doula yet", which is what approving one of them has always done.
--
-- The reference is to staff, the same as requested_by and decided_by. A
-- Membership ends by a DELETE of the practice_memberships row
-- (staffauth's removal) and the staff row stays, so a Request keeps the
-- name of the person it named after she has left. Whether she can still
-- be attached is a rule about her Membership on the day of approval, so
-- the endpoint checks it then (engagementrequest.approve) and no
-- constraint here could.
ALTER TABLE engagement_requests ADD COLUMN doula_staff_id uuid REFERENCES staff (id);

-- +goose Down
ALTER TABLE engagement_requests DROP COLUMN doula_staff_id;
