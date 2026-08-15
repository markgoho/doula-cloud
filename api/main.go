// Command doula-cloud-api runs the Go BFF.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
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

func main() {
	port := resolvePort()

	http.HandleFunc("/hello", helloHandler)

	server := &http.Server{
		Addr:              ":" + port,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("listening on port %s", port)
	// coverage:ignore reason: listener startup, not exercised by unit tests
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
