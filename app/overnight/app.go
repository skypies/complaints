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

	"github.com/skypies/complaints/config"
	"github.com/skypies/complaints/ui"
	"github.com/skypies/complaints/complaintdb"
)

var(
	templates *template.Template

	emailerUrlStem = "/overnight/emailer"
	bksvStem       = "/overnight/bksv"
)

func init() {
	http.HandleFunc("/report/summary", summaryReportHandler)
	
	http.HandleFunc("/overnight/csv", csvHandler)
	http.HandleFunc("/overnight/monthly-report", monthlySummaryReportHandler)
	http.HandleFunc("/overnight/counts", countsHandler)
	
	http.HandleFunc("/overnight/bigquery/day", publishComplaintsDayHandler)
	// http.HandleFunc("/overnight/bigquery/all", publishComplaintsAllHandler)
	
	http.HandleFunc(emailerUrlStem+"/yesterday",  emailYesterdayHandler)

	http.HandleFunc("/overnight/submissions/debug", SubmissionsDebugHandler)
	http.HandleFunc("/overnight/submissions/debugcomp", complaintdb.ComplaintDebugHandler)

	http.HandleFunc(bksvStem+"/scan-dates",       bksvScanDateRangeHandler)
	http.HandleFunc(bksvStem+"/scan-day",         bksvScanDayHandler)
	http.HandleFunc(bksvStem+"/scan-yesterday",   bksvScanDayHandler)
	http.HandleFunc(bksvStem+"/submit-complaint", bksvSubmitComplaintHandler)

	templates = ui.LoadTemplates("app/overnight/web/templates")
	ui.InitSessionStore(config.Get("sessions.key"), config.Get("sessions.prevkey"))
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

// TODO: move to util/date, bump the version
func DayWindows(s,e time.Time) [][]time.Time {
	out := [][]time.Time{}
	s = s.Add(-1*time.Second) // Tip s into previous day, so that it counts as an 'intermediate'
	for _,tMidnight := range date.IntermediateMidnights(s,e) {
		out = append(out, []time.Time{tMidnight, tMidnight.AddDate(0,0,1).Add(-1*time.Second) })
	}
	return out
}
