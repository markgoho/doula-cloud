-- +goose Up
-- +goose NO TRANSACTION
-- #892: a Staff person deleting her own login. ADR-0033 records the
-- rules; this is the schema half, split in two the way 00071/00072 split
-- theirs -- ALTER TYPE ... ADD VALUE cannot run inside a transaction
-- block, and the CHECK constraint that has to name the new value cannot
-- run in the same transaction that added it. 00101 is the second half.
--
-- deleted_at is to a staff row exactly what clients.erased_at (00064) is
-- to a Client's: proof the act ran, and the gate that stops it running
-- twice. A hard DELETE FROM staff is not merely unwise here, it is
-- impossible -- sixteen migrations carry a foreign key to staff (id),
-- activity.actor_staff_id among them, and activity holds GRANT SELECT,
-- INSERT and no DELETE. So the row keeps its id forever and every
-- actor_staff_id, offered_by, decided_by, ended_by and requested_by
-- still resolves, to a row that no longer names a person.
ALTER TABLE staff ADD COLUMN deleted_at timestamptz;

-- Partial rather than plain: the column is NULL for every live Staff
-- person and the only reads are "is this one deleted?" at a send-time
-- recheck, which the row lookup already answers -- this index exists so
-- an operator asking "who has deleted a login?" scans the deleted rows
-- rather than the table.
CREATE INDEX staff_deleted_idx ON staff (deleted_at) WHERE deleted_at IS NOT NULL;

-- The act itself, recorded person-scoped rather than Practice-scoped.
-- 00062's own comment gives the reason and it applies here word for
-- word: activity.practice_id is NOT NULL and its INSERT policy needs
-- app.current_practice_id, but deleting a login is a fact about a
-- person. A person whose Memberships were all removed by Owners belongs
-- to no Practice at all, so an activity-per-Practice recording of this
-- act would leave it unrecorded for exactly the person with the least
-- else on file. She is her own actor, the same shape 'self_service'
-- already has.
ALTER TYPE staff_auth_event_reason ADD VALUE 'login_deleted';

-- +goose Down
-- Postgres has no ALTER TYPE ... DROP VALUE. Reversing the enum means
-- rebuilding it, which 00101's Down does -- it is the half that can run
-- in a transaction. This half drops only what it added.
DROP INDEX staff_deleted_idx;
ALTER TABLE staff DROP COLUMN deleted_at;
