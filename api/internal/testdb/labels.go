package testdb

import (
	"os"
	"strconv"

	"github.com/testcontainers/testcontainers-go"
)

// The labels every container a Go test starts carries, so
// .claude/hooks/testdb-reap.ts can tell an orphan from a live run
// without guessing (#2096). Ryuk stays disabled under Podman
// (docs/testing/api.md), so a test process killed before its teardown
// leaves its container behind; the reaper removes a container whose
// OwnerLabel is "true" and whose OwnerPIDLabel names a process that no
// longer exists, and keeps every other container.
const (
	// OwnerLabel marks a container as one a Go test started. Its value
	// is always "true".
	OwnerLabel = "cloud.doula.test-container"
	// OwnerPIDLabel holds the pid of the test process that started the
	// container, in decimal.
	OwnerPIDLabel = "cloud.doula.test-container.pid"
)

// OwnerLabels returns the request option that writes OwnerLabel and
// OwnerPIDLabel onto a container. Pass it to every postgres.Run (or
// other testcontainers start) in a Go test.
func OwnerLabels() testcontainers.CustomizeRequestOption {
	return testcontainers.WithLabels(map[string]string{
		OwnerLabel:    "true",
		OwnerPIDLabel: strconv.Itoa(os.Getpid()),
	})
}
