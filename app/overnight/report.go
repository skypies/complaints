package main

import(
	"net/http"
	
	"golang.org/x/net/context"

	"github.com/skypies/util/date"
	"github.com/skypies/util/widget"
	
	"github.com/skypies/complaints/pkg/complaintdb"
)

// {{{ summaryReportHandler

// stop.jetnoise.net/report/summary?date=day&day=2016/05/04&peeps=1

func summaryReportHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	cdb := complaintdb.NewDB(ctx)

	if r.FormValue("date") == "" {
		var params = map[string]interface{}{
			"Title": "Summary of disturbance reports",
			"FormUrl": "/overnight/report/summary",
			"Yesterday": date.NowInPdt().AddDate(0,0,-1),
		}
		//params["Message"] = "Please do not scrape this form. Instead, get in touch !"
		if err := templates.ExecuteTemplate(w, "date-report-form", params); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// See old code in backend/summary.go if you need to bring this back
	//if isBanned(r) {
	//	http.Error(w, "bad user", http.StatusUnauthorized)
	//	return
	//}
	
	start,end,_ := widget.FormValueDateRange(r)
	countByUser := r.FormValue("peeps") != ""
	zipFilter := map[string]int{}
	
	str,err := cdb.SummaryReport(start, end, countByUser, zipFilter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(str))	
}

// }}}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
