package offer_test

import (
	"os"
	"testing"

	"doula-cloud/api/internal/testdb"
)

// TestMain hands off to testdb.Main so the one Postgres container this
// package's tests share is terminated once at process exit.
func TestMain(m *testing.M) {
	os.Exit(testdb.Main(m))
}
