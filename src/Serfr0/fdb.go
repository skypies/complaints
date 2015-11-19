package serfr0

import (	
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	
	"appengine"
	"appengine/memcache"
	"appengine/taskqueue"
	"appengine/urlfetch"

	"github.com/skypies/date"
	"github.com/skypies/geo"
	"github.com/skypies/geo/sfo"
	ftype "github.com/skypies/flightdb"
	fdb   "github.com/skypies/flightdb/gae"
	fdbfa "abw/flightdbfa"
	fdb24 "abw/flightdbfr24"
)

const (
	kMemcacheFIFOSetKey = "fdb:fifoset"
	kFIFOSetMaxAgeMins = 120
)

func init() {
	// Behind-the-scenes functions to populate the DB
	http.HandleFunc("/fdb/scan", scanHandler)
	http.HandleFunc("/fdb/addflight", addflightHandler)
	http.HandleFunc("/fdb/addtrack", addtrackHandler)
	http.HandleFunc("/fdb/decodetrack", decodetrackHandler)

	// User-facing screens
	http.HandleFunc("/fdb/test", testFdbHandler)
	http.HandleFunc("/fdb/lookup", lookupHandler)
	http.HandleFunc("/fdb/query", queryHandler)

	// Main DB summary screens
	http.HandleFunc("/fdb/recent",    flightListHandler)
	http.HandleFunc("/fdb/today",     flightListHandler)
	http.HandleFunc("/fdb/yesterday", flightListHandler)
}

// {{{ {load|save}FIFOSet

func loadFIFOSet(c appengine.Context, set *ftype.FIFOSet) (error) {
	if _,err := memcache.Gob.Get(c, kMemcacheFIFOSetKey, set); err == memcache.ErrCacheMiss {
    // cache miss, but we don't care
		return nil
	} else if err != nil {
    c.Errorf("error getting item: %v", err)
		return err
	}
	return nil
}

func saveFIFOSet(c appengine.Context, s ftype.FIFOSet) error {
	s.AgeOut(time.Minute * kFIFOSetMaxAgeMins)
	item := memcache.Item{Key:kMemcacheFIFOSetKey, Object:s}
	if err := memcache.Gob.Set(c, &item); err != nil {
		c.Errorf("error setting item: %v", err)
	}
	return nil
}

// }}}
// {{{ scanHandler

// Look for new flights that we should add to our database. Invoked by cron.
func scanHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if db,err1 := fdb24.NewFlightDBFr24(urlfetch.Client(c)); err1 != nil {
		c.Errorf(" /mdb/scan: newdb: %v", err1)
		http.Error(w, err1.Error(), http.StatusInternalServerError)

	} else {
		if flights,err2 := db.LookupList(sfo.KBoxSFO120K); err2 != nil {
			c.Errorf(" /mdb/scan: lookup: %v", err2)
			http.Error(w, err2.Error(), http.StatusInternalServerError)
		} else {

			set := ftype.FIFOSet{}
			if err3 := loadFIFOSet(c,&set); err3 != nil {
				c.Errorf(" /mdb/scan: loadcache: %v", err3)
				http.Error(w, err3.Error(), http.StatusInternalServerError)
			}
			new := set.FindNew(flights)
			if err4 := saveFIFOSet(c,set); err4 != nil {
				c.Errorf(" /mdb/scan: savecache: %v", err4)
				http.Error(w, err4.Error(), http.StatusInternalServerError)
			}

			// Enqueue the new flights
			n := 1000
			for i,fs := range new {
				if i >= n { break }
				if fsStr,err5 := fs.Base64Encode(); err5 != nil {
					http.Error(w, err5.Error(), http.StatusInternalServerError)
					return
				} else {
					t := taskqueue.NewPOSTTask("/fdb/addflight", map[string][]string{
						"flightsnapshot": {fsStr},
					})

					// We could be smarter about this.
					t.Delay = time.Minute * 45

					if _,err6 := taskqueue.Add(c, t, "addflight"); err6 != nil {
						c.Errorf(" /mdb/scan: enqueue: %v", err6)
						http.Error(w, err6.Error(), http.StatusInternalServerError)
						return
					}
				}
			}
			
			var params = map[string]interface{}{
				"New": new,
				"Flights": flights,
			}	
			if err7 := templates.ExecuteTemplate(w, "fdb-scan", params); err7 != nil {
				http.Error(w, err7.Error(), http.StatusInternalServerError)
			}
		}
	}
}

