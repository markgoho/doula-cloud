-- +goose Up
-- #1166: a Practice states its own zone, so the stopgap default goes.
--
-- 00110_practice_timezone.sql landed practices.timezone NOT NULL DEFAULT
-- 'America/New_York' and said in its own comment why the default stayed
-- rather than taking guardrail_test.go's usual DEFAULT-then-DROP form:
-- nothing on the signup path asked for a zone, so every INSERT into
-- practices would have failed the moment it went away. That is no longer
-- true. Self-signup now asks the founder which zone her Practice keeps
-- its calendar days in and writes it on the INSERT, and an Owner or
-- Admin can change it afterwards, so a Practice's clock is something a
-- person states rather than something Doula Cloud picks (ADR-0036).
--
-- Dropping the default is deliberately the whole of it. Every existing
-- row keeps the value it holds -- DROP DEFAULT changes what a future
-- INSERT that omits the column does, not any row already written -- and
-- an INSERT that omits it now fails loudly against the NOT NULL rather
-- than quietly inheriting New York. Pre-launch there is no such row to
-- reconsider anyway.
--
-- Row safety: ALTER COLUMN ... DROP DEFAULT is catalog-only. It consults
-- no existing row, so it is outside every one of rowsafety.go's classes
-- and needs no safety note -- it is in fact the second half of the
-- remedy that file prescribes for the NOT-NULL-without-DEFAULT class.
ALTER TABLE practices ALTER COLUMN timezone DROP DEFAULT;

-- +goose Down
ALTER TABLE practices ALTER COLUMN timezone SET DEFAULT 'America/New_York';
