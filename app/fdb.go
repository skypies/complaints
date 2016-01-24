package complaints

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

	"github.com/skypies/geo"
	"github.com/skypies/geo/sfo"
	"github.com/skypies/util/date"
	ftype "github.com/skypies/flightdb"
	fdb   "github.com/skypies/flightdb/gae"
	fdbfa "github.com/skypies/flightdb/flightdbfa"
	fdb24 "github.com/skypies/flightdb/flightdbfr24"
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
	// http.HandleFunc("/fdb/test", testFdbHandler)
	http.HandleFunc("/fdb/deb", debugHandler)
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
					url := fmt.Sprintf("/fdb/addflight?deb=%s", fs.F.UniqueIdentifier())
					t := taskqueue.NewPOSTTask(url, map[string][]string{
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

	log := fmt.Sprintf("* addFlightHandler invoked: %s\n", time.Now().UTC())
	
	fsStr := r.FormValue("flightsnapshot")
	fs := ftype.FlightSnapshot{}
	if err := fs.Base64Decode(fsStr); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fr24Id := fs.F.Id.ForeignKeys["fr24"] // This is the only field we take from the 
	log += fmt.Sprintf("* fr24 key: %s\n", fr24Id)
	
	db := fdb.FlightDB{C: c}
	fr24db,err := fdb24.NewFlightDBFr24(urlfetch.Client(c));
	if err != nil {
		c.Errorf(" /mdb/addflight: newdb: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Be idempotent - check to see if this flight has already been recorded
	/* Can't do this check now; the fs.F.Id we cached might be different from the
   * f.Id we get back from LookupPlayback(), because fr24 reuse their keys.
   *
	if exists,err := db.FlightExists(fs.F.Id.UniqueIdentifier()); err != nil {
		c.Errorf(" /mdb/addflight: FlightExists check failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else if exists {
		c.Infof(" /mdb/addflight: already exists %s", fs)
		w.Write([]byte(fmt.Sprintf("Skipped %s\n", fs)))
		return
	}
	log += fmt.Sprintf("* FlightExists('%s') -> false\n", fs.F.Id.UniqueIdentifier())
  */

	// Now grab an initial flight (with track), from fr24.
	var f *ftype.Flight
	if f,err = fr24db.LookupPlayback(fr24Id); err != nil {
		// c.Errorf(" /mdb/addflight: lookup: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log += fmt.Sprintf("* the fr24/default track has %d points\n", len(f.Track))

	// Kludge: fr24 keys get reused, so the flight fr24 thinks it refers to might be
	// different than when we cached it. So we do the uniqueness check here, to avoid
	// dupes in the DB. Need a better solution to this.
	if exists,err := db.FlightExists(f.Id.UniqueIdentifier()); err != nil {
		c.Errorf(" /mdb/addflight: FlightExists check failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else if exists {
		c.Infof(" /mdb/addflight: already exists %s", *f)
		w.Write([]byte(fmt.Sprintf("Skipped %s\n", *f)))
		return
	}
	log += fmt.Sprintf("* FlightExists('%s') -> false\n", f.Id.UniqueIdentifier())

	
	// If we have any locally received ADSB fragments for this flight, add them in
	if err := db.MaybeAddTrackFragmentsToFlight(f); err != nil {
		c.Errorf(" /mdb/addflight: addTrackFrags(%s): %v", f.Id, err)
	}

	f.AnalyseFlightPath() // Takes a coarse look at the flight path
	log += fmt.Sprintf("* Initial tags: %v\n", f.TagList())
	
	// For flights on the SERFR1 or BRIXX1 approaches, fetch a flightaware track
	if f.HasTag(ftype.KTagSERFR1) || f.HasTag(ftype.KTagBRIXX) {
		u,p := kFlightawareAPIUsername,kFlightawareAPIKey
		if err := fdbfa.AddFlightAwareTrack(urlfetch.Client(c),f,u,p); err != nil {
			c.Errorf(" /mdb/addflight: addflightaware: %v", err)
		}
	}
				
	f.Analyse()
	
	if err := db.PersistFlight(*f, log); err != nil {
		c.Errorf(" /mdb/addflight: persist: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Success !
	w.Write([]byte(fmt.Sprintf("Added %s\n", f)))
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

// {{{ MapPoint{}, MapLine{}

type MapPoint struct {
	ITP   *ftype.InterpolatedTrackPoint
	TP    *ftype.TrackPoint
	Pos   *geo.Latlong

	Icon   string  // Name of /static/dot-<foo>.png
	Text   string	
}

func (mp MapPoint)ToJSStr(text string) string {
	if mp.Icon == "" { mp.Icon = "pink" }
	tp := ftype.TrackPoint{Source:"n/a"}
	
	if mp.ITP != nil {
		tp = mp.ITP.TrackPoint
		tp.Source += "/interp"
		mp.Text = fmt.Sprintf("** Interpolated trackpoint\n"+
			" * Pre :%s\n * This:%s\n * Post:%s\n * Ratio: %.2f\n%s",
			mp.ITP.Pre, mp.ITP, mp.ITP.Post, mp.ITP.Ratio, mp.Text)
	} else if mp.TP != nil {
		tp = *mp.TP
		mp.Text = fmt.Sprintf("** Raw TP\n* %s\n%s", mp.TP, mp.Text)
	} else {
		tp.Latlong = *mp.Pos
	}

	mp.Text += text
	
	return fmt.Sprintf("source:\"%s\", pos:{lat:%.6f,lng:%.6f}, "+
		"alt:%.0f, speed:%.0f, icon:%q, info:%q",
		tp.Source, tp.Latlong.Lat, tp.Latlong.Long, tp.AltitudeFeet, tp.SpeedKnots, mp.Icon, mp.Text)
}


type MapLine struct {
	Line        *geo.LatlongLine
	Start, End  *geo.Latlong

	Color  string  // A hex color value (e.g. "#ff8822")
}
func (ml MapLine)ToJSStr(text string) string {
	color := ml.Color
	if color == "" { color = "#000000" }

	if ml.Line != nil {
		return fmt.Sprintf("s:{lat:%f, lng:%f}, e:{lat:%f, lng:%f}, color:\"%s\"",
			ml.Line.From.Lat, ml.Line.From.Long, ml.Line.To.Lat, ml.Line.To.Long, color) 
	} else {
		return fmt.Sprintf("s:{lat:%f, lng:%f}, e:{lat:%f, lng:%f}, color:\"%s\"",
			ml.Start.Lat, ml.Start.Long, ml.End.Lat, ml.End.Long, color) 
	}
}

func LatlongTimeBoxToMapLines(tb geo.LatlongTimeBox, color string) []MapLine {
	SW,NE,SE,NW := tb.SW, tb.NE, tb.SE(), tb.NW()
	if color == "" { color = "#22aa33" }
	lines := []MapLine{
		MapLine{Start:&SE, End:&SW, Color:color},
		MapLine{Start:&SW, End:&NW, Color:color},
		MapLine{Start:&NW, End:&NE, Color:color},
		MapLine{Start:&NE, End:&SE, Color:color},
	}
	return lines
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
			
		f.Analyse()  // Repopulate the flight tags; useful when debugging new analysis stuff

		// Todo: collapse all these separate tracks down into the single point/line list thing
		fr24TrackJSVar := classBTrack.ToJSVar()

		// For Flightaware tracks
		//faClassBTrack := fdb.ClassBTrack{}
		faTrackJSVar   := template.JS("{}")
		if _,exists := f.Tracks["FA"]; exists==true {
			_,faClassBTrack := f.SFOClassB("FA")
			faTrackJSVar = faClassBTrack.ToJSVar()
			//fr24TrackJSVar = template.JS("{}")
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

		mapPoints := []MapPoint{}
		mapLines  := []MapLine{}

		// &waypoint=EPICK
		if waypoint := r.FormValue("waypoint"); waypoint != "" {
			//pos := geo.Latlong{37.060312, -121.990814}
			pos := sfo.KFixes[waypoint]
			if itp,err := f.BestTrack().PointOfClosestApproach(pos); err != nil {
				c.Infof(" ** Error: %v", err)
			} else {
				mapPoints = append(mapPoints, MapPoint{Icon:"red", ITP:&itp})
				mapPoints = append(mapPoints, MapPoint{Pos:&itp.Ref, Text:"** Reference point"})
				mapLines = append(mapLines, MapLine{Line:&itp.Line, Color:"#ff8822"})
				mapLines = append(mapLines, MapLine{Line:&itp.Perp, Color:"#ff2288"})
			}
		}

		// &boxes=1
		if r.FormValue("boxes") != "" {
			if true {
				// fr24
				for _,box := range f.Track.AsContiguousBoxes() {
					mapLines = append(mapLines, LatlongTimeBoxToMapLines(box, "#118811")...)  
				}
			}
			if t,exists := f.Tracks["FA"]; exists==true {
				for _,box := range t.AsContiguousBoxes() {
					mapLines = append(mapLines, LatlongTimeBoxToMapLines(box, "#1111aa")...)
				}
			}
			if t,exists := f.Tracks["ADSB"]; exists==true {
				for _,box := range t.AsContiguousBoxes() {
					mapLines = append(mapLines, LatlongTimeBoxToMapLines(box, "#aaaa11")...)
				}
			}
		}
		
		pointsStr := "{\n"
		for i,mp := range mapPoints { pointsStr += fmt.Sprintf("    %d: {%s},\n", i, mp.ToJSStr("")) }
		pointsJS := template.JS(pointsStr + "  }\n")		
		linesStr := "{\n"
		for i,ml := range mapLines { linesStr += fmt.Sprintf("    %d: {%s},\n", i, ml.ToJSStr("")) }
		linesJS := template.JS(linesStr + "  }\n")
		
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

			"Points": pointsJS,
			"Lines": linesJS,
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
// {{{ debugHandler

func debugHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	db := fdb.FlightDB{C: c}
	id := r.FormValue("id")
	blob,f,err := db.GetBlobById(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else if blob == nil {
		http.Error(w, fmt.Sprintf("id=%s not found", id), http.StatusInternalServerError)
		return
	}

	blob.Flight = []byte{} // Zero this out

	s,e := f.Track.TimesInBox(sfo.KBoxSFO120K)
	log := blob.GestationLog
	blob.GestationLog = ""

	str := fmt.Sprintf("OK\n* Flight found: %s\n* Tracks: %q\n", f, f.TrackList())
	str += fmt.Sprintf("* Points in default track: %d\n", len(f.Track))
	str += fmt.Sprintf("* Default's start/end: %s, %s\n", s, e)
	str += fmt.Sprintf("\n** Gestation log:-\n%s\n", log)
	str += fmt.Sprintf("** Blob:-\n* Id=%s, Icao24=%s\n* Enter/Leave: %s -> %s\n* Tags: %v\n",
		blob.Id, blob.Icao24, blob.EnterUTC, blob.LeaveUTC, blob.Tags)

	str += "\n**** Tracks\n\n"
	str += fmt.Sprintf("** %s [DEFAULT]\n", f.Track)
	for _,t := range f.Tracks { str += fmt.Sprintf("** %s\n", t) }

	consistent,debug := f.TracksAreConsistentDebug()
	str += fmt.Sprintf("\n** Track consistency (%v)\n\n%s\n", consistent, debug)
	
	//str += "\n** Default track\n"
	//for _,tp := range f.Track { str += fmt.Sprintf(" * %s\n", tp) }
	
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(str))
}

// }}}

/* To populate the dev server flight DB: comment out the 45m timeout, then call
 *  localhost:8080/fdb/scan */

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
