package engagement_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"doula-cloud/api/internal/authntest"
	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/idempotency"
	"doula-cloud/api/internal/staffauth"
	"doula-cloud/api/internal/testdb"
)

// newServer mounts this package's whole surface through engagement.Mount,
// the same call main.go makes on the real GatedRouter and
// idempotency.Router, and seeds a live session for uid.
func newServer(t *testing.T, db *testdb.DB, uid string) (srv *httptest.Server, session string) {
	t.Helper()
	mux := http.NewServeMux()
	g := staffauth.NewGatedRouter(mux, db.App)
	ir := idempotency.NewRouter(g, db.App)
	engagement.Mount(g, ir)
	return httptest.NewServer(mux), authntest.SeedSession(t, db.App, uid)
}

const doulaRole = "doula"

// ownerRole is named once so golangci-lint's goconst check doesn't see
// repeated "owner" literals across this package's test surface.
const ownerRole = "owner"

// employeeType is named once for the same reason, shared by every test
// that seeds a Staff member's employment_type.
const employeeType = "employee"

// careCompleteReason is named once for the same reason, shared by every
// transition test that completes an Engagement without caring which of
// the six legal ending reasons it carries.
const careCompleteReason = "care_complete"

// contractorType is named once for the same reason, shared by every test
// that seeds ADR-0008's other employment type.
const contractorType = "contractor"

// employeeDoulaKind is the role-table case label an employee Doula's row
// carries, named once for the same reason -- two role tables in this
// package name her, and one of them branches on the label.
const employeeDoulaKind = "employee doula"
