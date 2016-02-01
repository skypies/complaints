package complaints

// {{{ import()

import (
	//"encoding/csv"
	"html/template"
	"fmt"
	"net/http"
	"strings"
	"time"
	
	oldappengine "appengine"
	//newappengine "google.golang.org/appengine"
	//"golang.org/x/net/context"

	//"github.com/skypies/geo"
	"github.com/skypies/geo/sfo"
	"github.com/skypies/util/date"
	//"github.com/skypies/util/gcs"
	//"github.com/skypies/util/histogram"
	"github.com/skypies/util/widget"

	"github.com/skypies/flightdb"
	oldfgae "github.com/skypies/flightdb/gae"

	//"github.com/skypies/complaints/complaintdb"
)

// }}}

func init() {
	http.HandleFunc("/report2", reportHandler)
	http.HandleFunc("/report2/", reportHandler)
}
func bool2string(b bool) string { if b { return "1" } else { return "" } }
func flight2Url(f flightdb.Flight) template.HTML {
	return template.HTML("/fdb/lookup?map=1&id="+f.Id.UniqueIdentifier())
}

type ReportOptions struct {
	// Class B
	ClassB_OnePerFlight  bool
	ClassB_LocalDataOnly bool
	// Skimmers
	Skimmer_AltitudeTolerance float64
	Skimmer_MinDurationNM     float64
	// Waypoint stuff
	Waypoint string
	
	// General
  TrackDataSource  string

	//// The META report
	// Need some more general widgets for 'region of interest' (pos+radius; pos+side)
	// Then for things that pass through regions; altitude/speed/airline/equip filters
}
// {{{ FormValueReportOptions

func FormValueReportOptions(w http.ResponseWriter, r *http.Request) (ReportOptions, error) {
	opt := ReportOptions{
		ClassB_OnePerFlight: widget.FormValueCheckbox(r, "classb_oneperflight"),
		ClassB_LocalDataOnly: widget.FormValueCheckbox(r, "classb_localdataonly"),
		Skimmer_AltitudeTolerance: widget.FormValueFloat64(w,r,"skimmer_altitude_tolerance"),
		Skimmer_MinDurationNM: widget.FormValueFloat64(w,r,"skimmer_min_duration_nm"),
	}

	switch r.FormValue("datasource") {
	case "adsb": opt.TrackDataSource = "ADSB"
	case "fa":   opt.TrackDataSource = "FA"
	default:     opt.TrackDataSource = "fr24"
	}
	
	if fix := strings.ToUpper(r.FormValue("waypoint")); fix != "" {
		if _,exists := sfo.KFixes[fix]; !exists {
			err := fmt.Errorf("Waypoint '%s' not known",fix)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return opt, err
		}
		opt.Waypoint = fix
	}

	return opt, nil
}

// }}}

// {{{ GenericRow{}

/*
func (c GenericRow)ToCSVHeaders() []string {
	return []string{
		"Airline", "Flightnumber", "Origin", "Destination",
		"Registration", "Icao24", "EnterDate(PDT)", "EnterTime(PDT)", "Description"}
}
func (r GenericRow) ToCSV() []string {
	return []string{
		r.F.Id.Designator.IATAAirlineDesignator,
		r.F.Id.Designator.String(),
		r.F.Id.Origin,
		r.F.Id.Destination,
		r.F.Id.Registration,
		r.F.Id.ModeS,
		date.InPdt(r.F.EnterUTC).Format("2006/01/02"),
		date.InPdt(r.F.EnterUTC).Format("15:04:05.999999999"),
		string(r.Str),
	}
}

type Row interface {
	ToCSVHeaders() []string
	ToCSV() []string
	//ToHTMLRow() []string
}
*/

// }}}
// {{{ GenericReport{}

type GenericRow struct {
	Url     template.HTML
	F       flightdb.Flight
	Str     template.HTML
	Cells []template.HTML
}

type GenericReport struct {
	Rows    []GenericRow
	Numbers   map[string]float64
	Strings   map[template.HTML]template.HTML
}

type GenericReporter interface {
	Generate(req *http.Request, s,e time.Time, opt ReportOptions) error
	GetRows()     []GenericRow
	GetNumbers()    map[string]float64
	GetStrings()    map[template.HTML]template.HTML
}

// }}}

// {{{ LevelFlightReport

type LevelFlightReport GenericReport
func (r *LevelFlightReport)GetRows() []GenericRow { return r.Rows }
func (r *LevelFlightReport)GetNumbers() map[string]float64 { return r.Numbers }
func (r *LevelFlightReport)GetStrings() map[template.HTML]template.HTML { return r.Strings }

