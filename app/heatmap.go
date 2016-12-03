package complaints

import (
	"encoding/json"
	"net/http"
	
	"github.com/skypies/util/date"

	"github.com/skypies/complaints/complaintdb"
)

func init() {
	http.HandleFunc("/cdb/heatmap", heatmapHandler)
}

func heatmapHandler(w http.ResponseWriter, r *http.Request) {
	ctx := req2ctx(r)
	cdb := complaintdb.NewDB(ctx)
	s,e := date.WindowForToday()
	
	if complaintPoints,err := cdb.GetComplaintPositionsInSpan(s,e); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else if jsonBytes,err := json.Marshal(complaintPoints); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonBytes)
	}
}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
