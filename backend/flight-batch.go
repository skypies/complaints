package backend

// {{{ import()

import (
	"fmt"
	"net/http"

	"sort"
	"strings"
	"time"

	oldappengine "appengine"
	olddatastore "appengine/datastore"
	
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/taskqueue"
	"google.golang.org/appengine/urlfetch"
	"golang.org/x/net/context"

	"github.com/skypies/geo/sfo"
	"github.com/skypies/util/date"
	"github.com/skypies/util/widget"
	oldfdb "github.com/skypies/flightdb"
	oldfgae "github.com/skypies/flightdb/gae"
)

// }}}

func init() {
	//http.HandleFunc("/backend/fdb-batch", batchFlightScanHandler)
	http.HandleFunc("/backend/fdb-batch/range", batchFlightDateRangeHandler)
	http.HandleFunc("/backend/fdb-batch/day", batchFlightDayHandler)
	http.HandleFunc("/backend/fdb-batch/flight", batchSingleFlightHandler)
}

// To add a new batch handler, clone a jobFoo routine, and then add it into the switch{}
// in batchSingleFlightHandler. (And consider if you need cleverer flight selection logic
// in batchFlightScanHandler ...)

// {{{ batchFlightDateRangeHandler

// http://backend-dot-serfr0-1000.appspot.com/backend/fdb-batch/range?date=range&range_from=2016/01/21&range_to=2016/01/26&job=retag

