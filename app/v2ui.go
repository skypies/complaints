package complaints

import(
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	oldappengine "appengine"

	"github.com/skypies/util/widget"

	oldfgae "github.com/skypies/flightdb/gae"
	newfdb  "github.com/skypies/flightdb2"
	newui   "github.com/skypies/flightdb2/ui"
)

func init() {
	http.HandleFunc("/map", newui.MapHandler)

	http.HandleFunc("/fdb/track2", v2TrackHandler)
	//http.HandleFunc("/fdb/trackset2", v2TracksetHandler)
	http.HandleFunc("/fdb/trackset3", v3TracksetHandler)
	http.HandleFunc("/fdb/approach2", v2ApproachHandler)
	http.HandleFunc("/fdb/json2", v2JsonHandler)
	http.HandleFunc("/fdb/vector", vectorHandler)
}

// Provides thin wrappers to do DB lookup & data model upgrade, and then passes over
// to the rendering routines in the New Shiny, flightdb2/ui

// {{{ idspecsToFlightV2s, idspecsToMapLines

func FormValueIdSpecs(r *http.Request) ([]string, error) {
	ret := []string{}
	r.ParseForm()

	if r.FormValue("idspec") != "" {
		for _,v := range r.Form["idspec"] {
			for _,str := range strings.Split(v, ",") {
				ret = append(ret, str)
			}
		}
	} else if r.FormValue("id") != "" {
		for _,v := range r.Form["id"] {
			for _,str := range strings.Split(v, ",") {
				ret = append(ret, str)
			}
		}
	}
	
	return ret, nil
}

func idspecsToFlightV2s(r *http.Request) ([]*newfdb.Flight, error) {
	c := oldappengine.NewContext(r)
	db := oldfgae.FlightDB{C:c}
	newFlights := []*newfdb.Flight{}

	idspecs,err := FormValueIdSpecs(r)
	if err != nil {
		return newFlights, err
	}

	for _,idspec := range idspecs {
		oldF,err := db.LookupById(idspec)
		if err != nil {
			return newFlights, err
		} else if oldF == nil {
			return newFlights, fmt.Errorf("flight '%s' not found", idspec)
		}
		newF,err := oldF.V2()
		if err != nil {
			return newFlights, err
		}
		newFlights = append(newFlights, newF)
	}

	return newFlights, nil
}

func idspecsToMapLines(r *http.Request) ([]newui.MapLine, error) {
	c := oldappengine.NewContext(r)
	db := oldfgae.FlightDB{C:c}

	lines := []newui.MapLine{}
	
	idspecs,err := FormValueIdSpecs(r)
	if err != nil {
		return lines, err
	}

	for _,idspec := range idspecs {
		oldF,err := db.LookupById(idspec)
		if err != nil {
			return lines, err
		} else if oldF == nil {
			return lines, fmt.Errorf("flight '%s' not found", idspec)
		}
		newF,err := oldF.V2()
		if err != nil {
			return lines, err
		}
		flightLines := newui.FlightToMapLines(newF, "") // Sigh
		lines = append(lines, flightLines...)
	}

	return lines, nil
}

// }}}

// {{{ v2TrackHandler

func v2TrackHandler(w http.ResponseWriter, r *http.Request) {
	flights,err := idspecsToFlightV2s(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	newui.OutputTracksOnAMap(w, r, flights)
}

// }}}
// {{{ v2TracksetHandler

func v2TracksetHandler(w http.ResponseWriter, r *http.Request) {

	flightLines, err := idspecsToMapLines(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	newui.OutputMapLinesOnAMap(w, r, flightLines)
}

// }}}
// {{{ v2ApproachHandler

func v2ApproachHandler(w http.ResponseWriter, r *http.Request) {
	flights,err := idspecsToFlightV2s(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	newui.OutputApproachesAsPDF(w, r, flights)
}

// }}}
// {{{ v2JsonHandler

func v2JsonHandler(w http.ResponseWriter, r *http.Request) {
	flights,err := idspecsToFlightV2s(r)

	jsonBytes,err := json.Marshal(flights)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonBytes)
}

// }}}

// {{{ v3TracksetHandler

func v3TracksetHandler(w http.ResponseWriter, r *http.Request) {
	idspecs,err := FormValueIdSpecs(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	newui.OutputMapLinesOnAStreamingMap(w, r, idspecs)
}

// }}}

// {{{ vectorHandler

// ?idspec=F12123@144001232[,...]
// &json=1
// &track={ADSB|fr24|...}

func vectorHandler(w http.ResponseWriter, r *http.Request) {
	c := oldappengine.NewContext(r)
	db := oldfgae.FlightDB{C:c}

	if r.FormValue("json") == "" {
		http.Error(w, "vectorHandler is json only at the moment", http.StatusInternalServerError)
		return
	}

	idspec := ""
	if idspecs,err := FormValueIdSpecs(r); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else if len(idspecs) != 1 {
		http.Error(w, "vectorHandler takes exactly one idspec", http.StatusInternalServerError)
		return
	} else {
		idspec = idspecs[0]
	}
	
	w.Header().Set("Content-Type", "application/json")
	
	oldF,err := db.LookupById(idspec)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else if oldF == nil {
		http.Error(w, fmt.Sprintf("flight '%s' not found", idspec), http.StatusInternalServerError)
		return
	}
	newF,err := oldF.V2()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	trackName,_ := newF.PreferredTrack(widget.FormValueCommaSepStrings(r, "trackspec"))

	lines := newui.FlightToMapLines(newF, trackName)
	/*if r.FormValue("opacity") != "" {
		for i,_ := range lines {
			lines.Opacity = r.FormValue("opacity")
		}
	}*/
	jsonBytes,err := json.Marshal(lines)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(jsonBytes)
}

// }}}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
