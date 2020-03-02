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

var(
	emailerUrlStem = "/overnight/emailer"
	bksvStem       = "/overnight/bksv"
)

func init() {
	http.HandleFunc("/overnight/submissions/debug", SubmissionsDebugHandler)
	http.HandleFunc("/overnight/submissions/debugcomp", complaintdb.ComplaintDebugHandler)

	http.HandleFunc(emailerUrlStem+"/yesterday",  emailYesterdayHandler)

	http.HandleFunc(bksvStem+"/scan-dates",       bksvScanDateRangeHandler)
	http.HandleFunc(bksvStem+"/scan-day",         bksvScanDayHandler)
	http.HandleFunc(bksvStem+"/scan-yesterday",   bksvScanDayHandler)
	http.HandleFunc(bksvStem+"/submit-complaint", bksvSubmitComplaintHandler)

	// For go111, appengine uses the module root, which is the root of the git repo; so
	// the relative dirname for templates is relative to the root of the git repo.
	// tmpl = widget.ParseRecursive(template.New("").Funcs(ui.TemplateFuncMap()), "app/frontend/templates")
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
