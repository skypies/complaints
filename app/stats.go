package complaints

import (
	"fmt"
	"net/http"
	"regexp"
	"appengine"

	"github.com/skypies/complaints/complaintdb"
)

func init() {
	http.HandleFunc("/speedbrakes", speedbrakeHandler)
	http.HandleFunc("/stats", statsHandler)
	http.HandleFunc("/stats-reset", statsResetHandler)
}

// {{{ statsResetHandler

func statsResetHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	cdb := complaintdb.ComplaintDB{C: c}

	cdb.ResetGlobalStats()
	
	w.Write([]byte(fmt.Sprintf("Stats reset\n")))
}

// }}}
// {{{ statsHandler

func statsHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	cdb := complaintdb.ComplaintDB{C: c}

	if gs,err := cdb.LoadGlobalStats(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {

		// Sigh. Ignore.
		// sort.Sort(sort.Reverse(complaintdb.DailyCountDesc(gs.Counts)))
		
		var params = map[string]interface{}{
			"GlobalStats": gs,
		}
		if err := templates.ExecuteTemplate(w, "stats", params); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
	
}

// }}}
// {{{ speedbrakeHandler

func speedbrakeHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	cdb := complaintdb.ComplaintDB{C: c}

	if complaints,err := cdb.GetComplaintsWithSpeedbrakes(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

	} else {
		data := [][]string{}
		counts := map[string]int{}
		
		for _,c := range complaints {
			airline := regexp.MustCompile("^(..)(\\d+)$").ReplaceAllString(c.AircraftOverhead.FlightNumber, "$1")
			counts[airline] += 1
		}

		for k,v := range counts {
			data = append(data, []string{k, fmt.Sprintf("%d",v)})
			c.Infof("Aha: {%s}, {%s}", k, fmt.Sprintf("%d",v))
		}
		
		var params = map[string]interface{}{
			"Data": data,
		}
		if err := templates.ExecuteTemplate(w, "report", params); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

// }}}


// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
