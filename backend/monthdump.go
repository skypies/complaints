package backend

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"time"

	oldappengine "appengine"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"golang.org/x/net/context"

	"github.com/skypies/util/date"
	"github.com/skypies/util/gcs"

	"github.com/skypies/complaints/complaintdb"
)

func init() {
	http.HandleFunc("/backend/monthdump", monthCSVTaskHandler)
}

// {{{ monthCSVTaskHandler

// Dumps the monthly CSV file into Google Cloud Storage, ready to be emailed to BKSV et al.
// Defaults to the previous month; else can specify an explicit year & month.

// http://backend-dot-serfr0-1000.appspot.com/backend/monthdump
// http://backend-dot-serfr0-1000.appspot.com/backend/monthdump?year=2016&month=4

func monthCSVTaskHandler(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)

	cdb := complaintdb.ComplaintDB{
		C:oldappengine.Timeout(oldappengine.NewContext(r), 599*time.Second),
		Req:r,
	}
	
	month,year,err := FormValueMonthDefaultToPrev(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}	
	
	tStart := time.Now()
	if filename,n,err := generateMonthlyCSV(ctx, cdb, month,year); err != nil {
		http.Error(w, fmt.Sprintf("monthly %d.%d: %v", year,month,err), http.StatusInternalServerError)
		return
	} else {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(fmt.Sprintf("OK!\nGCS file %s written, %d rows, took %s", filename, n,
			time.Since(tStart))))
	}
}

// }}}

// {{{ generateMonthlyCSV

// Dumps the monthly CSV file into Google Cloud Storage, ready to be emailed to BKSV et al.

// http://backend-dot-serfr0-1000.appspot.com/backend/monthdump?year=2016&month=4

func generateMonthlyCSV(ctx context.Context, cdb complaintdb.ComplaintDB, month,year int) (string, int, error) {
	bucketname := "serfr0-reports"
	
	now := date.NowInPdt()
	s := time.Date(int(year), time.Month(month), 1, 0,0,0,0, now.Location())
	e := s.AddDate(0,1,0).Add(-1 * time.Second)
	log.Infof(ctx, "Starting /be/month: %s", s)

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

	tStart := time.Now()
	n := 0
	for _,dayStart := range days {
		dayEnd := dayStart.AddDate(0,0,1).Add(-1 * time.Second)
		log.Infof(ctx, " /be/month: %s - %s", dayStart, dayEnd)

		tIter := time.Now()
		iter := cdb.NewLongBatchingIter(cdb.QueryInSpan(dayStart, dayEnd))
		for {
			c,err := iter.NextWithErr();
			if err != nil {
				return gcsName,0,fmt.Errorf("iterator failed after %s (%s): %v", err, time.Since(tIter),
					time.Since(tStart))
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
				return gcsName,0,err
			}

			n++
		}
	}
	csvWriter.Flush()

	if err := gcsHandle.Close(); err != nil {
		return gcsName,0,err
	}

	log.Infof(ctx, "monthly CSV successfully written to %s, %d rows", gcsName, n)

	return gcsName,n,nil
}

// }}}

/*
// {{{ monthTaskHandler

// Dumps the monthly CSV file into Google Cloud Storage, ready to be emailed to BKSV et al.

// http://backend-dot-serfr0-1000.appspot.com/backend/monthdump?year=2016&month=4

func monthTaskHandler(w http.ResponseWriter, r *http.Request) {
	//ctx,_ := context.WithTimeout(appengine.NewContext(r), 599*time.Second)
	ctx := appengine.NewContext(r)

	cdb := complaintdb.ComplaintDB{
		//C: oldappengine.NewContext(r),
		C:oldappengine.Timeout(oldappengine.NewContext(r), 599*time.Second),
		Req:r,
	}
	bucketname := "serfr0-reports"

	year,month := 0,0
	
	if r.Formvalue(
	
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

	if exists,err := gcs.Exists(ctx, bucketname, filename); err != nil {
		http.Error(w, fmt.Sprintf("gcs.Exists gs://%s/%s : %v", bucketname, filename, err),
			http.StatusInternalServerError)
		return
	} else if exists {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(fmt.Sprintf("gs://%s/%s already exists, skipping", bucketname, filename)))
		return
	}
	
	gcsHandle,err := gcs.OpenRW(ctx, bucketname, filename, "text/plain")
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

	tStart := time.Now()
	for _,dayStart := range days {
		dayEnd := dayStart.AddDate(0,0,1).Add(-1 * time.Second)
		log.Infof(ctx, " /be/month: %s - %s", dayStart, dayEnd)

		tIter := time.Now()
		iter := cdb.NewLongBatchingIter(cdb.QueryInSpan(dayStart, dayEnd))
		for {
			c,err := iter.NextWithErr();
			if err != nil {
				errStr := fmt.Sprintf("iterator failed after %s (%s): %v", err, time.Since(tIter),
					time.Since(tStart))
				http.Error(w, errStr, http.StatusInternalServerError)
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
*/

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
