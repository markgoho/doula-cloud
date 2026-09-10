# Migration safety notes

A safety note says why a row-dependent statement in a migration cannot be broken by the rows a table already holds. It exists because the `migrate` job runs only on a push to trunk: a pull request builds an empty database, so a statement that only *existing rows* can refuse is green on the PR and red on trunk, with `deploy-api` and `deploy-app` stuck behind it (see #1021, and the rule stated at the top of `../rowsafety.go`).

One note per migration, named for the migration file: `00107_portal_account_client_pair_unique.md` covers `../00107_portal_account_client_pair_unique.sql`.

The note lives here rather than inside the migration because a migration that has already applied is goose-checksummed and must not be edited. A new migration may still be written with its proof in the SQL comments — the note is what the guardrail reads, so write it either way.

A note must carry a `## <class>` heading for every class the guardrail reports on that migration, spelled exactly as the guardrail names it, and under each heading the reason those rows cannot break the statement. `guardrail_test.go` fails a note that covers a class the migration does not raise, so a note cannot outlive the statement it justifies.
