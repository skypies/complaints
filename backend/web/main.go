package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/skypies/complaints/backend"
	"github.com/skypies/complaints/complaintdb"
)

func init() {
	http.HandleFunc("/", noopHandler)
	http.HandleFunc("/_ah/start", noopHandler)
	http.HandleFunc("/_ah/stop", noopHandler)
	http.HandleFunc("/backend/submissions/debug", complaintdb.SubmissionsDebugHandler)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Listening on port %s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}

func noopHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("OK, backend noop\n"))
}
