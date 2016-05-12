package complaints

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	
	"appengine"
	
	"github.com/skypies/complaints/complaintdb"
	"github.com/skypies/util/date"
	"github.com/skypies/util/widget"
)

func init() {
	http.HandleFunc("/complaints-for", complaintsForHandler)
}

// {{{ complaintsForHandler

// ?flight=UA123        (IATA flight number)
// &start=123&end=123   (epochs; defaults to current PDT day)
// &debug=1             (render as text/plain)

func complaintsForHandler(w http.ResponseWriter, r *http.Request) {

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

	ctx := appengine.Timeout(appengine.NewContext(r), 60*time.Second)
	cdb := complaintdb.ComplaintDB{C: ctx}

	times, err := cdb.GetComplaintTimesInSpanByFlight(s,e,flightnumber)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

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
