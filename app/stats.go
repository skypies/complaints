package complaints

import (
	"fmt"
	"net/http"
	"appengine"

	"github.com/skypies/complaints/complaintdb"
)

func init() {
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

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
