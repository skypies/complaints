package backend

// {{{ import()

import (
	"fmt"
	"net/http"

	"time"

	oldappengine "appengine"
	olddatastore "appengine/datastore"
	
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/taskqueue"
	//"golang.org/x/net/context"

	"github.com/skypies/geo/sfo"
	"github.com/skypies/util/date"
	"github.com/skypies/util/widget"
	oldfdb "github.com/skypies/flightdb"
	oldfgae "github.com/skypies/flightdb/gae"
)

// }}}

func init() {
	http.HandleFunc("/backend/fdb-batch", batchFlightScanHandler)
	http.HandleFunc("/backend/fdb-batch/flight", batchSingleFlightHandler)
}

// To add a new batch handler, clone a jobFoo routine, and then add it into the switch{}
// in batchSingleFlightHandler. (And consider if you need cleverer flight selection logic
// in batchFlightScanHandler ...)

// {{{ batchFlightScanHandler

// /backend/fdb-batch/start?
// date=range,range_from=2016/01/21&range_to=2016/01/26
// &job=foo

// &unit=flight [or unit=day; defaults to 'flight']

// This enqueues tasks for each individual day, or flight
func batchFlightScanHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	tags := []string{"ADSB"} // Maybe make this configurable ...
	
	n := 0
	str := ""
	s,e,_ := widget.FormValueDateRange(r)
	job := r.FormValue("job")
	if job == "" {
		http.Error(w, "Missing argument: &job=foo", http.StatusInternalServerError)
	}
	
	days := date.IntermediateMidnights(s.Add(-1 * time.Second),e) // decrement start, to include it
	for _,day := range days {
		// Get the keys for all the flights on this day.
		fdb := oldfgae.FlightDB{C: oldappengine.NewContext(r)}

		dStart,dEnd := date.WindowForTime(day)
		dEnd = dEnd.Add(-1 * time.Second)
		keys,err := fdb.KeysInTimeRangeByTags(tags, dStart, dEnd)
		if err != nil {
			log.Errorf(c, "upgradeHandler: enqueue: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		for _,key := range keys {
			str += fmt.Sprintf("Enqueing job=%s, day=%s, flight=%s\n",
				job, day.Format("2006.01.02"), key.Encode())

			t := taskqueue.NewPOSTTask("/backend/fdb-batch/flight", map[string][]string{
				"date": {day.Format("2006.01.02")},
				"key": {key.Encode()},
				"job": {job},
			})

			if _,err := taskqueue.Add(c, t, "batch"); err != nil {
				log.Errorf(c, "upgradeHandler: enqueue: %v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			
			n++
		}
	}

	log.Infof(c, "enqueued %d batch items for '%s'", n, job)

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(fmt.Sprintf("OK, batch, enqueued %d tasks for %s\n%s", n, job, str)))
}

// }}}
// {{{ batchSingleFlightHandler

// A super widget, for all the batch jobs
func formValueFlightByKey(r *http.Request) (*oldfdb.Flight, error) {
	fdb := oldfgae.FlightDB{C: oldappengine.NewContext(r)}
	
	key,err := olddatastore.DecodeKey(r.FormValue("key"))
	if err != nil {
		return nil, fmt.Errorf("fdb-batch: %v", err)
	}
	f,err := fdb.KeyToFlight(key)
	if err != nil {
		return nil, fmt.Errorf("fdb-batch: %v", err)
	}
	return f, nil
}

func batchSingleFlightHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	f,err := formValueFlightByKey(r)
	if err != nil {
		log.Errorf(c, "batch/fdb/track/getflight: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// You now have a job name, and a flight object. Get to it !	
	job := r.FormValue("job")
	var str string = ""
	switch job {
	case "tracktimezone":
		str, err = jobTrackTimezoneHandler(r,f)
	}

	if err != nil {
		log.Errorf(c, "%s", str)
		log.Errorf(c, "backend/fdb-batch/flight: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Debugf(c, "job=%s, on %s: %s", job, f, str)
		
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(fmt.Sprintf("OK\n * %s\n%s\n", f, str)))
}

// }}}

// {{{ jobTracktimefixHandler

// /backend/fdb-batch?job=tracktimezone&date=range&range_to=2016/01/25&range_from=2016/01/25

func jobTrackTimezoneHandler(r *http.Request, f *oldfdb.Flight) (string, error) {
	c := appengine.NewContext(r)

	defaultTP := f.Track.ClosestTrackpoint(sfo.KFixes["EPICK"])
	adsbTP    := f.Tracks["ADSB"].ClosestTrackpoint(sfo.KFixes["EPICK"])
	trackTimeDelta := defaultTP.TimestampUTC.Sub(adsbTP.TimestampUTC)

	str := fmt.Sprintf("OK, looked up %s\n Default: %s\n ADSB   : %s\n delta: %s\n",
		f, defaultTP, adsbTP, trackTimeDelta)

	if trackTimeDelta < -4 * time.Hour || trackTimeDelta > 4* time.Hour {
		str += fmt.Sprintf("* recoding\n* before: %s\n", f.Tracks["ADSB"])

		for i,_ := range f.Tracks["ADSB"] {
			f.Tracks["ADSB"][i].TimestampUTC = f.Tracks["ADSB"][i].TimestampUTC.Add(time.Hour * -8)
		}
		str += fmt.Sprintf("* after : %s\n", f.Tracks["ADSB"])

		db := oldfgae.FlightDB{C: oldappengine.NewContext(r)}
		if err := db.UpdateFlight(*f); err != nil {
			log.Errorf(c, "Persist Flight %s: %v", f, err)
			return str, err
		}
		log.Infof(c, "Updated flight %s", f)
		str += fmt.Sprintf("--\nFlight was updated\n")

	} else {
		log.Debugf(c, "Skipped flight %s, delta=%s", f, trackTimeDelta)
		str += "--\nFlight was OK, left untouched\n"
	}

	return str, nil
}

// }}}


// {{{ slowHandler

// Attempt to test timeouts in deployed appengine; but proved elusive.
func slowHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	d,_ := time.ParseDuration(r.FormValue("d"))

	tStart := time.Now()

	start,end := date.WindowForTime(tStart)
	end = end.Add(-1 * time.Second)
	str := ""
	
	for time.Since(tStart) < d {	
		q := datastore.
			NewQuery(oldfgae.KFlightKind).
			Filter("EnterUTC >= ", start).
			Filter("EnterUTC < ", end).
			KeysOnly()
		keys, err := q.GetAll(c, nil);
		if err != nil {
			log.Errorf(c, "batch/day: GetAll: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		str += fmt.Sprintf("Found %d flight objects at %s\n", len(keys), time.Now())
		time.Sleep(2 * time.Second)
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(fmt.Sprintf("OK, waited for %s !\n%s", r.FormValue("d"), str)))
}

// }}}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
