package complaints

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/skypies/geo"
	"github.com/skypies/util/gaeutil"
	"github.com/skypies/util/widget"

	"github.com/skypies/complaints/complaintdb"
)

func init() {
	http.HandleFunc("/heatmap", heatmapHandler)
	gob.Register(geo.LatlongSlice{})
}

// TODO: support explicit s,e values; use a longer lived TTL (and different key) when they're used.
func heatmapHandler(w http.ResponseWriter, r *http.Request) {
	ctx := req2ctx(r)
	cdb := complaintdb.NewDB(ctx)

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers",
		"Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	
	dur := widget.FormValueDuration(r, "d")
	if dur == 0 { dur = 15 * time.Minute }
	if dur > 24 * time.Hour { dur = 24 * time.Hour }
	
	e := time.Now()	
	s := e.Add(-1 * dur)

	icaoid := r.FormValue("icaoid") // Might be empty string (match everything)

	key := fmt.Sprintf("heatmap-%s:%s", dur, icaoid)
	positions := geo.LatlongSlice{} // Use explicit type as registered with encoding/gob

	if err := gaeutil.LoadFromMemcacheShardsTTL(ctx, key, &positions); err == nil {
		// Fresh data from cache; we will use it !
	} else if positions,err = cdb.GetComplaintPositionsInSpanByIcao(s,e,icaoid); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else {
    // Datastore reutrned data; save to memcache, ignoring errors
		ttl := time.Second * 2
		gaeutil.SaveToMemcacheShardsTTL(ctx, key, &positions, ttl)
	}

	if jsonBytes,err := json.Marshal(positions); err != nil {
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
