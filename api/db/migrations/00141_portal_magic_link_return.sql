-- +goose Up
-- #2496: a Client who opens a saved portal address signed out is sent to
-- /portal/login, and the address she asked for has to survive the mailed
-- link, possibly opened on another device. It rides here, bound to the
-- magic-link token, never in the mailed URL, so a crafted link cannot
-- name where the session it mints is sent.
--
-- Keyed 1:1 on the auth_tokens row it belongs to, like
-- portal_sign_in_address_changes (00079): authtoken.Mint deletes the
-- prior unspent token on a re-request, and this row cascades with it.
-- identifier cascades off portal_accounts, so an Erasure reaches it.
--
-- The CHECK is the database's copy of clientauth.portalReturnPath: only a
-- path under /portal/engagements/ on this app, never a host or a scheme.
--
-- No RLS, like auth_tokens (00061): it is read at spend time, before
-- any session exists.
CREATE TABLE portal_magic_link_returns (
    token_hash text PRIMARY KEY REFERENCES auth_tokens (token_hash) ON DELETE CASCADE,
    identifier text NOT NULL REFERENCES portal_accounts (identifier) ON DELETE CASCADE,
    return_to  text NOT NULL CHECK (
        return_to LIKE '/portal/engagements/%'
        AND length(return_to) <= 512
        AND position('\' IN return_to) = 0
        AND position('//' IN return_to) = 0
    ),
    created_at timestamptz NOT NULL DEFAULT now()
);

GRANT SELECT, INSERT ON portal_magic_link_returns TO app_runtime;

-- +goose Down
DROP TABLE portal_magic_link_returns;
