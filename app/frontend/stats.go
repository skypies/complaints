package main

import (
	"fmt"
	"net/http"

	"github.com/skypies/complaints/complaintdb"
)

// {{{ statsResetHandler

// Just don't call this. It will be a nightmare to rebuild them.
func statsResetHandler(w http.ResponseWriter, r *http.Request) {
	ctx := req2ctx(r)
	complaintdb.NewDB(ctx).ResetGlobalStats()
	w.Write([]byte(fmt.Sprintf("Stats reset\n")))
}

// }}}
// {{{ statsHandler

func statsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := req2ctx(r)
	cdb := complaintdb.NewDB(ctx)

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
