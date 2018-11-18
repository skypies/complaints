package main

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/skypies/geo"
	"github.com/skypies/util/ae"
	"github.com/skypies/util/widget"

	//"github.com/skypies/util/singleton/memcache"
	//"github.com/skypies/util/singleton/ttl"
	//"github.com/skypies/complaints/config"
	"github.com/skypies/complaints/complaintdb"
)

var (
	ttl := time.Second * 2
)

func init() {
	http.HandleFunc("/heatmap", heatmapHandler)
	gob.Register(geo.LatlongSlice{})
}

// /heatmap?
//   d=2h            - duration to get complaints over (end point is 'now')
//  [icaoid=A04565] - limit to complaints for that one IcaoID
//  [uniques=1]     - unique users, not complaints (i.e. one complaint per user within duration)
//  [allusers=1]    - just dump all users via profile lookup (overrides 'd','icaoid' and 'uniques')

// TODO: support explicit s,e values; use a longer lived TTL (and different key) when they're used.
func heatmapHandler(w http.ResponseWriter, r *http.Request) {
	ctx := req2ctx(r)
	cdb := complaintdb.NewDB(ctx)

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers",
		"Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token")
	w.Header().Set("Access-Control-Allow-Credentials", "true")

	icaoid := r.FormValue("icaoid") // Might be empty string (match everything)
	uniqueUsers := widget.FormValueCheckbox(r,"uniques")

	dur := widget.FormValueDuration(r, "d")
	if dur == 0 { dur = 15 * time.Minute }
	if !uniqueUsers && dur > 24 * time.Hour { dur = 24 * time.Hour }

	e := time.Now()	
	s := e.Add(-1 * dur)
	
	key := fmt.Sprintf("heatmap-uniques=%v-%s:%s", uniqueUsers, dur, icaoid)
	positions := geo.LatlongSlice{} // Use explicit type as registered with encoding/gob

	// Now try a range of ways to fill up positions[] ...
	//memcached := config.Get("memcached.server")
	//sp := ttl.NewProvider(ttl, memcache.NewProvider(memcached))
	//sp.SingletonProvider.NumShards = 32
	//if err := sp.ReadSingleton(ctx, key, nil, &positions); err != nil {
	if err := ae.LoadFromMemcacheShardsTTL(ctx, key, &positions); err == nil {
		// Fresh data from cache; we will use it !

	} else if r.FormValue("allusers") != "" {
		// Simply dump all user locations
		if positions,err = cdb.GetProfileLocations(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

	} else if positions,err=cdb.GetComplaintPositionsInSpanByIcao(s,e,uniqueUsers,icaoid); err!=nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return

	} else {
    // Datastore reutrned data; save to memcache, ignoring errors
		if uniqueUsers { ttl = time.Hour * 24 }

		// sp.WriteMemcache(ctx, key, nil, &positions)
		ae.SaveToMemcacheShardsTTL(ctx, key, &positions, ttl)
	}

	if jsonBytes,err := json.Marshal(positions); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonBytes)
	}
}

/*
func lookupComplaintPositions(s,e time.Time, uniqueUsers bool, icaoid string) ([]geo.Latlong, error) {
}
*/

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
