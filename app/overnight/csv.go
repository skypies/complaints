package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/skypies/util/date"
	"github.com/skypies/util/gcp/gcs"
	"github.com/skypies/util/widget"

	"github.com/skypies/complaints/complaintdb"
)

// {{{ csvHandler

// Dumps the monthly CSV file into Google Cloud Storage, ready to be emailed to BKSV et al.
// Defaults to the previous month; else can specify an explicit year & month.

// https://overnight-dot-serfr0-1000.appspot.com/overnight/csv
//   ?year=2016&month=4
//   ?date=range&range_from=2006/01/01&range_to=2018/01/01


func csvHandler(w http.ResponseWriter, r *http.Request) {
	ctx := req2ctx(r)
	cdb := complaintdb.NewDB(ctx)
	tStart := time.Now()

	var s,e time.Time
	
	if r.FormValue("date") == "range" {
		s,e,_ = widget.FormValueDateRange(r)

	} else {
		month,year,err := formValueMonthDefaultToPrev(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	
		now := date.NowInPdt()
		s = time.Date(int(year), time.Month(month), 1, 0,0,0,0, now.Location())
		e = s.AddDate(0,1,0).Add(-1 * time.Second)
	}
	
	if filename,n,err := generateComplaintsCSV(cdb, s, e); err != nil {
		http.Error(w, fmt.Sprintf("monthly %s->%s: %v", s,e,err), http.StatusInternalServerError)
		return
	} else {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(fmt.Sprintf("OK!\nGCS file %s written, %d rows, took %s", filename, n,
			time.Since(tStart))))
	}
}

// }}}

// {{{ generateComplaintsCSV

// Generates a report and puts in GCS

func generateComplaintsCSV(cdb complaintdb.ComplaintDB, s,e time.Time) (string, int, error) {
	ctx := cdb.Ctx()
	bucketname := "serfr0-reports"
	
	log.Printf("Starting generateComplaintsCSV: %s -> %s", s, e)

	// One time, at 00:00, for each day of the given month
	days := date.IntermediateMidnights(s.Add(-1 * time.Second),e)

	filename := s.Format("complaints-20060102") + e.Format("-20060102.csv")

	gcsName := "gs://"+bucketname+"/"+filename
	
	if exists,err := gcs.Exists(ctx, bucketname, filename); err != nil {
		return gcsName,0,fmt.Errorf("gcs.Exists=%v for gs://%s/%s (err=%v)", exists, bucketname, filename, err)
	} else if exists {
		return gcsName,0,nil
	}

	gcsHandle,err := gcs.OpenRW(ctx, bucketname, filename, "text/plain")
	if err != nil {
		return gcsName,0,err
	}

	tStart := time.Now()
	n := 0
	
	for _,dayStart := range days {
		dayEnd := dayStart.AddDate(0,0,1).Add(-1 * time.Second)
		q := cdb.NewComplaintQuery().ByTimespan(dayStart, dayEnd)
		log.Printf(" /be/month: %s - %s", dayStart, dayEnd)

		if num,err := cdb.WriteCQueryToCSV(q, gcsHandle.IOWriter(), (n==0)); err != nil {
			return gcsName,0,fmt.Errorf("failed; time since start: %s. Err: %v", time.Since(tStart), err)
		} else {
			n += num
		}
	}

	if err := gcsHandle.Close(); err != nil {
		return gcsName,0,err
	}

	log.Printf("monthly CSV successfully written to %s, %d rows", gcsName, n)

	return gcsName,n,nil
}

// }}}
// {{{ formValueMonthDefaultToPrev

// Gruesome. This pseudo-widget looks at 'year' and 'month', or defaults to the previous month.
// Everything is in Pacific Time.
func formValueMonthDefaultToPrev(r *http.Request) (month, year int, err error){
	// Default to the previous month
	oneMonthAgo := date.NowInPdt().AddDate(0,-1,0)
	month = int(oneMonthAgo.Month())
	year  = int(oneMonthAgo.Year())

	// Override with specific values, if present
	if r.FormValue("year") != "" {
		if y,err2 := strconv.ParseInt(r.FormValue("year"), 10, 64); err2 != nil {
			err = fmt.Errorf("need arg 'year' (2015)")
			return
		} else {
			year = int(y)
		}
		if m,err2 := strconv.ParseInt(r.FormValue("month"), 10, 64); err2 != nil {
			err = fmt.Errorf("need arg 'month' (1-12)")
			return
		} else {
			month = int(m)
		}
	}

	return
}

// }}}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
