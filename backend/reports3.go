package backend

import (
	"html/template"
	"fmt"
	"net/http"
	"strings"
	
	"google.golang.org/appengine"
	"google.golang.org/appengine/urlfetch"

	"github.com/skypies/geo/sfo"
	"github.com/skypies/util/date"

	oldfdb  "github.com/skypies/flightdb"
	oldfgae "github.com/skypies/flightdb/gae"

	"github.com/skypies/flightdb2/report"
	"github.com/skypies/flightdb2/metar"
	_ "github.com/skypies/flightdb2/analysis" // populate the reports registry
)

func init() {
	http.HandleFunc("/report", report3Handler)
	http.HandleFunc("/report/", report3Handler)
}

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
// {{{ maybeButtonPOST

func maybeButtonPOST(idspecs []string, title string, url string) string {
	if len(idspecs) == 0 { return "" }

	return ButtonPOST(
		fmt.Sprintf("%d %s", len(idspecs), title),
		fmt.Sprintf("http://stop.jetnoise.net%s", url),
		idspecs)
}

// }}}

// {{{ report3Handler

func report3Handler(w http.ResponseWriter, r *http.Request) {
	if r.FormValue("rep") == "" {
		var params = map[string]interface{}{
			"Yesterday": date.NowInPdt().AddDate(0,0,-1),
			"Reports": report.ListReports(),
			"FormUrl": "/report/",
			"Waypoints": sfo.ListWaypoints(),
			"Title": "Reports (DB v1)",
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
	client := urlfetch.Client(appengine.NewContext(r))
	metars,err := metar.FetchFromNOAA(client, "KSFO",
		rep.Start.AddDate(0,0,-1), rep.End.AddDate(0,0,1))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	v1idspecs := []string{}
	v2idspecs := []string{}
	v1RejectByRestrict := []string{}
	v1RejectByReport := []string{}
	
	reportFunc := func(oldF *oldfdb.Flight) {
		newF,err := oldF.V2()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		newF.ComputeIndicatedAltitudes(metars)

		outcome,err := rep.Process(newF)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		switch outcome {
		case report.RejectedByGeoRestriction:
			v1RejectByRestrict = append(v1RejectByRestrict, oldF.UniqueIdentifier())
		case report.RejectedByReport:
			v1RejectByReport = append(v1RejectByReport, oldF.UniqueIdentifier())
		case report.Accepted:
			v1idspecs = append(v1idspecs, oldF.UniqueIdentifier())
			v2idspecs = append(v2idspecs, newF.IdSpec().String())
		}
	}

	tags := rep.Options.Tags

	for _,wp := range rep.Waypoints {
		tags = append(tags, fmt.Sprintf("%s%s", oldfdb.KWaypointTagPrefix, wp))
	}
	
	db := oldfgae.NewDB(r)
	s,e := rep.Start,rep.End
	if err := db.LongIterWith(db.QueryTimeRangeByTags(tags,s,e), reportFunc); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rep.FinishSummary()

	if rep.ResultsFormat == "csv" {
		rep.OutputAsCSV(w)
		return
	}

	postButtons := ""
	url := fmt.Sprintf("/fdb/trackset2?%s", rep.ToCGIArgs())
	postButtons += maybeButtonPOST(v1RejectByRestrict, "Restriction Rejects as VectorMap",url)
	postButtons += maybeButtonPOST(v1RejectByReport, "Report Rejects as VectorMap", url)
	
	url = fmt.Sprintf("/fdb/descent2?%s", rep.ToCGIArgs())
	postButtons += maybeButtonPOST(v1RejectByRestrict, "Restriction Rejects as DescentGraph",url)
	postButtons += maybeButtonPOST(v1RejectByReport, "Report Rejects DescentGraph", url)
		
	if rep.Name == "sfoclassb" {
		url = fmt.Sprintf("/fdb/approach2?%s", rep.ToCGIArgs())
		postButtons += maybeButtonPOST(v1idspecs, "Matches as ClassB",url)
		postButtons += maybeButtonPOST(v1RejectByReport, "Report Rejects as ClassB", url)
	}

	// The only way to get embedded CGI args without them getting escaped is to submit a whole tag
	vizFormURL := "http://stop.jetnoise.net/fdb/visualize2?"+rep.ToCGIArgs()
	vizFormTag := "<form action=\""+vizFormURL+"\" method=\"post\" target=\"_blank\">"
	
	var params = map[string]interface{}{
		"R": rep,
		"Metadata": rep.MetadataTable(),
		"PostButtons": template.HTML(postButtons),
		"OptStr": template.HTML(fmt.Sprintf("<pre>%s</pre>\n", rep.Options)),
		"IdSpecs": template.HTML(strings.Join(v1idspecs,",")),
		"VisualizationFormTag": template.HTML(vizFormTag),
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
