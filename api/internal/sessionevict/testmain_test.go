package sessionevict_test

import (
	"os"
	"testing"

	"doula-cloud/api/internal/testdb"
)

func TestMain(m *testing.M) {
	os.Exit(testdb.Main(m))
}
