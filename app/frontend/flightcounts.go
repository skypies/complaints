package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/skypies/complaints/pkg/complaintdb"
	"github.com/skypies/util/date"
	"github.com/skypies/util/widget"
)

// {{{ complaintsForFlightHandler

// ?flight=UA123        (IATA flight number)
// &start=123&end=123   (epochs; defaults to current PDT day)
// &debug=1             (render as text/plain)

type timeAsc []time.Time
func (a timeAsc) Len() int           { return len(a) }
func (a timeAsc) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a timeAsc) Less(i, j int) bool { return a[i].Before(a[j]) }

func complaintsForFlightHandler(w http.ResponseWriter, r *http.Request) {
	ctx := req2ctx(r)

	s,e := widget.FormValueEpochTime(r,"start"),widget.FormValueEpochTime(r,"end")
	if r.FormValue("start") == "" {
		s,e = date.WindowForToday()
	}
	flightnumber := r.FormValue("flight")
	if e.Sub(s) > (time.Hour*24) {
		http.Error(w, "time span too wide", http.StatusInternalServerError)
		return
	} else if s.Year() < 2015 {
		http.Error(w, "times in the past", http.StatusInternalServerError)
		return
	} else if flightnumber == "" {
		http.Error(w, "no flightnumber", http.StatusInternalServerError)
		return
	}

	times := []time.Time{}
	cdb := complaintdb.NewDB(ctx)
	q := cdb.NewComplaintQuery().ByTimespan(s,e).ByFlight(flightnumber).Project("Timestamp")
	if complaints,err := cdb.LookupAll(q); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else {
		for _,c := range complaints {
			times = append(times, c.Timestamp)
		}
	}
	sort.Sort(timeAsc(times))

	if r.FormValue("debug") != "" {
		str := fmt.Sprintf("* %s\n* %s\n* %s\n* [%d]\n\n", s, e, flightnumber, len(times))
		for i,t := range times {
			str += fmt.Sprintf("%3d  %s\n", i, date.InPdt(t))
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(str))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	jsonBytes,err := json.Marshal(times)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(jsonBytes)
}

// }}}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