// Enqueues one 'day' task per day in the range
func batchFlightDateRangeHandler(w http.ResponseWriter, r *http.Request) {
	c,_ := context.WithTimeout(appengine.NewContext(r), 10*time.Minute)

	n := 0
	str := ""
	s,e,_ := widget.FormValueDateRange(r)
	job := r.FormValue("job")
	if job == "" {
		http.Error(w, "Missing argument: &job=foo", http.StatusInternalServerError)
		return
	}
	
	str += fmt.Sprintf("** s: %s\n** e: %s\n", s, e)

	days := date.IntermediateMidnights(s.Add(-1 * time.Second),e) // decrement start, to include it
	for _,day := range days {

		dayUrl := "/backend/fdb-batch/day"
		dayStr := day.Format("2006/01/02")
		
		str += fmt.Sprintf(" * adding %s, %s via %s\n", job, dayStr, dayUrl)
		
		if r.FormValue("dryrun") == "" {
			t := taskqueue.NewPOSTTask(dayUrl, map[string][]string{
				"day": {dayStr},
				"job": {job},
			})

			if _,err := taskqueue.Add(c, t, "batch"); err != nil {
				log.Errorf(c, "upgradeHandler: enqueue: %v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		n++
	}

	log.Infof(c, "enqueued %d batch items for '%s'", n, job)

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(fmt.Sprintf("OK, batch, enqueued %d tasks for %s\n%s", n, job, str)))
}

// }}}
// {{{ batchFlightDayHandler

// /backend/fdb-batch/day?day=2016/01/21&job=foo

// Dequeue a single day, and enqueue a job for each flight on that day
func batchFlightDayHandler(w http.ResponseWriter, r *http.Request) {
	c,_ := context.WithTimeout(appengine.NewContext(r), 10*time.Minute)

	tags := []string{}//"ADSB"} // Maybe make this configurable ...
	
	n := 0
	str := ""
	job := r.FormValue("job")
	if job == "" {
		http.Error(w, "Missing argument: &job=foo", http.StatusInternalServerError)
	}

	day := date.ArbitraryDatestring2MidnightPdt(r.FormValue("day"), "2006/01/02")
	
	fdb := oldfgae.FlightDB{C: oldappengine.NewContext(r)}

	dStart,dEnd := date.WindowForTime(day)
	dEnd = dEnd.Add(-1 * time.Second)
	keys,err := fdb.KeysInTimeRangeByTags(tags, dStart, dEnd)
	if err != nil {
		log.Errorf(c, "upgradeHandler: enqueue: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	singleFlightUrl := "/backend/fdb-batch/flight"
	for _,key := range keys {
		str += fmt.Sprintf("Enqueing day=%s: %s?job=%s&key=%s\n",
			day.Format("2006.01.02"), singleFlightUrl, job, key.Encode())

		if r.FormValue("dryrun") == "" {
			t := taskqueue.NewPOSTTask(singleFlightUrl, map[string][]string{
				// "date": {day.Format("2006.01.02")},
				"key": {key.Encode()},
				"job": {job},
			})

			if _,err := taskqueue.Add(c, t, "batch"); err != nil {
				log.Errorf(c, "upgradeHandler: enqueue: %v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		n++
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

// To run a job directly: /backend/fdb-batch/flight?job=...&key=...&
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
	case "tracktimezone": str, err = jobTrackTimezoneHandler(r,f)
	case "oceanictag":    str, err = jobOceanicTagHandler(r,f)
	case "retag":         str, err = jobRetagHandler(r,f)
	case "v2adsb":        str, err = jobV2adsbHandler(r,f)
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

// Old; use the range one instead
// {{{ batchFlightScanHandler

// /backend/fdb-batch?
// date=range,range_from=2016/01/21&range_to=2016/01/26
// &job=foo

// &unit=flight [or unit=day; defaults to 'flight']

// This enqueues tasks for each individual day, or flight
func batchFlightScanHandler(w http.ResponseWriter, r *http.Request) {
	c,_ := context.WithTimeout(appengine.NewContext(r), 10*time.Minute)
	//c := appengine.NewContext(r)

	tags := []string{}//"ADSB"} // Maybe make this configurable ...
	
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

		singleFlightUrl := "/backend/fdb-batch/flight"
		for _,key := range keys {
			str += fmt.Sprintf("Enqueing day=%s: %s?job=%s&key=%s\n",
				day.Format("2006.01.02"), singleFlightUrl, job, key.Encode())

			if r.FormValue("dryrun") == "" {
				t := taskqueue.NewPOSTTask(singleFlightUrl, map[string][]string{
					"key": {key.Encode()},
					"job": {job},
				})

				if _,err := taskqueue.Add(c, t, "batch"); err != nil {
					log.Errorf(c, "upgradeHandler: enqueue: %v", err)
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}

			n++
		}
	}

	log.Infof(c, "enqueued %d batch items for '%s'", n, job)

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(fmt.Sprintf("OK, batch, enqueued %d tasks for %s\n%s", n, job, str)))
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
// {{{ jobOceanicTagHandler

// use non-cloudflare hostname: http://backend-dot-serfr0-1000.appspot.com/
// /backend/fdb-batch?job=oceanictag&date=range&range_to=2016/01/25&range_from=2016/01/25

// THIS IS DEAD AND BROKEN

func jobOceanicTagHandler(r *http.Request, f *oldfdb.Flight) (string, error) {
	c := appengine.NewContext(r)
	str := ""
	
	if f.HasTag("OCEANIC") { return "", nil }
	//if !f.IsOceanicOrign() &&  { return "", nil }

	// It's oceanic, but missing a tag ... update
	//f.Tags[oldfdb.KTagOceanic] = true
	
	db := oldfgae.FlightDB{C: oldappengine.NewContext(r)}
	if err := db.UpdateFlight(*f); err != nil {
		log.Errorf(c, "Persist Flight %s: %v", f, err)
		return str, err
	}
	log.Infof(c, "Updated flight %s", f)
	str += fmt.Sprintf("--\nFlight was updated\n")

	return str, nil
}

// }}}
// {{{ jobV2adsbHandler

// use non-cloudflare hostname: http://backend-dot-serfr0-1000.appspot.com/
// /backend/fdb-batch?job=v2adsb&date=range&range_to=2016/01/25&range_from=2016/01/25

func jobV2adsbHandler(r *http.Request, f *oldfdb.Flight) (string, error) {
	c := appengine.NewContext(r)
	str := ""

	// Allow overwrite, for the (36,0) disastery
	// if f.HasTrack("ADSB") { return "", nil } // Already has one

	err,deb := f.GetV2ADSBTrack(urlfetch.Client(c))
	str += fmt.Sprintf("*getv2ADSB [%v]:-\n", err, deb)
	if err != nil {
		return str, err
	}

	if ! f.HasTrack("ADSB") { return "", nil } // Didn't find one

	f.Analyse() // Retrigger Class-B stuff
	
	db := oldfgae.FlightDB{C: oldappengine.NewContext(r)}
	if err := db.UpdateFlight(*f); err != nil {
		log.Errorf(c, "Persist Flight %s: %v", f, err)
		return str, err
	}
	log.Infof(c, "Updated flight %s", f)
	str += fmt.Sprintf("--\nFlight was updated\n")

	return str, nil
}

// }}}
// {{{ jobRetagHandler

// use non-cloudflare hostname: http://backend-dot-serfr0-1000.appspot.com/
// /backend/fdb-batch?job=retag&date=range&range_to=2016/01/25&range_from=2016/01/25

//http://backend-dot-serfr0-1000.appspot.com/backend/fdb-batch?job=retag&date=range&range_to=2016/03/07&range_from=2016/03/07

func mapKeys(m map[string]bool) []string {
	ret := []string{}
	for k,_ := range m { ret = append(ret,k) }
	return ret
}

func taglistsEqual(t1,t2 []string) bool {
	sort.Strings(t1)
	sort.Strings(t2)
	return strings.Join(t1,",") == strings.Join(t2,",")
}

func jobRetagHandler(r *http.Request, f *oldfdb.Flight) (string, error) {
	c := appengine.NewContext(r)
	str := ""

	oldtags := f.TagList()

	f.Tags = map[string]bool{}

	f.AnalyseFlightPath()
	f.Analyse()

	if taglistsEqual(oldtags, f.TagList()) {
		return fmt.Sprintf("* no change to tags: %v", f.TagList()), nil
	}
	
	db := oldfgae.FlightDB{C: oldappengine.NewContext(r)}
	if err := db.UpdateFlight(*f); err != nil {
		log.Errorf(c, "Persist Flight %s: %v", f, err)
		return str, err
	}
	log.Infof(c, "Updated flight %s", f)
	str += fmt.Sprintf("--\nFlight was updated\n")

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
