// Package migrations embeds the goose migration files so they can be
// applied programmatically (e.g. by the testcontainers-go test harness)
// without depending on a filesystem path at runtime.
//
// # The rule every migration in here is held to
//
// A migration's Up section may contain no statement whose success
// depends on the rows a table already holds, unless a safety note in
// safety/ says why those rows cannot break it.
//
// The reason is the shape of CI. The job that applies migrations to a
// real database (migrate, .github/workflows/ci.yml) runs only on a push
// to trunk, alongside deploy-api and deploy-app. Every pull request
// builds an empty Postgres per test process instead, so any statement
// that only existing rows can refuse is green on the PR and red on the
// first trunk push -- and the deploys queued behind migrate never run.
// #1021 is the incident: #967 shipped
// `ALTER TABLE contracts ADD COLUMN amount_cents bigint NOT NULL`, green
// on its PR, and trunk stayed red across seven merges.
//
// RowDependent (rowsafety.go) reports every statement the rule forbids;
// rowClasses there enumerates the family, each member derived from a
// Postgres operation documented as scanning, rewriting or verifying
// existing rows, and each proved against a real populated Postgres in
// rowsafety_pg_test.go. guardrail_test.go, a required PR check, is what
// fails the build. #1139 widened the check from one member of the family
// to all of it.
package migrations

import "embed"

// FS holds the embedded goose migration files.
//
//go:embed *.sql
var FS embed.FS
