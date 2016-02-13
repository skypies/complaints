package backend

import (
	"html/template"
	"fmt"
	"net/http"
	"strings"
	"time"
	
	oldappengine "appengine"
	newappengine "google.golang.org/appengine"
	newurlfetch "google.golang.org/appengine/urlfetch"

	"github.com/skypies/geo/sfo"
	"github.com/skypies/util/date"

	oldfdb  "github.com/skypies/flightdb"
	oldfgae "github.com/skypies/flightdb/gae"

	"github.com/skypies/flightdb2/report"
	"github.com/skypies/flightdb2/metar"
	_ "github.com/skypies/flightdb2/analysis" // populate the reports registry
)

func init() {
	http.HandleFunc("/report3", report3Handler)
	http.HandleFunc("/report3/", report3Handler)
}

// {{{ tagList

// This is a v1 hack, although not needed for a bit ...
func tagList(opt report.Options) []string {
	tags := []string{}
	tags = append(tags, opt.Tags...)

	for _,proc := range opt.Procedures {
		switch proc {
		case "SERFR": tags = append(tags, oldfdb.KTagSERFR1)
		case "BRIXX": tags = append(tags, oldfdb.KTagBRIXX)
		}
	}

	return tags
}

// }}}

// {{{ ButtonPOST

func ButtonPOST(anchor, action string, idspecs []string) string {
	// Would be nice to view the complement - approaches of flights that did not match
	posty := fmt.Sprintf("<form action=\"%s\" method=\"post\" target=\"_blank\">", action)
	posty += fmt.Sprintf("<button type=\"submit\" name=\"idspec\" value=\"%s\" "+
		"class=\"btn-link\">%s</button>", strings.Join(idspecs,","), anchor)
	posty += "</form>\n"
	return posty
}

// }}}

// {{{ report3Handler

func report3Handler(w http.ResponseWriter, r *http.Request) {
	if r.FormValue("rep") == "" {
		var params = map[string]interface{}{
			"Yesterday": date.NowInPdt().AddDate(0,0,-1),
			"Reports": report.ListReports(),
			"FormUrl": "/report3/",
			"Waypoints": sfo.ListWaypoints(),
		}
		if err := templates.ExecuteTemplate(w, "report3-form", params); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	
	rep,err := report.SetupReport(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if r.FormValue("debug") != "" {
		str := fmt.Sprintf("Report Options\n\n%s\n", rep.Options)
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(fmt.Sprintf("OK\n\n%s\n", str)))
		return
	}
	
	//airframes := ref.NewAirframeCache(c)
	client := newurlfetch.Client(newappengine.NewContext(r))
	metars,err := metar.FetchFromNOAA(client, "KSFO",
		rep.Start.AddDate(0,0,-1), rep.End.AddDate(0,0,1))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	v1idspecs := []string{}
	v2idspecs := []string{}

	v1idspecComplement := []string{}
	
	reportFunc := func(oldF *oldfdb.Flight) {
		newF,err := oldF.V2()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			newF.ComputeIndicatedAltitudes(metars)
			if included,err := rep.Process(newF); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			} else if included {
				v1idspecs = append(v1idspecs, oldF.UniqueIdentifier())
				v2idspecs = append(v2idspecs, newF.IdSpec())
			} else {
				v1idspecComplement = append(v1idspecComplement, oldF.UniqueIdentifier())
			}
		}
	}

	tags := tagList(rep.Options)

	db := oldfgae.FlightDB{C: oldappengine.Timeout(oldappengine.NewContext(r), 600*time.Second)}
	s,e := rep.Start,rep.End
	if err := db.LongIterWith(db.QueryTimeRangeByTags(tags,s,e), reportFunc); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	postButtons := ButtonPOST(fmt.Sprintf("%d Matches as a VectorMap", len(v1idspecs)),
		fmt.Sprintf("/fdb/trackset2?%s", rep.ToCGIArgs()), v1idspecs)
	postButtons += ButtonPOST(fmt.Sprintf("%d Non-matches as a VectorMap", len(v1idspecComplement)),
		fmt.Sprintf("/fdb/trackset2?%s", rep.ToCGIArgs()), v1idspecComplement)
	if rep.Name == "sfoclassb" {
		postButtons += ButtonPOST(fmt.Sprintf("%d Matches as ClassBApproaches", len(v1idspecs)),
			fmt.Sprintf("/fdb/approach2?%s", rep.ToCGIArgs()), v1idspecs)
		// This is kinda useless, as it isn't limited to SERFR1 things
		postButtons += ButtonPOST(fmt.Sprintf("%d Non-matches as ClassBApproaches", len(v1idspecComplement)),
			fmt.Sprintf("/fdb/approach2?%s", rep.ToCGIArgs()), v1idspecComplement)

		postButtons += ButtonPOST(fmt.Sprintf("%d Matches as ClassBApproaches (delta)",
			len(v1idspecs)), fmt.Sprintf("/fdb/approach2?%s&colorby=delta", rep.ToCGIArgs()), v1idspecs)
		// This is kinda useless, as it isn't limited to SERFR1 things
		postButtons += ButtonPOST(fmt.Sprintf("%d Non-matches as ClassBApproaches (delta)",
			len(v1idspecComplement)), fmt.Sprintf("/fdb/approach2?%s&colorby=delta",
			rep.ToCGIArgs()), v1idspecComplement)
	}
	
	var params = map[string]interface{}{
		"R": rep,
		"Metadata": rep.MetadataTable(),
		"PostButtons": template.HTML(postButtons),
		"OptStr": template.HTML(fmt.Sprintf("<pre>%s</pre>\n", rep.Options)),
		"IdSpecs": template.HTML(strings.Join(v1idspecs,",")),
	}
	if err := templates.ExecuteTemplate(w, "report3-results", params); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}	

}

// }}}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
