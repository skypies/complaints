package complaints

import(
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	oldappengine "appengine"

	oldfgae "github.com/skypies/flightdb/gae"
	newfdb  "github.com/skypies/flightdb2"
	newui   "github.com/skypies/flightdb2/ui"
)

func init() {
	http.HandleFunc("/fdb/track2", v2TrackHandler)
	http.HandleFunc("/fdb/trackset2", v2TracksetHandler)
	http.HandleFunc("/fdb/approach2", v2ApproachHandler)
	http.HandleFunc("/fdb/json2", v2JsonHandler)
	http.HandleFunc("/fdb/map2", newui.MapHandler)
}

// Provides thin wrappers to do DB lookup & data model upgrade, and then passes over
// to the rendering routines in the New Shiny, flightdb2/ui

// {{{ idspecsToFlightV2s, idspecsToMapLines

func FormValueIdSpecs(r *http.Request) ([]string, error) {
	ret := []string{}
	r.ParseForm()
	for _,v := range r.Form["idspec"] {
		for _,str := range strings.Split(v, ",") {
			ret = append(ret, str)
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
		flightLines := newui.FlightToMapLines(newF)
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


// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
