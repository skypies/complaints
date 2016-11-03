package complaints

import(
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	oldfgae "github.com/skypies/flightdb/gae"
	newfdb  "github.com/skypies/flightdb2"
	newui   "github.com/skypies/flightdb2/ui"
)

func init() {
	http.HandleFunc("/map", newui.MapHandler)

	http.HandleFunc("/fdb/track2", v2TrackHandler)
	http.HandleFunc("/fdb/trackset2", v2TracksetHandler)
	http.HandleFunc("/fdb/approach2", v2ApproachHandler)
	http.HandleFunc("/fdb/descent2", v2DescentHandler)
	http.HandleFunc("/fdb/json2", v2JsonHandler)
	http.HandleFunc("/fdb/vector2", v2VectorHandler)
	http.HandleFunc("/fdb/visualize2", v2VisualizeHandler)
}

// Provides thin wrappers to do DB lookup & data model upgrade, and then passes over
// to the rendering routines in the New Shiny, flightdb2/ui
// {{{ FormValueIdSpecs, idspecsToFlightV2s

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
	db := oldfgae.NewDB(r)
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
	idspecs,err := FormValueIdSpecs(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	newui.OutputMapLinesOnAStreamingMap(w, r, "", idspecs, "/fdb/vector2")
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
// {{{ v2DescentHandler

func v2DescentHandler(w http.ResponseWriter, r *http.Request) {
	/*
	flights,err := idspecsToFlightV2s(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	newui.OutputDescentsAsPDF(w, r, flights)
*/

	db := oldfgae.NewDB(r)

	idspecs,err := FormValueIdSpecs(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	dp := newui.DescentPDFInit(w, r, len(idspecs))

	if len(idspecs) > 10 {
		dp.LineThickness = 0.1
		dp.LineOpacity = 0.25
	}
	
	for _,idspec := range idspecs {
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

		if err := newui.DescentPDFAddFlight(r, dp, newF); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	
	newui.DescentPDFFinalize(w,r,dp)
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
// {{{ v2VectorHandler

// ?idspec=F12123@144001232[,...]
// &json=1
// &track={ADSB|fr24|...}

func v2VectorHandler(w http.ResponseWriter, r *http.Request) {
	if r.FormValue("json") == "" {
		http.Error(w, "vectorHandler is json only at the moment", http.StatusBadRequest)
		return
	}

	flights,err := idspecsToFlightV2s(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else if len(flights) != 1 {
		http.Error(w, "vectorHandler takes exactly one idspec", http.StatusBadRequest)
		return
	}

	newui.OutputFlightAsVectorJSON(w, r, flights[0])
}

// }}}
// {{{ v2VisualizeHandler

func v2VisualizeHandler(w http.ResponseWriter, r *http.Request) {
	switch r.FormValue("viewtype") {
	case "vector":   v2TracksetHandler(w,r)
	case "descent":  v2DescentHandler(w,r)
	case "track":    v2TrackHandler(w,r)
	default:         http.Error(w, "Specify viewtype={vector|descent|track}", http.StatusBadRequest)
	}		
}

// }}}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
