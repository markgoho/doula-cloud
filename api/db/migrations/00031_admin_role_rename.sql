-- +goose Up
-- ADR-0008: office_manager renamed to admin. Glossary parity only --
-- CONTEXT.md's Admin entry already used the word; the enum did not.
ALTER TYPE practice_role RENAME VALUE 'office_manager' TO 'admin';

-- +goose Down
ALTER TYPE practice_role RENAME VALUE 'admin' TO 'office_manager';
