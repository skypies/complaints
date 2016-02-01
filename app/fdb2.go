package complaints

import(
	"html/template"
	"fmt"
	"net/http"
	"strings"
	"time"

	oldappengine "appengine"
	
	"github.com/skypies/geo/sfo"
	oldfgae "github.com/skypies/flightdb/gae"
)

func init() {
	http.HandleFunc("/fdb2/trackset", tracksetHandler)
}

// {{{ FormValueIdSpecs



// Presumes a form field 'idspec', as per identity.go
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

// }}}

// {{{ STOLEN FROM newfdb/mapshapes.go

func MapPointsToJSVar(points []MapPoint) template.JS {
	str := "{\n"
	for i,mp := range points {
		str += fmt.Sprintf("    %d: {%s},\n", i, mp.ToJSStr(""))
	}
	return template.JS(str + "  }\n")		
}

func MapLinesToJSVar(lines []MapLine) template.JS {
	str := "{\n"
	for i,ml := range lines {
		str += fmt.Sprintf("    %d: {%s},\n", i, ml.ToJSStr(""))
	}
	return template.JS(str + "  }\n")		
}

// }}}

// {{{ tracksetHandler

// STOLEN from fdb2

// ?idspec==XX,YY,...


func tracksetHandler(w http.ResponseWriter, r *http.Request) {
	c := oldappengine.NewContext(r)
	db := oldfgae.FlightDB{C:c}


	str := ""

	//colorscheme := FormValueColorScheme(r)
	idspecs,err := FormValueIdSpecs(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	lines  := []MapLine{}

	for _,idspec := range idspecs {
		f,err := db.LookupById(idspec)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		} else if f == nil {
			http.Error(w, fmt.Sprintf("idspec %s not found", idspec), http.StatusInternalServerError)
			return
		}

		sampleRate := time.Millisecond * 2500
		t := f.Track.SampleEvery(sampleRate, false)
		for i,_ := range t[1:] { // Line from i to i+1
			color := "#ffaa66"
			line := MapLine{
				Start: &t[i].Latlong,
				End: &t[i+1].Latlong,
				Color: color,
			}
			lines = append(lines, line)
		}
	}
	
	if r.FormValue("debug") != "" {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(fmt.Sprintf("OK\n\n%s", str)))
		return
	}

	legend := fmt.Sprintf("%d tracks", len(idspecs))
	var params = map[string]interface{}{
		"Legend": legend,
		"Points": template.JS("{}"),
		"Lines": MapLinesToJSVar(lines),
		"MapsAPIKey": "",//kGoogleMapsAPIKey,
		"Center": sfo.KFixes["EPICK"], //sfo.KLatlongSFO,
		"Zoom": 8,
	}
	if err := templates.ExecuteTemplate(w, "fdb2-tracks", params); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// }}}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
