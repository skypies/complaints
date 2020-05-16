package main

import(
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"golang.org/x/net/context"

	"github.com/skypies/util/date"
	hw "github.com/skypies/util/handlerware"

	// "github.com/skypies/complaints/config"
	"github.com/skypies/complaints/complaintdb"
)

var(
	templates *template.Template

	emailerUrlStem = "/overnight/emailer"
	bksvStem       = "/overnight/bksv"
)

func init() {
  hw.InitTemplates("app/overnight/web/templates") // Must be relative to module root, i.e. git repo root
	templates = hw.Templates
	
  hw.RequireTls = true
  hw.CtxMakerCallback = req2ctx
	
	http.HandleFunc("/report/summary",                  hw.WithAdmin(summaryReportHandler))

	http.HandleFunc("/overnight/hello1",                helloHandler)
	http.HandleFunc("/overnight/hello2",                hw.WithAdmin(hw.WithoutCtx(helloHandler)))

	http.HandleFunc("/overnight/csv",                   hw.WithAdmin(hw.WithoutCtx(csvHandler)))
	http.HandleFunc("/overnight/monthly-report",        hw.WithAdmin(hw.WithoutCtx(monthlySummaryReportHandler)))
	http.HandleFunc("/overnight/counts",                hw.WithAdmin(hw.WithoutCtx(countsHandler)))

	http.HandleFunc("/overnight/bigquery/day",          hw.WithAdmin(hw.WithoutCtx(publishComplaintsDayHandler)))

	http.HandleFunc(emailerUrlStem+"/yesterday",        hw.WithAdmin(hw.WithoutCtx(emailYesterdayHandler)))

	http.HandleFunc("/overnight/submissions/debug",     hw.WithAdmin(hw.WithoutCtx(SubmissionsDebugHandler)))
	http.HandleFunc("/overnight/submissions/debugcomp", hw.WithAdmin(hw.WithoutCtx(complaintdb.ComplaintDebugHandler)))

	http.HandleFunc(bksvStem+"/scan-dates",             hw.WithAdmin(hw.WithoutCtx(bksvScanDateRangeHandler)))
	http.HandleFunc(bksvStem+"/scan-day",               hw.WithAdmin(hw.WithoutCtx(bksvScanDayHandler)))
	http.HandleFunc(bksvStem+"/scan-yesterday",         hw.WithAdmin(hw.WithoutCtx(bksvScanDayHandler)))
	http.HandleFunc(bksvStem+"/submit-complaint",       hw.WithAdmin(hw.WithoutCtx(bksvSubmitComplaintHandler)))
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Listening on port %s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}

func req2ctx(r *http.Request) context.Context {
	ctx,_ := context.WithTimeout(r.Context(), 599 * time.Second)
	return ctx
}
func req2client(r *http.Request) *http.Client {
	return &http.Client{}
}

// TODO: move to util/date, bump the version
func DayWindows(s,e time.Time) [][]time.Time {
	out := [][]time.Time{}
	s = s.Add(-1*time.Second) // Tip s into previous day, so that it counts as an 'intermediate'
	for _,tMidnight := range date.IntermediateMidnights(s,e) {
		out = append(out, []time.Time{tMidnight, tMidnight.AddDate(0,0,1).Add(-1*time.Second) })
	}
	return out
}

func helloHandler (w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(fmt.Sprintf("OK\nHello Handler for %s\n", r)))

}
