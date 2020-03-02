package main

import(
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"golang.org/x/net/context"
	
	"github.com/skypies/complaints/complaintdb"
)

func init() {
	http.HandleFunc("/tmp/submissions/debug", complaintdb.SubmissionsDebugHandler)
	http.HandleFunc("/tmp/submissions/debug2", complaintdb.SubmissionsDebugHandler2)
	http.HandleFunc("/tmp/cdb/comp/debug", complaintdb.ComplaintDebugHandler)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}

func req2ctx(r *http.Request) context.Context {
	ctx,_ := context.WithTimeout(r.Context(), 599 * time.Second)
	return ctx
}
func req2client(r *http.Request) *http.Client {
	return &http.Client{}
}
