package complaints

import (
	"encoding/json"
	"net/http"
	"time"
	
	"github.com/skypies/util/widget"
	//"github.com/skypies/util/date"

	"github.com/skypies/complaints/complaintdb"
)

func init() {
	http.HandleFunc("/heatmap", heatmapHandler)
}

func heatmapHandler(w http.ResponseWriter, r *http.Request) {
	ctx := req2ctx(r)
	cdb := complaintdb.NewDB(ctx)

	dur := widget.FormValueDuration(r, "d")
	if dur == 0 { dur = 15 * time.Minute }
	if dur > 24 * time.Hour { dur = 24 * time.Hour }
	
	e := time.Now()	
	s := e.Add(-1 * dur)

	//s,e := date.WindowForToday()

	// Temporary hack, to let goapp serve'd things call the deployed version of this URL
	//w.Header().Set("Access-Control-Allow-Origin", "http://localhost:8080")

	w.Header().Set("Access-Control-Allow-Origin", "*")
	// w.Header().Set("Access-Control-Allow-Origin", "fdb.serfr1.org")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers",
		"Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token")
	w.Header().Set("Access-Control-Allow-Credentials", "true")

	if positions,err := cdb.GetComplaintPositionsInSpan(s,e); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else if jsonBytes,err := json.Marshal(positions); err != nil {
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
