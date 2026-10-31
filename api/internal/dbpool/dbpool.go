// Package dbpool caps doula-api's and doula-ops's Postgres pools so every
// instance Cloud Run may start of each, and the migrate login, fit
// together under doula-cloud-pg's max_connections (#2432, #2468).
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
// their share, split across every doula-api instance. OpsPoolShare is
// doula-ops's whole share, so it runs one instance at most:
//
//	usable  = 25 - 3                 = 22
//	doula-api 4 instances x 4        = 16  ((22 - 1 - 2) / 4 = 4)
//	doula-ops 1 instance  x 2        =  2
//	migrate                          =  1
//	total                            = 19 of 22
const (
	MaxConnections        = 25
	ReservedConnections   = 3
	MigrateLogins         = 1
	OpsPoolShare          = 2
	OpsMaxInstances       = 1
	APIMaxInstances       = 4
	APIRequestConcurrency = 8
	IdleTimeout           = 5 * time.Minute

	APIPerInstance = (MaxConnections - ReservedConnections - MigrateLogins - OpsPoolShare) / APIMaxInstances
	OpsPerInstance = OpsPoolShare / OpsMaxInstances
)

// Apply caps db at this doula-api instance's share of the budget and
// closes an idle connection after IdleTimeout.
func Apply(db *sql.DB) {
	capAt(db, APIPerInstance)
}

// ApplyOps caps db at this doula-ops instance's share of the budget and
// closes an idle connection after IdleTimeout.
func ApplyOps(db *sql.DB) {
	capAt(db, OpsPerInstance)
}

func capAt(db *sql.DB, n int) {
	db.SetMaxOpenConns(n)
	db.SetMaxIdleConns(n)
	db.SetConnMaxIdleTime(IdleTimeout)
}
