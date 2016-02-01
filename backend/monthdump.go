package backend

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	oldappengine "appengine"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	//"golang.org/x/net/context"

	"github.com/skypies/util/date"
	"github.com/skypies/util/gcs"

	"github.com/skypies/complaints/complaintdb"
)

func init() {
	http.HandleFunc("/backend/monthdump", monthTaskHandler)
}

// {{{ monthTaskHandler

// http://stop.jetnoise.net/backend/monthdump?year=2015&month=9

func monthTaskHandler(w http.ResponseWriter, r *http.Request) {
	//ctx,_ := context.WithTimeout(appengine.NewContext(r), 599*time.Second)
	ctx := appengine.NewContext(r)

	cdb := complaintdb.ComplaintDB{
		//C: oldappengine.NewContext(r),
		C:oldappengine.Timeout(oldappengine.NewContext(r), 599*time.Second),
	}
	
	year,err := strconv.ParseInt(r.FormValue("year"), 10, 64)
	if err != nil {
		http.Error(w, "need arg 'year' (2015)", http.StatusInternalServerError)
		return
	}
	month,err := strconv.ParseInt(r.FormValue("month"), 10, 64)
	if err != nil {
		http.Error(w, "need arg 'month' (1-12)", http.StatusInternalServerError)
		return
	}

	now := date.NowInPdt()
	s := time.Date(int(year), time.Month(month), 1, 0,0,0,0, now.Location())
	e := s.AddDate(0,1,0).Add(-1 * time.Second)
	log.Infof(ctx, "Starting /be/month: %s", s)

	// One time, at 00:00, for each day of the given month
	days := date.IntermediateMidnights(s.Add(-1 * time.Second),e)

	filename := s.Format("complaints-20060102") + e.Format("-20060102.csv")

	gcsHandle,err := gcs.OpenRW(ctx, "serfr0-reports", filename, "text/plain")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	csvWriter := csv.NewWriter(gcsHandle.IOWriter())

	cols := []string{
		"CallerCode", "Name", "Address", "Zip", "Email", "HomeLat", "HomeLong", 
		"UnixEpoch", "Date", "Time(PDT)", "Notes", "ActivityDisturbed", "Flightnumber",
		"Notes",
		// Column names above are incorrect, but BKSV are used to them.
		//
		//"CallerCode", "Name", "Address", "Zip", "Email", "HomeLat", "HomeLong", 
		//"UnixEpoch", "Date", "Time(PDT)", "Notes", "Flightnumber",
		//"ActivityDisturbed", "CcSFO",
	}
	csvWriter.Write(cols)

	for _,dayStart := range days {
		dayEnd := dayStart.AddDate(0,0,1).Add(-1 * time.Second)
		log.Infof(ctx, " /be/month: %s - %s", dayStart, dayEnd)

		iter := cdb.NewIter(cdb.QueryInSpan(dayStart, dayEnd))
		for {
			c,err := iter.NextWithErr();
			if err != nil {
				http.Error(w, fmt.Sprintf("iterator failed: %v", err),
					http.StatusInternalServerError)
				return
			}
			if c == nil { break }
		
			r := []string{
				c.Profile.CallerCode,
				c.Profile.FullName,
				c.Profile.Address,
				c.Profile.StructuredAddress.Zip,
				c.Profile.EmailAddress,
				fmt.Sprintf("%.4f",c.Profile.Lat),
				fmt.Sprintf("%.4f",c.Profile.Long),

				fmt.Sprintf("%d", c.Timestamp.UTC().Unix()),
				c.Timestamp.Format("2006/01/02"),
				c.Timestamp.Format("15:04:05"),
				c.Description,
				c.AircraftOverhead.FlightNumber,
				c.Activity,
				fmt.Sprintf("%v",c.Profile.CcSfo),
			}

			if err := csvWriter.Write(r); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}
	csvWriter.Flush()

	if err := gcsHandle.Close(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Infof(ctx, "GCS report '%s' successfully written", filename)

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(fmt.Sprintf("OK!\nGCS file '%s' written to bucket", filename)))
}

// }}}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
