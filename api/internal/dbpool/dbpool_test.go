package dbpool_test

import (
	"database/sql"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"doula-cloud/api/internal/dbpool"
)

func TestApplyCapsOpenConnectionsAtTheAPIShare(t *testing.T) {
	db, err := sql.Open("pgx", "postgres://nobody@127.0.0.1:1/none")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	dbpool.Apply(db)

	if got := db.Stats().MaxOpenConnections; got != 4 {
		t.Fatalf("MaxOpenConnections = %d, want 4", got)
	}
}

func TestApplyOpsCapsOpenConnectionsAtTheOpsShare(t *testing.T) {
	db, err := sql.Open("pgx", "postgres://nobody@127.0.0.1:1/none")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	dbpool.ApplyOps(db)

	if got := db.Stats().MaxOpenConnections; got != 2 {
		t.Fatalf("MaxOpenConnections = %d, want 2", got)
	}
}

func TestEveryPoolFitsUnderMaxConnections(t *testing.T) {
	usable := dbpool.MaxConnections - dbpool.ReservedConnections
	used := dbpool.APIMaxInstances*dbpool.APIPerInstance + dbpool.OpsMaxInstances*dbpool.OpsPerInstance + dbpool.MigrateLogins
	if used > usable {
		t.Fatalf("%d doula-api instances x %d + %d doula-ops instances x %d + migrate %d = %d, over the %d usable connections",
			dbpool.APIMaxInstances, dbpool.APIPerInstance, dbpool.OpsMaxInstances, dbpool.OpsPerInstance, dbpool.MigrateLogins, used, usable)
	}
	if dbpool.OpsMaxInstances*dbpool.OpsPerInstance > dbpool.OpsPoolShare {
		t.Fatalf("%d doula-ops instances x %d is over its share of %d", dbpool.OpsMaxInstances, dbpool.OpsPerInstance, dbpool.OpsPoolShare)
	}
	if dbpool.OpsPerInstance < 1 {
		t.Fatalf("OpsPerInstance = %d; database/sql reads 0 as no limit at all", dbpool.OpsPerInstance)
	}
	if dbpool.APIPerInstance < 1 {
		t.Fatalf("APIPerInstance = %d; database/sql reads 0 as no limit at all", dbpool.APIPerInstance)
	}
}

func TestTerraformScalesDoulaAPINoWiderThanTheBudget(t *testing.T) {
	raw, err := os.ReadFile("../../../terraform/cloud_run.tf")
	if err != nil {
		t.Fatalf("read terraform: %v", err)
	}
	src := string(raw)
	start := strings.Index(src, `resource "google_cloud_run_v2_service" "doula_api" {`)
	if start < 0 {
		t.Fatal("doula_api service not found in terraform/cloud_run.tf")
	}
	block := src[start:]
	if end := strings.Index(block, "\n}\n"); end >= 0 {
		block = block[:end]
	}
	counts := regexp.MustCompile(`max_instance_count\s*=\s*(\d+)`).FindAllStringSubmatch(block, -1)
	if len(counts) == 0 {
		t.Fatal("doula_api declares no max_instance_count")
	}
	for _, m := range counts {
		n, _ := strconv.Atoi(m[1])
		if n > dbpool.APIMaxInstances {
			t.Errorf("doula_api max_instance_count = %d, over dbpool.APIMaxInstances = %d", n, dbpool.APIMaxInstances)
		}
	}
	concurrency := regexp.MustCompile(`max_instance_request_concurrency\s*=\s*(\d+)`).FindStringSubmatch(block)
	if concurrency == nil || concurrency[1] != strconv.Itoa(dbpool.APIRequestConcurrency) {
		t.Errorf("doula_api max_instance_request_concurrency = %v, want dbpool.APIRequestConcurrency = %d", concurrency, dbpool.APIRequestConcurrency)
	}
}

func TestTerraformScalesDoulaOpsNoWiderThanTheBudget(t *testing.T) {
	raw, err := os.ReadFile("../../../terraform/cloud_run.tf")
	if err != nil {
		t.Fatalf("read terraform: %v", err)
	}
	src := string(raw)
	start := strings.Index(src, `resource "google_cloud_run_v2_service" "doula_ops" {`)
	if start < 0 {
		t.Fatal("doula_ops service not found in terraform/cloud_run.tf")
	}
	block := src[start:]
	if end := strings.Index(block, "\n}\n"); end >= 0 {
		block = block[:end]
	}
	counts := regexp.MustCompile(`max_instance_count\s*=\s*(\d+)`).FindAllStringSubmatch(block, -1)
	if len(counts) < 2 {
		t.Fatalf("doula_ops declares %d max_instance_count; want both the service's and the template's", len(counts))
	}
	for _, m := range counts {
		n, _ := strconv.Atoi(m[1])
		if n > dbpool.OpsMaxInstances {
			t.Errorf("doula_ops max_instance_count = %d, over dbpool.OpsMaxInstances = %d", n, dbpool.OpsMaxInstances)
		}
	}
}
