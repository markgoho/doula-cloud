// Command doula-cloud-api runs the Go BFF.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	// Registers the "pgx" driver with database/sql; never referenced by name.
	_ "github.com/jackc/pgx/v5/stdlib"

	"doula-cloud/api/internal/authn"
	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/engagement"
	"doula-cloud/api/internal/portal"
	"doula-cloud/api/internal/staffauth"
)

func helloHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	// coverage:ignore reason: response encoding failure, not exercised by unit tests
	if err := json.NewEncoder(w).Encode(map[string]string{"message": "hello world"}); err != nil {
		log.Printf("helloHandler: encode response: %v", err)
	}
}

func resolvePort() string {
	if port := os.Getenv("PORT"); port != "" {
		return port
	}
	return "8080"
}

// practiceSessionResponse confirms to the frontend which Practice the
// caller landed on -- and, as a side effect of running through
// staffauth.Middleware, records it as the Staff member's last-used
// Practice for their next login.
type practiceSessionResponse struct {
	PracticeID   string   `json:"practiceId"`
	PracticeName string   `json:"practiceName"`
	Roles        []string `json:"roles"`
}

func practiceSessionHandler(w http.ResponseWriter, r *http.Request) {
	tx, _ := staffauth.Tx(r.Context())
	staffID, _ := staffauth.StaffID(r.Context())
	practiceID, _ := staffauth.PracticeID(r.Context())

	var name string
	if err := tx.QueryRowContext(r.Context(), `SELECT name FROM practices WHERE id = $1`, practiceID).Scan(&name); err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		return
	}

	roles, err := staffauth.Roles(r.Context(), tx, practiceID, staffID)
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		http.Error(w, staffauth.MsgInternalError, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	// coverage:ignore reason: response encoding failure, not exercised by unit tests
	if err := json.NewEncoder(w).Encode(practiceSessionResponse{PracticeID: practiceID, PracticeName: name, Roles: roles}); err != nil {
		log.Printf("practiceSessionHandler: encode response: %v", err)
	}
}

// routes builds the BFF's route table. verifier and db are threaded
// through so tests can substitute a fake Identity Platform verifier and a
// test Postgres instance instead of the real ones main() wires up.
func routes(verifier authn.Verifier, db *sql.DB) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/hello", helloHandler)
	mux.Handle("POST /api/staff/signup", staffauth.SignupHandler(verifier, db))
	mux.Handle("GET /api/staff/session", staffauth.SessionHandler(verifier, db))
	mux.Handle("POST /api/staff/accept-invite", staffauth.AcceptInviteHandler(verifier, db))
	mux.Handle("GET /api/practices/{practiceId}/session",
		staffauth.Middleware(verifier, db)(http.HandlerFunc(practiceSessionHandler)))
	mux.Handle("POST /api/practices/{practiceId}/invitations",
		staffauth.Middleware(verifier, db)(staffauth.InviteHandler()))
	mux.Handle("PATCH /api/practices/{practiceId}/staff/{staffId}/roles",
		staffauth.Middleware(verifier, db)(staffauth.AssignRolesHandler()))
	mux.Handle("GET /api/practices/{practiceId}/clients",
		staffauth.Middleware(verifier, db)(engagement.ListHandler()))
	mux.Handle("POST /api/practices/{practiceId}/clients",
		staffauth.Middleware(verifier, db)(engagement.CreateHandler()))
	mux.Handle("GET /api/practices/{practiceId}/engagements/{engagementId}",
		staffauth.Middleware(verifier, db)(engagement.DetailHandler()))
	mux.Handle("GET /api/portal/session", clientauth.SessionHandler(verifier, db))
	mux.Handle("GET /api/portal/engagements/{engagementId}",
		clientauth.Middleware(verifier, db)(portal.DetailHandler()))
	return mux
}

func main() {
	port := resolvePort()

	// coverage:ignore reason: requires a real DATABASE_URL and network access, not exercised by unit tests
	db, err := sql.Open("pgx", os.Getenv("DATABASE_URL"))
	if err != nil {
		// coverage:ignore reason: requires a real DATABASE_URL and network access, not exercised by unit tests
		log.Fatalf("open db: %v", err)
	}

	// coverage:ignore reason: requires real GCP credentials and network access, not exercised by unit tests
	verifier, err := authn.NewFirebaseVerifier(context.Background(), os.Getenv("GCP_PROJECT_ID"))
	if err != nil {
		// coverage:ignore reason: requires real GCP credentials and network access, not exercised by unit tests
		log.Fatalf("init verifier: %v", err)
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           routes(verifier, db),
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("listening on port %s", port)
	// coverage:ignore reason: listener startup, not exercised by unit tests
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
