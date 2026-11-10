-- +goose Up
-- A Staff Invitation's lifecycle, made explicit (staffinvitation's
-- lifecycle.go is the Go half). Each stored status now carries exactly
-- the who and when that status means, and nothing else:
--
--   pending   no revoked_by/revoked_at, no accepted_staff_id/accepted_at
--   accepted  accepted_staff_id and accepted_at, no revoked_by/revoked_at
--   revoked   revoked_by and revoked_at, no accepted_staff_id/accepted_at
--
-- 'expired' is in none of the three, so no write can store it again.
-- Expiry is derived -- a pending row whose expires_at has passed
-- (staffinvitation.StateOf) -- because a stored copy disagreed with the
-- derived one: the accept path flipped a stale row to 'expired', which
-- took it off the Staff screen, while an unflipped lapsed row stayed on
-- it. The enum keeps the label because dropping an enum value rewrites
-- the column, and a row trunk's database may still hold in 'expired'
-- would refuse that cast. There is no expired_at column for the same
-- reason there is no stored expiry: expires_at already says when, and
-- nobody acts to make it happen.
--
-- NOT VALID: the constraint holds every row written from here on, without
-- scanning the rows trunk's database already has, one of which may carry
-- the accept-after-revoke race this migration's code closes (an
-- 'accepted' row still holding revoked_by/revoked_at).
ALTER TABLE practice_invitations ADD CONSTRAINT practice_invitations_lifecycle CHECK (
    (status = 'pending'
        AND revoked_by IS NULL AND revoked_at IS NULL
        AND accepted_staff_id IS NULL AND accepted_at IS NULL)
 OR (status = 'accepted'
        AND accepted_staff_id IS NOT NULL AND accepted_at IS NOT NULL
        AND revoked_by IS NULL AND revoked_at IS NULL)
 OR (status = 'revoked'
        AND revoked_by IS NOT NULL AND revoked_at IS NOT NULL
        AND accepted_staff_id IS NULL AND accepted_at IS NULL)
) NOT VALID;

-- +goose Down
ALTER TABLE practice_invitations DROP CONSTRAINT practice_invitations_lifecycle;
