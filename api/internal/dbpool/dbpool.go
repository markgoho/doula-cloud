// Package dbpool caps doula-api's Postgres pool so every instance Cloud
// Run may start, doula-ops's pool and the migrate login fit together
// under doula-cloud-pg's max_connections (#2432).
package dbpool

import (
	"database/sql"
	"time"
)

// The connection budget on doula-cloud-pg. MaxConnections is Cloud SQL's
// default for a db-f1-micro ("tiny (~0.5 GB)": 25), with no
// max_connections flag set; ReservedConnections is Postgres's
// superuser_reserved_connections default, which no application role can
// use. APIPerInstance is what is left once doula-ops and migrate take
// their share, split across every doula-api instance.
const (
	MaxConnections        = 25
	ReservedConnections   = 3
	MigrateLogins         = 1
	OpsPoolShare          = 2
	APIMaxInstances       = 4
	APIRequestConcurrency = 8
	IdleTimeout           = 5 * time.Minute

	APIPerInstance = (MaxConnections - ReservedConnections - MigrateLogins - OpsPoolShare) / APIMaxInstances
)

// Apply caps db at this instance's share of the budget and closes an
// idle connection after IdleTimeout.
func Apply(db *sql.DB) {
	db.SetMaxOpenConns(APIPerInstance)
	db.SetMaxIdleConns(APIPerInstance)
	db.SetConnMaxIdleTime(IdleTimeout)
}
