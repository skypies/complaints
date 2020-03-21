package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/skypies/util/gcp/gcs"
	"github.com/skypies/util/date"
	
	"github.com/skypies/complaints/complaintdb"
)

// {{{ monthlySummaryReportHandler

// Dumps the monthly report file into Google Cloud Storage.
// Defaults to the previous month; else can specify an explicit year & month.

// ?year=2015&month=09
func monthlySummaryReportHandler(w http.ResponseWriter, r *http.Request) {
	month,year,err := formValueMonthDefaultToPrev(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}	
	countByUser := false
	zipFilter := map[string]int{} // Empty
	now := date.NowInPdt()
	start := time.Date(int(year), time.Month(month), 1, 0,0,0,0, now.Location())
	end   := start.AddDate(0,1,0).Add(-1 * time.Second)

	bucketname := "serfr0-reports"
	filename := start.Format("2006-01-summary.txt")	
	ctx := req2ctx(r)
	cdb := complaintdb.NewDB(ctx)

	if exists,err := gcs.Exists(ctx, bucketname, filename); err != nil {
		http.Error(w, fmt.Sprintf("gcs.Exists=%v for gs://%s/%s (err=%v)", exists,
			bucketname, filename, err), http.StatusInternalServerError)
		return
	} else if exists {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(fmt.Sprintf("OK!\nGCS file %s/%s already exists\n", bucketname, filename)))
		return
	}

	tStart := time.Now()
	str,err := cdb.SummaryReport(start, end, countByUser, zipFilter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	gcsHandle,err := gcs.OpenRW(ctx, bucketname, filename, "text/plain")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	gcsHandle.IOWriter().Write([]byte(str))
	if err := gcsHandle.Close(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(fmt.Sprintf("OK!\nGCS monthly report %s/%s written, took %s",
		bucketname, filename, time.Since(tStart))))
}

// }}}


// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
