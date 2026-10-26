package testdb_test

import (
	"os"
	"strconv"
	"testing"

	"github.com/testcontainers/testcontainers-go"

	"doula-cloud/api/internal/testdb"
)

// TestOwnerLabelsNameTheTestProcess pins the contract
// .claude/hooks/testdb-reap.ts reads: the marker is "true" and the pid
// is this process's own, in decimal.
func TestOwnerLabelsNameTheTestProcess(t *testing.T) {
	req := testcontainers.GenericContainerRequest{}
	if err := testdb.OwnerLabels().Customize(&req); err != nil {
		t.Fatalf("customize request: %v", err)
	}
	if got := req.Labels[testdb.OwnerLabel]; got != "true" {
		t.Errorf("%s = %q, want %q", testdb.OwnerLabel, got, "true")
	}
	if got, want := req.Labels[testdb.OwnerPIDLabel], strconv.Itoa(os.Getpid()); got != want {
		t.Errorf("%s = %q, want %q", testdb.OwnerPIDLabel, got, want)
	}
}