func (r *LevelFlightReport)Generate(req *http.Request, s,e time.Time, opt ReportOptions) error {
	r.Rows = []GenericRow{}
	r.Numbers = map[string]float64{}
	r.Strings = map[template.HTML]template.HTML{}
	
	n := 0
	ids := []string{}
	reportFunc := func(f *flightdb.Flight) {
		n++

		if !f.Id.IsScheduled() { return } // We only want scheduled flights
		
		// We shouldn't use the ADSB data here, as it peters out over Palo Alto
		t := f.Track

		if opt.TrackDataSource == "FA" {
			if f.HasTrack("FA") {
				t = f.Tracks["FA"]
			} else {
				return // No track data
			}
		}
		
		ev := t.LevelFlightAcrossBox(opt.Skimmer_AltitudeTolerance, sfo.KBoxPaloAlto20K, "Palo Alto")
		if ev == nil {
			return
		}

		if ev.Start.AltitudeFeet > 8000.0 {
			return
		}
		
		ids = append(ids, f.Id.UniqueIdentifier())
		fClone := f.ShallowCopy()
		fClone.Tags = map[string]bool{}
		url := flight2Url(*fClone) + "&report=level"
		
		row := GenericRow{ url, *fClone, template.HTML(""), ev.Cells() }
		r.Rows = append(r.Rows, row)
	}

	db := oldfgae.FlightDB{C: oldappengine.Timeout(oldappengine.NewContext(req), 60*time.Second)}
	// tags := []string{flightdb.KTagSERFR1}  // FIXME - should be in opt, really
	tags := []string{}
	
	if err := db.IterWith(db.QueryTimeRangeByTags(tags,s,e), reportFunc); err != nil {
		return err
	}

	r.Numbers["[A] Total flights examined "] = float64(n)

	tracksUrl := "http://stop.jetnoise.net/fdb2/trackset?idspec="
	tracksUrl += strings.Join(ids, ",")
	r.Strings[template.HTML("URLs")] = template.HTML(fmt.Sprintf("[<a target=\"_blank\" href=\"%s\">TrackSet</a>]", tracksUrl))
	
	return nil
}

// }}}
// {{{ AltitudeStackingReport

type AltitudeStackingReport GenericReport
func (r *AltitudeStackingReport)GetRows() []GenericRow { return r.Rows }
func (r *AltitudeStackingReport)GetNumbers() map[string]float64 { return r.Numbers }
func (r *AltitudeStackingReport)GetStrings() map[template.HTML]template.HTML { return r.Strings }

func alt2bkt(f float64) string {
	g := float64(int((f+500)/1000.0))  // Round to nearest thousand: 11499 -> 11, 11501 -> 12	
	return fmt.Sprintf("%05.0f-%05.0f", g*1000-500, g*1000+500)
}

func (r *AltitudeStackingReport)Generate(req *http.Request, s,e time.Time, opt ReportOptions) error {
	r.Rows = []GenericRow{}
	r.Numbers = map[string]float64{}
	r.Strings = map[template.HTML]template.HTML{}
	
	ids := []string{}
	reportFunc := func(f *flightdb.Flight) {
		r.Numbers["[A] Total flights examined "] += 1

		if !f.Id.IsScheduled() { return } // We only want scheduled flights
		
		// We shouldn't use the ADSB data here, as it peters out over Palo Alto
		t := f.Track
		if opt.TrackDataSource == "FA" {
			if f.HasTrack("FA") {
				t = f.Tracks["FA"]
			} else {
				return // No track data
			}
		}

		// We hijack this, and ignore the 'level flight' aspect
		ev := t.LevelFlightAcrossBox(10000.0, sfo.KBoxSFO10K, "SFO")
		if ev == nil {
			return
		}

		avg := ev.Start.AltitudeFeet + (ev.End.AltitudeFeet - ev.Start.AltitudeFeet) / 2.0
		bkt := alt2bkt(avg)

		if avg < 8000.0 { return }
		
		r.Numbers["[A] Flights passing through box, above 8000'"] += 1
		r.Numbers[fmt.Sprintf("[B] %s ",bkt)] += 1
		
		ids = append(ids, f.Id.UniqueIdentifier())
		fClone := f.ShallowCopy()
		fClone.Tags = map[string]bool{}
		url := flight2Url(*fClone) + "&report=stack"
		
		row := GenericRow{ url, *fClone, template.HTML(""), ev.Cells() }
		r.Rows = append(r.Rows, row)
	}

	tags := []string{}	
	db := oldfgae.FlightDB{C: oldappengine.Timeout(oldappengine.NewContext(req), 60*time.Second)}
	if err := db.IterWith(db.QueryTimeRangeByTags(tags,s,e), reportFunc); err != nil {
		return err
	}

	tracksUrl := "http://stop.jetnoise.net/fdb2/trackset?idspec="
	tracksUrl += strings.Join(ids, ",")
	r.Strings[template.HTML("URLs")] = template.HTML(fmt.Sprintf("[<a target=\"_blank\" href=\"%s\">TrackSet</a>]", tracksUrl))
	
	return nil
}

// }}}

// {{{ reportHandler

func reportHandler(w http.ResponseWriter, r *http.Request) {
	if r.FormValue("date") == "" {
		var params = map[string]interface{}{
			"Yesterday": date.NowInPdt().AddDate(0,0,-1),
		}
		if err := templates.ExecuteTemplate(w, "report2-form", params); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	opt,err := FormValueReportOptions(w,r)
	if err != nil { return }
	
	reportWriter (w,r,opt, r.FormValue("reportname"), r.FormValue("resultformat"))
}

// }}}
// {{{ reportWriter

func reportWriter (w http.ResponseWriter, r *http.Request, opt ReportOptions, rep string, format string) {

	s,e,_ := widget.FormValueDateRange(r)	
	
	// Interfacery: http://play.golang.org/p/I7GQNx2n_m
	var report GenericReporter
	switch rep {
	case "level": l := LevelFlightReport{}; report = &l
	case "stack": a := AltitudeStackingReport{}; report = &a
	}

	if err := report.Generate(r,s,e,opt); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Consider other formats.
	
	var params = map[string]interface{}{
		"Name": rep,
		"Start": s,
		"End": e,
		"Numbers": report.GetNumbers(),
		"Strings": report.GetStrings(),
		"Rows":    report.GetRows(),
		"Options": opt,
	}		
	if err := templates.ExecuteTemplate(w, "report2-generic", params); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// }}}


// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
