// Package migrations embeds the goose migration files so they can be
// applied programmatically (e.g. by the testcontainers-go test harness)
// without depending on a filesystem path at runtime.
package migrations

import "embed"

// FS holds the embedded goose migration files.
//
//go:embed *.sql
var FS embed.FS
