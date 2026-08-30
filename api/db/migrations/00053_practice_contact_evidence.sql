-- +goose Up
-- #420, the escheatment half. New York reaches an unspent Credit balance
-- under APL 1315(1-b) at three years' dormancy -- the whole balance, with
-- no business-to-business exemption. What stops the clock is recorded
-- contact, and 2 NYCRR 125.1 accepts "a verifiable login by the owner".
--
-- We had no such record that survives. sessions (00028) is swept on
-- expiry, and a request log is rotated -- so the evidence that a Practice
-- was here would have been gone long before the three years it has to
-- cover. This column is that evidence, on the row it describes.
--
-- Retention is decided here rather than inherited: it is kept for the life
-- of the staff row, forever. It is one timestamp per person, the dormancy
-- test it answers is three years long, and a retention period shorter than
-- the obligation is the same as having no record at all.
--
-- Written by staffauth.Middleware once a request has both a resolved
-- Staff member and a confirmed Membership, throttled to at most one write
-- a day per person: it is the practice-context path, so
-- staff_practice_visibility (00002) admits the UPDATE. The sign-in itself
-- cannot write it -- app.current_practice_id is unset there, and the only
-- policy that matches is staff_self_visibility, which is SELECT only.
ALTER TABLE staff ADD COLUMN last_active_at timestamptz;

-- The dormancy sweep asks the same question every time: which Practices
-- hold Credits and have not been seen. It reads this column across a
-- Practice's members, so it is indexed for the ordering, not for a lookup.
CREATE INDEX staff_last_active_at ON staff (last_active_at);

-- +goose Down
DROP INDEX staff_last_active_at;
ALTER TABLE staff DROP COLUMN last_active_at;
