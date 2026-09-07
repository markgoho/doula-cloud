package sessionnotice_test

import (
	"os"
	"testing"

	"doula-cloud/api/internal/testdb"
)

// TestMain terminates the shared Postgres container testdb.New starts for
// this test process -- see testdb.Main's doc comment. Missing here until
// now, this package's container was never terminated on process exit,
// clean or not -- found while diagnosing the leaked-container reaper
// (see .claude/hooks/testdb-reap.ts).
func TestMain(m *testing.M) {
	os.Exit(testdb.Main(m))
}