// }}}
// {{{ addflightHandler

func addflightHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	fsStr := r.FormValue("flightsnapshot")
	fs := ftype.FlightSnapshot{}
	if err := fs.Base64Decode(fsStr); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	fr24Id := fs.F.Id.ForeignKeys["fr24"]
	// c.Infof(" /mdb/addflight: %s, %s", fr24Id, fs)

	if false {
		//c.Infof("\n\n  ** %s\n\n", fr24Id)
		c.Infof(" /mdb/addflight: * bailing %s", fs)
		w.Write([]byte(fmt.Sprintf("bailing\n")))
		return
	}
	
	if db,err := fdb24.NewFlightDBFr24(urlfetch.Client(c)); err != nil {
		c.Errorf(" /mdb/addflight: newdb: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		db.Fdb.C = c

		// Be idempotent - check to see if this flight has already been recorded
		if exists,err2 := db.Fdb.FlightExists(fs.F.Id.UniqueIdentifier()); err2 != nil {
			c.Errorf(" /mdb/addflight: FlightExists: %v", err2)
			http.Error(w, err2.Error(), http.StatusInternalServerError)

		} else if exists {
			c.Infof(" /mdb/addflight: already exists %s", fs)
			w.Write([]byte(fmt.Sprintf("Skipped %s\n", fs)))
			return

		} else {	
			// This depends on some apache on a nice IP configured as follows:
			//     ProxyPass  "/fr24/"   "http://mobile.api.fr24.com/"

			if f,err3 := db.LookupPlayback(fr24Id); err3 != nil {
				// c.Errorf(" /mdb/addflight: lookup: %v", err3)
				http.Error(w, err3.Error(), http.StatusInternalServerError)

			} else {

				f.AnalyseFlightPath() // Work out how to tag it

				if f.HasTag(ftype.KTagSERFR1) || f.HasTag(ftype.KTagBRIXX) {
					if err := fdbfa.AddFlightAwareTrack(urlfetch.Client(c),f); err != nil {
						c.Errorf(" /mdb/addflight: addflightaware: %v", err)
					}
				}

				// What the hell, do this on every flight we can
				if err := db.Fdb.MaybeAddTrackFragmentsToFlight(f); err != nil {
					c.Errorf(" /mdb/addflight: addTrackFrags(%s): %v", f.Id, err)
				}
				
				f.Analyse()

				if err4 := db.Fdb.PersistFlight(*f); err4 != nil {
					c.Errorf(" /mdb/addflight: persist: %v", err4)
					http.Error(w, err4.Error(), http.StatusInternalServerError)

				} else {
					// Success !
					// c.Infof(" /mdb/addflight: persisted %s, %s", fr24Id, f)
					w.Write([]byte(fmt.Sprintf("Added %s\n", f)))
				}
			}
		}
	}
}

// }}}
// {{{ addtrackHandler

func addtrackHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	db := fdb.FlightDB{C: c}

	icaoId := r.FormValue("icaoid")
	callsign := strings.TrimSpace(r.FormValue("callsign"))
	tStr := r.FormValue("track")

	// Validate it works before persisting
	t := ftype.Track{}
	if err := t.Base64Decode(tStr); err != nil {
		c.Errorf(" /mdb/addtrack: decode failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	ftf := ftype.FrozenTrackFragment{
		TrackBase64: tStr,
		Callsign: callsign,
		Icao24: icaoId,
	}
	if err := db.AddTrackFrgament(ftf); err != nil {
		c.Errorf(" /mdb/addtrack: db.AddTrackFragment failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	
	// 3. Routine to merge track fragments ? Extra credit ?
	
	// c.Infof(" /mdb/addtrack: added %d points for [%s][%s]", len(t), icaoId, callsign)
	w.Write([]byte(fmt.Sprintf("Added %d for %s\n", len(t), icaoId)))
}

// }}}

// {{{ flights2params

func flights2params(flights []ftype.Flight) []map[string]interface{} {
	var flightsParams = []map[string]interface{}{}
	for _,f := range flights {
		flightsParams = append(flightsParams, map[string]interface{}{
			"Url": fmt.Sprintf("/fdb/lookup?map=1&id=%s", f.Id.UniqueIdentifier()),
			"Oneline": f.String(),
			"ExtraNotes": "aasdasd",
			"F": f,
			"Text": fmt.Sprintf("%#v", f.Id),
		})
	}
	return flightsParams
}

// }}}
// {{{ snapshots2params

func snapshots2params(snapshots []ftype.FlightSnapshot) []map[string]interface{} {
	var flightsParams = []map[string]interface{}{}
	for _,s := range snapshots {
		flightsParams = append(flightsParams, map[string]interface{}{
			"Url": fmt.Sprintf("/fdb/lookup?map=1&id=%s", s.F.Id.UniqueIdentifier()),
			"Oneline": s.F.String(),
			"F": s.F,
			"Pos": s.Pos,
			"Text": fmt.Sprintf("%#v", s.F.Id),
		})
	}
	return flightsParams
}

// }}}
// {{{ buildLegend

func legendUrl(t time.Time, offset int64, val string) string {
	epoch := t.Unix() + offset
	return fmt.Sprintf("<a href=\"/fdb/query?epoch=%d\">%s</a>", epoch, val)
}

func buildLegend(t time.Time) string {
	legend := date.InPdt(t).Format("15:04:05 MST (2006/01/02)")

	legend += " ["+
		legendUrl(t,-3600,"-1h")+", "+
		legendUrl(t,-1200,"-20m")+", "+
		legendUrl(t, -600,"-10m")+", "+
		legendUrl(t, -300,"-5m")+", "+
		legendUrl(t,  -60,"-1m")+", "+
		legendUrl(t,  -30,"-30s")+"; "+
		legendUrl(t,   30,"+30s")+"; "+
		legendUrl(t,   60,"+1m")+", "+
		legendUrl(t,  300,"+5m")+", "+
		legendUrl(t,  600,"+10m")+", "+
		legendUrl(t, 1200,"+20m")+", "+
		legendUrl(t, 3600,"+1h")+
		"]"
	return legend
}

// }}}

// {{{ flightListHandler

// We examine the tags CGI arg, which should be a pipe-delimited set of flight tags.
func flightListHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	db := fdb.FlightDB{C: c}
	
	tags := []string{}
	if r.FormValue("tags") != "" {
		tags = append(tags, strings.Split(r.FormValue("tags"), "|")...)
	}
	
	timeRange :=regexp.MustCompile("/fdb/(.+)$").ReplaceAllString(r.URL.Path, "$1")
	var flights []ftype.Flight
	var err error
	
	switch timeRange {
	case "recent":
		flights,err = db.LookupRecentByTags(tags,200)
	case "today":
		s,e := date.WindowForToday()
		flights,err = db.LookupTimeRangeByTags(tags,s,e)
	case "yesterday":
		s,e := date.WindowForYesterday()
		flights,err = db.LookupTimeRangeByTags(tags,s,e)
	}

	if err != nil {
		c.Errorf(" %s: %v", r.URL.Path, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var params = map[string]interface{}{
		"Tags": tags,
		"TimeRange": timeRange,
		"Flights": flights2params(flights),
	}	
	if err := templates.ExecuteTemplate(w, "fdb-recentlist", params); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// }}}
// {{{ lookupHandler

func lookupHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	db := fdb.FlightDB{C: c}
	id := r.FormValue("id")

	if f,err2 := db.LookupById(id); err2 != nil {
		c.Errorf(" /mdb/lookup: %v", err2)
		http.Error(w, err2.Error(), http.StatusInternalServerError)

	} else if f == nil {
		http.Error(w, fmt.Sprintf("id=%s not found", id), http.StatusInternalServerError)
		
	} else {
		c.Infof("Tags: %v, Tracks: %v", f.TagList(), f.TrackList())
		_,classBTrack := f.SFOClassB("")
			
		f.Analyse()  // Populate the flight tags

		fr24TrackJSVar := classBTrack.ToJSVar()

		// For Flightaware tracks
		//faClassBTrack := fdb.ClassBTrack{}
		faTrackJSVar   := template.JS("{}")
		if _,exists := f.Tracks["FA"]; exists==true {
			_,faClassBTrack := f.SFOClassB("FA")
			faTrackJSVar = faClassBTrack.ToJSVar()
			fr24TrackJSVar = template.JS("{}")
		}

		// For ADS-B tracks !
		adsbTrackJSVar   := template.JS("{}")
		if _,exists := f.Tracks["ADSB"]; exists==true {
			_,adsbClassBTrack := f.SFOClassB("ADSB")
			adsbTrackJSVar = adsbClassBTrack.ToJSVar()
		}

		skimTrackJSVar := template.JS("{}")
		if r.FormValue("skim") != "" {
			alttol,_  := strconv.ParseFloat(r.FormValue("alttol"), 64)
			mindist,_ := strconv.ParseFloat(r.FormValue("mindist"), 64)
			skimTrack,_ := f.BestTrack().SkimsToSFO(alttol, mindist, 15.0, 40.0)
			skimTrackJSVar = skimTrack.ToJSVar()
			fr24TrackJSVar = template.JS("{}")
			faTrackJSVar = template.JS("{}")
			adsbTrackJSVar = template.JS("{}")
		}

		c.Infof("\n Ah ha 1: %s", kGoogleMapsAPIKey)
		
		var params = map[string]interface{}{
			"F": f,
			"Legend": f.Legend(),
			"Oneline": f.String(),
			"Text": fmt.Sprintf("%#v", f.Id),
			"ClassB": "",

			"MapsAPIKey": kGoogleMapsAPIKey,
			"Center": sfo.KLatlongSERFR1,
			"Zoom": 10,
			"CaptureArea": sfo.KBoxSFO120K,

			"MapsTrack": fr24TrackJSVar,
			"FlightawareTrack": faTrackJSVar,
			"ADSBTrack": adsbTrackJSVar,
			"SkimTrack": skimTrackJSVar,
		}
		
		templateName := "fdb-lookup"
		if r.FormValue("map") != "" { templateName = "fdb-lookup-map" }
		if err7 := templates.ExecuteTemplate(w, templateName, params); err7 != nil {
			http.Error(w, err7.Error(), http.StatusInternalServerError)
		}
	}
}

// }}}
// {{{ queryHandler

func queryHandler(w http.ResponseWriter, r *http.Request) {
	if r.FormValue("date") == "" && r.FormValue("epoch") == "" {
		var params = map[string]interface{}{
			"TwoHoursAgo": date.NowInPdt().Add(-2 * time.Hour),
		}
		if err := templates.ExecuteTemplate(w, "fdb-queryform", params); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	c := appengine.NewContext(r)
	db := fdb.FlightDB{C: c, Memcache:true}

	var t time.Time
	if r.FormValue("epoch") != "" {
		if epoch,err := strconv.ParseInt(r.FormValue("epoch"), 10, 64); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		} else {
			t = time.Unix(epoch,0)
		}		
	} else {
		var err2 error
		t,err2 = date.ParseInPdt("2006/01/02 15:04:05", r.FormValue("date")+" "+r.FormValue("time"))
		if err2 != nil {
			http.Error(w, err2.Error(), http.StatusInternalServerError)
			return
		}
	}

	var refPoint *geo.Latlong = nil
	if r.FormValue("lat") != "" {
		refPoint = &geo.Latlong{}
		var err error
		if refPoint.Lat,err = strconv.ParseFloat(r.FormValue("lat"), 64); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if refPoint.Long,err = strconv.ParseFloat(r.FormValue("long"), 64); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	
	if snapshots, err := db.LookupSnapshotsAtTimestampUTC(t.UTC(), refPoint, 1000); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		var params = map[string]interface{}{
			"Legend": buildLegend(t),
			"SearchTimeUTC": t.UTC(),
			"SearchTime": date.InPdt(t),
			"Flights": snapshots2params(snapshots),

			"FlightsJS": ftype.FlightSnapshotSet(snapshots).ToJSVar(),
			"MapsAPIKey": kGoogleMapsAPIKey,
			"Center": sfo.KLatlongSERFR1,
			"Zoom": 9,
			// "CaptureArea": fdb.KBoxSFO120K,  // comment out, as we don't want it in this view
		}

		if r.FormValue("resultformat") == "json" {
			for i,_ := range snapshots {
				snapshots[i].F.Track = nil
				snapshots[i].F.Tracks = nil
			}
			js, err := json.Marshal(snapshots)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(js)

		} else {
			templateName := "fdb-queryresults-map"
			if r.FormValue("resultformat") == "list" { templateName = "fdb-queryresults-list" }
		
			if err := templates.ExecuteTemplate(w, templateName, params); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}
	}
}

// }}}
// {{{ decodetrackHandler

func decodetrackHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	db := fdb.FlightDB{C: c}

	icao := r.FormValue("icaoid")
	callsign := strings.TrimSpace(r.FormValue("callsign"))
	if icao == "" || callsign == "" {
		http.Error(w, "need args {icaoid,callsign}", http.StatusInternalServerError)
		return
	}

	if tracks,err := db.ReadTrackFragments(icao, callsign); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else {
		var params = map[string]interface{}{
			"Tracks": tracks,
			"Callsign": callsign,
			"Icao24": icao,
		}
		if err := templates.ExecuteTemplate(w, "fdb-decodetrack", params); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

// }}}

// {{{ testFdbHandler

// Look for new flights that we should add to our database. Invoked by cron.
func testFdbHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	client := urlfetch.Client(c)
	
	if db,err1 := fdb24.NewFlightDBFr24(client); err1 != nil {
		c.Errorf(" /mdb/scan: newdb: %v", err1)
		http.Error(w, err1.Error(), http.StatusInternalServerError)

	} else {
		db.Fdb.C = c  // Sigh

		if flights,err2 := db.LookupList(sfo.KBoxSFO120K); err2 != nil {
			c.Errorf(" /mdb/scan: lookup: %v", err2)
			http.Error(w, err2.Error(), http.StatusInternalServerError)
		} else {

			set := ftype.FIFOSet{}
			if err3 := loadFIFOSet(c,&set); err3 != nil {
				c.Errorf(" /mdb/scan: loadcache: %v", err3)
				http.Error(w, err3.Error(), http.StatusInternalServerError)
				return
			}
			new := set.FindNew(flights)

			if len(new) == 0 {
				http.Error(w, "new was empty", http.StatusInternalServerError)
				return
			}

			fr24Id := new[0].F.Id.ForeignKeys["fr24"]
			for i,v := range new {
				if r.FormValue("n") != "" {
					c.Infof("Oh ho [%s][%s]: %s\n",
						r.FormValue("n"), fmt.Sprintf("%d",v.F.Id.Designator.FlightNumber), new[i])
					if fmt.Sprintf("%d",v.F.Id.Designator.FlightNumber) == r.FormValue("n") {
						fr24Id = v.F.Id.ForeignKeys["fr24"]
						break
					}
				} else if v.F.Id.Destination == "SFO" {
					fr24Id = v.F.Id.ForeignKeys["fr24"]
					break
				}
			}
			if f,err4 := db.LookupPlayback(fr24Id); err4 != nil {
				c.Errorf(" /mdb/test: lookup: %v", err4)
				http.Error(w, err4.Error(), http.StatusInternalServerError)

			} else {

				f.AnalyseFlightPath() // Work out how to tag it

				if f.Tags[ftype.KTagSERFR1] { fdbfa.AddFlightAwareTrack(urlfetch.Client(c),f) }

				f.Analyse() // Other tags

				// HACKETY HACK
				if false {
				if frags,err := db.Fdb.ExtractTrackFragments("A018EE","callsign"); err != nil {
					c.Errorf(" /mdb/test: fragsy: %v", err)
					http.Error(w, err.Error(), http.StatusInternalServerError)
				} else {
					if len(frags) > 1 {
						c.Errorf(" /mdb/test: multiple frags (%d); discarding all but last", len(frags))
					}
					if len(frags)>0 {
						frag := frags[len(frags)-1]
						f.Tracks["ADSB"] = frag
						c.Infof("/mdb/test, got a frag: %s", frag)
					} else {
						c.Infof("/mdb/test, no frags")
					}
				}
				}
				
				if r.FormValue("persist") != "" {
					if err5 := db.Fdb.PersistFlight(*f); err5 != nil {
						c.Errorf(" /mdb/test: persist: %v", err5)
						http.Error(w, err5.Error(), http.StatusInternalServerError)
					}
				}

				var params = map[string]interface{}{
					"New": new,
					"F": f,
					"ClassB": "",
					"Text": fmt.Sprintf("%#v", f.Id),
					"Oneline": f.String(),
					"Flights": flights,
				}	
				if err7 := templates.ExecuteTemplate(w, "fdb-test", params); err7 != nil {
					http.Error(w, err7.Error(), http.StatusInternalServerError)
				}
			}
		}
	}
}

// }}}

/* To populate the dev server dlight DB: comment out the 45m timeout, then call
 *  localhost:8080/fdb/scan */


// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
