package complaints

import (
	"fmt"
	"net/http"

	"github.com/skypies/complaints/complaintdb"
)

func init() {
	http.HandleFunc("/stats", statsHandler)
	http.HandleFunc("/stats-reset", statsResetHandler)
}

// {{{ statsResetHandler

func statsResetHandler(w http.ResponseWriter, r *http.Request) {
	complaintdb.NewDB(r).ResetGlobalStats()
	w.Write([]byte(fmt.Sprintf("Stats reset\n")))
}

// }}}
// {{{ statsHandler

func statsHandler(w http.ResponseWriter, r *http.Request) {
	cdb := complaintdb.NewDB(r)

	if gs,err := cdb.LoadGlobalStats(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
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
