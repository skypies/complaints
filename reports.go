package serfr0

import (
	"bytes"
	"encoding/csv"
	"encoding/gob"
	"html/template"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"
	
	"appengine"

	"github.com/skypies/complaints/complaintdb"

	"github.com/skypies/date"
	"github.com/skypies/geo"
	"github.com/skypies/geo/sfo"
	"github.com/skypies/flightdb"
	fdb "github.com/skypies/flightdb/gae"
	hist "github.com/skypies/histogram"
)

func init() {
	http.HandleFunc("/report", reportHandler)

	// Canned reports
	http.HandleFunc("/report/serfr1", cannedSerfr1Handler)
	http.HandleFunc("/report/discrep", cannedDiscrepHandler)
	http.HandleFunc("/report/classb", cannedClassBHandler)
	http.HandleFunc("/report/adsb", cannedAdsbClassBHandler)
	http.HandleFunc("/report/yesterday", cannedSerfr1ComplaintsHandler)
}

func bool2string(b bool) string { if b { return "1" } else { return "" } }
func flight2Url(f flightdb.Flight) template.HTML {
	return template.HTML("/fdb/lookup?map=1&id="+f.Id.UniqueIdentifier())
}

type ReportOptions struct {
	// Class B
	ClassB_OnePerFlight bool
	// Skimmers
	Skimmer_AltitudeTolerance float64
	Skimmer_MinDurationNM     float64
}

// {{{ ReportRow Interface

type ReportRow interface {
	ToCSVHeaders() []string
	ToCSV() []string
}

type ReportMetadata map[string]float64

// }}}

// {{{ maybeMemcache

func maybeMemcache(fdb *fdb.FlightDB, queryEnd time.Time) {
	if (time.Now().Sub(queryEnd) > time.Hour) {
		fdb.Memcache = true
	}
}


type memResults struct {
	Rows []ReportRow
	Meta ReportMetadata
}

func reportToMemcache(c appengine.Context, rows []ReportRow, meta ReportMetadata, memKey string) error {
	var buf bytes.Buffer
	dataToCache := memResults{Rows:rows, Meta:meta}
	if err := gob.NewEncoder(&buf).Encode(dataToCache); err != nil {
		c.Errorf(" #=== cdb error encoding item: %v", err)
		return err
	} else {
		b := buf.Bytes()
		complaintdb.BytesToShardedMemcache(c, memKey, b)
	}

	return nil
}

func reportFromMemcache(c appengine.Context, memKey string) ([]ReportRow, ReportMetadata, error) {
	if b,found := complaintdb.BytesFromShardedMemcache(c, memKey); found == true {
		buf := bytes.NewBuffer(b)
		results := memResults{}
		if err := gob.NewDecoder(buf).Decode(&results); err != nil { return nil,nil,err }
		c.Infof("=##== Report from cache ! %s", memKey)
		return results.Rows, results.Meta, nil
	} else {
		return nil,nil,fmt.Errorf("nothing found for '%s'", memKey)
	}
}

// }}}

// {{{ classbReport

type CBRow struct {
	Seq  int
	Url  template.HTML
	F    flightdb.Flight
	TP  *flightdb.TrackPoint
	A    geo.TPClassBAnalysis
}

func (c CBRow)ToCSVHeaders() []string {
	return []string{
		"Flightnumber", "Registration", "Icao24", "Date(PDT)", "Time(PDT)",
		"Dist2SFO", "Altitude", "BelowBy", "Lat", "Long", "DataSource"}
}
func (r CBRow) ToCSV() []string {
	return []string{
		r.F.Id.Designator.String(), r.F.Id.Registration, r.F.Id.ModeS,
		date.InPdt(r.TP.TimestampUTC).Format("2006/01/02"),
		date.InPdt(r.TP.TimestampUTC).Format("15:04:05.999999999"),
		fmt.Sprintf("%.1f",r.A.DistNM),
		fmt.Sprintf("%.0f",r.TP.AltitudeFeet),
		fmt.Sprintf("%.0f",r.A.BelowBy),
		fmt.Sprintf("%.4f",r.TP.Latlong.Lat),
		fmt.Sprintf("%.4f",r.TP.Latlong.Long),
		r.TP.LongSource(),
	}
}

func classbReport(c appengine.Context, s,e time.Time, opt ReportOptions) ([]ReportRow, ReportMetadata, error) {
	fdb := fdb.FlightDB{C: c}
	maybeMemcache(&fdb,e)
	c.Infof("Hey, memcache ! %v", fdb.Memcache)
	tags := []string{
		flightdb.KTagReliableClassBViolation,
		flightdb.KTagLocalADSBClassBViolation,
	}
	if flights,err := fdb.LookupTimeRangeByOrTags(tags,s,e); err != nil {
		return nil, nil, err
	} else {
		meta := ReportMetadata{}
		h := hist.Histogram{} // Only use it for the stats

		rows := []ReportRow{}

		if serfr1Flights,err := fdb.LookupTimeRangeByTags([]string{flightdb.KTagSERFR1},s,e); err == nil {
			meta["[A] Total SERFR1 Flights"] = float64(len(serfr1Flights))
		}

		for _,f := range flights {

			bestTrack := "FA"
			if f.HasTag(flightdb.KTagLocalADSBClassBViolation) { bestTrack = "ADSB" }

			_,cbt := f.SFOClassB(bestTrack)

			tmpRows :=[]ReportRow{}
			
			seq := 0
			for _,cbtp := range cbt {
				if cbtp.A.IsViolation() {					
					tmpRows = append(tmpRows, CBRow{ seq, flight2Url(f), f, cbtp.TP, cbtp.A } )
					seq++
				}
			}

			if len(tmpRows) == 0 { continue }
			
			worstCBRow := tmpRows[0].(CBRow)
			if seq > 0 {
				// Select the worst row
				n,belowBy := 0,0.0
				for i,row := range tmpRows {
					if row.(CBRow).A.BelowBy > belowBy { n,belowBy = i,row.(CBRow).A.BelowBy }
				}
				worstCBRow = tmpRows[n].(CBRow)
				worstCBRow.Seq = 0 // fake this out for the webpage
			}

			h.Add(hist.ScalarVal(worstCBRow.A.BelowBy))

			meta["[C] -- Detected via "+worstCBRow.TP.LongSource()]++
						
			if opt.ClassB_OnePerFlight {
				rows = append(rows, worstCBRow)
			} else {
				rows = append(rows, tmpRows...)
			}
		}

		if stats,valid := h.Stats(); valid {
			meta["[B] Num violating flights"] = float64(stats.N)
			meta["[D] Mean violation below Class B floor"] = float64(int(stats.Mean))
			meta["[D] Stddev"] = float64(int(stats.Stddev))
		} else {
			meta["[B] Num violating flights"] = 0.0
		}
		
		return rows, meta, nil
	}
}

// }}}
// {{{ serfr1Report

type SERFR1Row struct {
	Url             template.HTML
	F               flightdb.Flight
	HadAdsb         bool
	ClassBViolation bool
}

func (c SERFR1Row)ToCSVHeaders() []string {
	return []string{
		"Airline", "Flightnumber", "Origin", "Destination",
		"Registration", "Icao24", "EnterDate(PDT)", "EnterTime(PDT)",
		"HadADSB", "ClassBViolator"}
}
func (r SERFR1Row) ToCSV() []string {
	return []string{
		r.F.Id.Designator.IATAAirlineDesignator,
		r.F.Id.Designator.String(),
		r.F.Id.Origin,
		r.F.Id.Destination,
		r.F.Id.Registration,
		r.F.Id.ModeS,
		date.InPdt(r.F.EnterUTC).Format("2006/01/02"),
		date.InPdt(r.F.EnterUTC).Format("15:04:05.999999999"),
		bool2string(r.HadAdsb),
		bool2string(r.ClassBViolation),
	}
}

func serfr1Report(c appengine.Context, s,e time.Time, opt ReportOptions) ([]ReportRow, ReportMetadata, error) {
	meta := ReportMetadata{}
	fdb := fdb.FlightDB{C: c}
	maybeMemcache(&fdb,e)
	tags := []string{flightdb.KTagSERFR1}
	if flights,err := fdb.LookupTimeRangeByTags(tags,s,e); err != nil {
		return nil, nil, err

	} else {
		
		meta["[A] Total SERFR1 flights "] = float64(len(flights))

		out := []ReportRow{}
		for _,f := range flights {
			classBViolation := f.HasTag(flightdb.KTagReliableClassBViolation)		
			hasAdsb := false
			if _,exists := f.Tracks["ADSB"]; exists == true {
				meta["[B] with data from "+f.Tracks["ADSB"].LongSource()]++
			}
			if t,exists := f.Tracks["FA"]; exists == true {
				meta["[B] with data from "+f.Tracks["FA"].LongSource()]++
				hasAdsb = t.IsFromADSB()
			} else {
				meta["[B] with data from "+f.Track.LongSource()]++
			}

			row := SERFR1Row{ flight2Url(f), f, hasAdsb, classBViolation }
			out = append(out, row)
		}
		return out, meta, nil
	}
}

// }}}
// {{{ brixx1Report

func brixx1Report(c appengine.Context, s,e time.Time, opt ReportOptions) ([]ReportRow, ReportMetadata, error) {
	meta := ReportMetadata{}
	fdb := fdb.FlightDB{C: c}
	maybeMemcache(&fdb,e)
	tags := []string{flightdb.KTagBRIXX}
	if flights,err := fdb.LookupTimeRangeByTags(tags,s,e); err != nil {
		return nil, nil, err

	} else {
		out := []ReportRow{}
		
		meta["[A] Total BRIXX flights "] = float64(len(flights))

		for _,f := range flights {			
			hasAdsb := false
			if _,exists := f.Tracks["ADSB"]; exists == true {
				meta["[B] With data from "+f.Tracks["ADSB"].LongSource()]++
			}
			if t,exists := f.Tracks["FA"]; exists == true {
				meta["[B] With data from "+f.Tracks["FA"].LongSource()]++
				hasAdsb = t.IsFromADSB()
			} else {
				meta["[B] With data from "+f.Track.LongSource()]++
			}

			row := SERFR1Row{ flight2Url(f), f, hasAdsb, false }
			out = append(out, row)

		}
		return out, meta, nil
	}
}

// }}}
// {{{ serfr1ComplaintsReport

type SCRow struct {
	NumComplaints      int
	NumSpeedbrakes     int
	WeightedComplaints int
	F                  flightdb.Flight
}

func (c SCRow)ToCSVHeaders() []string {
	return []string{
		"Airline", "Flightnumber", "Origin", "Destination",
		"EnterDate(PDT)", "EnterTime(PDT)", "Tags",
		"NumComplaints", "NumSpeedbrakes", "BadnessScore",
	}
}
func (r SCRow)ToCSV() []string {
	return []string{
		r.F.Id.Designator.IATAAirlineDesignator,
		r.F.Id.Designator.String(),
		r.F.Id.Origin,
		date.InPdt(r.F.EnterUTC).Format("2006/01/02"),
		date.InPdt(r.F.EnterUTC).Format("15:04:05.999999999"),
		fmt.Sprintf("%v", r.F.TagList()),
		fmt.Sprintf("%d", r.NumComplaints),
		fmt.Sprintf("%d", r.NumSpeedbrakes),
		fmt.Sprintf("%d", r.WeightedComplaints),
	}
}

type SCRowByNumComplaints []SCRow
func (a SCRowByNumComplaints) Len() int           { return len(a) }
func (a SCRowByNumComplaints) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a SCRowByNumComplaints) Less(i, j int) bool {
	if a[i].NumComplaints != a[j].NumComplaints {
		return a[i].NumComplaints > a[j].NumComplaints
	} else {
		return a[i].WeightedComplaints > a[j].WeightedComplaints
	}
}

func serfr1ComplaintsReport(c appengine.Context, s,e time.Time, opt ReportOptions) ([]ReportRow, ReportMetadata, error) {
	fdb := fdb.FlightDB{C: c}
	maybeMemcache(&fdb,e)
	cdb := complaintdb.ComplaintDB{C: c, Memcache:true}
	meta := ReportMetadata{}

	memKey := "asdasdasd"
	
	if rows, meta, err := reportFromMemcache(c, memKey); err == nil {
		return rows,meta,err
	}
	
	tags := []string{flightdb.KTagSERFR1}
	if flights,err := fdb.LookupTimeRangeByTags(tags,s,e); err != nil {
		return nil, nil, err

	} else if complaints,err := cdb.GetComplaintsInSpan(s,e); err != nil {
		return nil, nil, err

	} else {
		meta["[A] Total overflights"] = float64(len(flights))
		meta["[B] Total disturbance reports"] = float64(len(complaints))
		meta["[C] Average number of reports per overflight"] =
			float64(int64(float64(len(complaints)) / float64(len(flights))))
		// c.Infof("I have %d flights, %d complaints\n", len(flights), len(complaints))
		// Aggregate complaints per flight
		nC,nSB,nWC := map[string]int{},map[string]int{},map[string]int{}
		for _,comp := range complaints {
			if k := comp.AircraftOverhead.FlightNumber; k != "" {
				nC[k] += 1
				nWC[k] += comp.Loudness
				if comp.HeardSpeedbreaks { nSB[k] += 1; nWC[k] += 4 }
			}
		}

		nFpC := map[string]int{}  // num flights per carrier
		nCpC := map[string]int{}  // num complaints per carrier
		
		rows := []SCRow{}
		for _,f := range flights {
			// c.Infof("abw1, %3d[%s]", i, f.Id.Designator.String())
			carrier := f.Id.Designator.IATAAirlineDesignator
			if carrier != "" {nFpC[carrier] += 1}
			
			if k := f.Id.Designator.String(); k != "" {
				if carrier != "" { nCpC[carrier] += nC[k] }
				scRow := SCRow{
					NumComplaints      : nC[k],
					NumSpeedbrakes     : nSB[k],
					WeightedComplaints : nWC[k],
					F                  : f,
				}
				rows = append(rows, scRow)
			}
		}
		
		sort.Sort(SCRowByNumComplaints(rows))		
		out := []ReportRow{}
		for _,r := range rows { out = append(out, r) }

		bestCarrier,worstCarrier := "",""
		bestScore,worstScore := 999,-1
		for carrier,nFlights := range nFpC {
			if nFlights < 3 { continue }
			cPerFlight := nCpC[carrier] / nFlights
			if cPerFlight < bestScore { bestCarrier,bestScore = carrier,cPerFlight }
			if cPerFlight > worstScore { worstCarrier,worstScore = carrier,cPerFlight }
		}

		meta["[D] Worst airline (average reports per overflight) - "+worstCarrier] = float64(worstScore)
		meta["[E] Best airline (average reports per overflight) - "+bestCarrier] = float64(bestScore)
		
		//reportToMemcache(c, memKey)
		return out, meta, nil
	}
}

// }}}
// {{{ discrepReport

func discrepReport(c appengine.Context, s,e time.Time, opt ReportOptions) ([]ReportRow, ReportMetadata, error) {
	meta := ReportMetadata{}
	fdb := fdb.FlightDB{C: c}
	maybeMemcache(&fdb,e)
	tags := []string{flightdb.KTagSERFR1}
	if flights,err := fdb.LookupTimeRangeByTags(tags,s,e); err != nil {
		return nil, nil, err

	} else {
		out := []ReportRow{}
		
		for _,f := range flights {
			// If only one source has ADS-B, flag it
			interest := false
			if  f.HasTrack("ADSB") && !f.HasTrack("FA:TA") {
				interest = true
				meta["Missing from FlightAware ADS-B"] += 1
			}
			if !f.HasTrack("ADSB") &&  f.HasTrack("FA:TA") {
				interest = true
				meta["Missing from local ADS-B"] += 1
			}

			// If both have ADS-B, look for missed violations
			if f.HasTrack("ADSB") && f.HasTrack("FA:TA") {
				if  f.HasTag("ClassB:ADSB") && !f.HasTag("ClassB:FA") {
					interest = true
					meta["Bonus violations"] += 1
				}
				if !f.HasTag("ClassB:ADSB") &&  f.HasTag("ClassB:FA") {
					interest = true
					meta["Missed violations"] += 1
				}
			}

			if !interest { continue }
			
			row := SERFR1Row{ flight2Url(f), f, false, false }
			out = append(out, row)
		}
		return out, meta, nil
	}
}

// }}}
// {{{ adsbClassbReport

type ACBRow struct {
	Seq  int
	Url  template.HTML
	F    flightdb.Flight

	HadLocalTrack  bool
	LocalViolation bool
	FAViolation    bool

	FoundBonusViolation   bool
	IncreasedViolationBy  float64
	
	FAAnalysis     geo.TPClassBAnalysis
	LocalAnalysis  geo.TPClassBAnalysis
}
func (c ACBRow)ToCSVHeaders() []string { return []string{} }
func (r ACBRow) ToCSV() []string { return []string{} }

func adsbClassbReport(c appengine.Context, s,e time.Time, opt ReportOptions) ([]ReportRow, ReportMetadata, error) {
	fdb := fdb.FlightDB{C: c}
	maybeMemcache(&fdb,e)
	tags := []string{flightdb.KTagSERFR1}
	if flights,err := fdb.LookupTimeRangeByTags(tags,s,e); err != nil {
		return nil, nil, err
	} else {
		meta := ReportMetadata{}
		rows := []ReportRow{}

		for _,f := range flights {
			row := ACBRow{F:f, Url:flight2Url(f)}

			_,cbt := f.SFOClassB("FA")
			worst := cbt.FindWorstPoint()
			row.FAViolation = (worst != nil)
			if worst != nil { row.FAAnalysis = worst.A }

			if f.HasTrack("ADSB") {
				row.HadLocalTrack = true
				_,cbt := f.SFOClassB("ADSB")
				worst := cbt.FindWorstPoint()
				row.LocalViolation = (worst != nil)
				if worst != nil { row.LocalAnalysis = worst.A }
			}

			if row.LocalViolation && !row.FAViolation {
				row.FoundBonusViolation = true
			}
			if row.LocalViolation && row.FAViolation {
				hFA    := row.FAAnalysis.BelowBy
				hLocal := row.LocalAnalysis.BelowBy
				row.IncreasedViolationBy = hLocal - hFA
			}

			if row.LocalViolation || row.FAViolation {
				rows = append(rows, row)
			}
		}
		
		return rows, meta, nil
	}
}

// }}}
// {{{ skimmerReport

type SkimRow struct {
	Url             template.HTML
	F               flightdb.Flight
	A               flightdb.SkimAnalysis
	Source          string
}
func (r SkimRow)ToCSVHeaders() []string {
	return []string{
		"Airline", "Flightnumber", "Registration", "Icao24", "Origin", "Destination",
		"StartNM", "EndNM", 
		"StartDate(PDT)", "StartTime(PDT)", "Duration(sec)",
		"DataSource"}
}
func (r SkimRow) ToCSV() []string {
	e := r.A.Events[0]
	return []string{
		r.F.Id.Designator.IATAAirlineDesignator,
		r.F.Id.Designator.String(),
		r.F.Id.Registration,
		r.F.Id.ModeS,
		r.F.Id.Origin,
		r.F.Id.Destination,
		fmt.Sprintf("%.1f", r.A.Events[0].StartNM),
		fmt.Sprintf("%.1f", r.A.Events[0].EndNM),
		date.InPdt(e.StartTP.TimestampUTC).Format("2006/01/02"),
		date.InPdt(e.StartTP.TimestampUTC).Format("15:04:05.999999999"),
		fmt.Sprintf("%.0f", e.EndTP.TimestampUTC.Sub(e.StartTP.TimestampUTC).Seconds()),
		r.Source,
	}
}

func skimmerReport(c appengine.Context, s,e time.Time, opt ReportOptions) ([]ReportRow, ReportMetadata, error) {
	meta := ReportMetadata{}
	fdb := fdb.FlightDB{C: c}
	maybeMemcache(&fdb,e)
	tags := []string{flightdb.KTagSERFR1}
	if flights,err := fdb.LookupTimeRangeByTags(tags,s,e); err != nil {
		return nil, nil, err

	} else {
		out := []ReportRow{}
		
		for _,f := range flights {			
			t := f.BestTrack()
			_,analysis := t.SkimsToSFO(opt.Skimmer_AltitudeTolerance, opt.Skimmer_MinDurationNM, 14.5, 50)

			// Output one row per event, in a kind of hacky way
			extras := fmt.Sprintf("&skim=1&alttol=%.0f&mindist=%.0f", opt.Skimmer_AltitudeTolerance,
				opt.Skimmer_MinDurationNM)
			for _,event := range analysis.Events {
				row := SkimRow{ Url:flight2Url(f)+template.HTML(extras), F:f, Source: t.LongSource(), A:analysis }
				row.A.Events = []flightdb.SkimEvent{event}				
				out = append(out, row)

			}
		}
		return out, meta, nil
	}
}

// }}}
// {{{ brixxViolationReport

type BrixxRow struct {
	Url             template.HTML
	F               flightdb.Flight
	TP             *flightdb.TrackPoint
	Source          string
}
// {{{ CSV

func (r BrixxRow)ToCSVHeaders() []string {
	return []string{
		"Airline", "Flightnumber", "Registration", "Icao24", "Origin", "Destination",
		"DataSource", "Lat", "Long", "Date(PDT)", "Time(PDT)"}
}
func (r BrixxRow) ToCSV() []string {
	return []string{
		r.F.Id.Designator.IATAAirlineDesignator,
		r.F.Id.Designator.String(),
		r.F.Id.Registration,
		r.F.Id.ModeS,
		r.F.Id.Origin,
		r.F.Id.Destination,
		r.Source,
		fmt.Sprintf("%.0f", r.TP.Latlong.Lat),
		fmt.Sprintf("%.0f", r.TP.Latlong.Long),
		date.InPdt(r.TP.TimestampUTC).Format("2006/01/02"),
		date.InPdt(r.TP.TimestampUTC).Format("15:04:05.999999999"),
	}
}

// }}}

/*

The minimum safe altitude, according to the chart, is 5500' leading up
to YADUT. However, they'll clearly descend below that as they approach
SJC, and many of them miss YADUT by a few miles. Because they fly very
close to east-west, I'd put the criteria for violation as:

- Alt < 5500'
- Longtitude < -122.023244° (so that if they are still West of it while low, they are violating)

*/

func brixxViolationReport(c appengine.Context, s,e time.Time, opt ReportOptions) ([]ReportRow, ReportMetadata, error) {
	meta := ReportMetadata{}
	fdb := fdb.FlightDB{C: c}
	maybeMemcache(&fdb,e)
	tags := []string{flightdb.KTagBRIXX}
	if flights,err := fdb.LookupTimeRangeByTags(tags,s,e); err != nil {
		return nil, nil, err

	} else {
		out := []ReportRow{}
		
		// Hmm. Really need a better way to link 'violations in this report' to dots on the map
		for _,f := range flights {			
			t := f.BestTrack()
			for i,tp := range t {
				if tp.Latlong.DistNM(sfo.KLatlongSJC) > 20 { continue }
				if tp.AltitudeFeet < 5500 && tp.Latlong.Long < sfo.KFixes["YADUT"].Long {
					row := BrixxRow{
						Url:flight2Url(f),
						F:f,
						Source: t.LongSource(),
						TP: &t[i],
					}
					out = append(out, row)
					break
				}
			}
		}
		return out, meta, nil
	}
}

// }}}

// Daily report for closest approaches to my latlong (acoustics)

// {{{ reportHandler

func reportHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if r.FormValue("date") == "" {
		var params = map[string]interface{}{
			"Yesterday": date.NowInPdt().AddDate(0,0,-1),
		}
		if err := templates.ExecuteTemplate(w, "report-form", params); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	var s,e time.Time
	switch r.FormValue("date") {
	case "today":
		s,_ = date.WindowForToday()
		e=s
	case "yesterday":
		s,_ = date.WindowForYesterday()
		e=s
	case "range":
		s = date.ArbitraryDatestring2MidnightPdt(r.FormValue("range_from"), "2006/01/02")
		e = date.ArbitraryDatestring2MidnightPdt(r.FormValue("range_to"), "2006/01/02")
		if s.After(e) { s,e = e,s }
	}
	e = e.Add(23*time.Hour + 59*time.Minute + 59*time.Second) // make sure e covers its whole day
	
	opt := ReportOptions{
		ClassB_OnePerFlight: checkbox2bool(r, "classb_oneperflight"),
	}
	if val,err := strconv.ParseFloat(r.FormValue("skimmer_altitude_tolerance"), 64); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else {
		opt.Skimmer_AltitudeTolerance = val
	}
	if val,err := strconv.ParseFloat(r.FormValue("skimmer_min_duration_nm"), 64); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else {
		opt.Skimmer_MinDurationNM = val
	}
	
	reportWriter (c,w,s,e,opt, r.FormValue("reportname"), r.FormValue("resultformat"))
}

// }}}
// {{{ reportWriter

func reportWriter (c appengine.Context, w http.ResponseWriter, s,e time.Time, opt ReportOptions, rep string, format string) {
	var rows []ReportRow
	var meta ReportMetadata
	var err error
	switch rep {
	case "classb":
		rows,meta,err = classbReport(c,s,e,opt)	
	case "adsbclassb":
		rows,meta,err = adsbClassbReport(c,s,e,opt)	
	case "discrep":
		rows,meta,err = discrepReport(c,s,e,opt)
	case "serfr1":
		rows,meta,err = serfr1Report(c,s,e,opt)
	case "brixx1":
		rows,meta,err = brixx1Report(c,s,e,opt)
	case "serfr1complaints":
		rows,meta,err = serfr1ComplaintsReport(c,s,e,opt)
	case "skimmer":
		rows,meta,err = skimmerReport(c,s,e,opt)
	case "brixxviolations":
		rows,meta,err = brixxViolationReport(c,s,e,opt)
	}
	if err != nil {	
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	
	if format == "csv" {
		filename := date.NowInPdt().AddDate(0,0,-1).Format(rep+"-20060102.csv")
		w.Header().Set("Content-Type", "application/csv")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
		csvWriter := csv.NewWriter(w)
		csvWriter.Write(rows[0].ToCSVHeaders())
		for _,r := range rows {
			if err := csvWriter.Write(r.ToCSV()); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}
		csvWriter.Flush()

	} else {
		var params = map[string]interface{}{
			"Start": s,
			"End": e,
			"Metadata": meta,
		}
		
		// Is there not a more elegant way to do this kind of thing ?
		switch rep {
		case "classb":
			out := []CBRow{}
			for _,r := range rows { out = append(out, r.(CBRow)) }
			params["Rows"] = out
		case "adsbclassb":
			out := []ACBRow{}
			for _,r := range rows { out = append(out, r.(ACBRow)) }
			params["Rows"] = out
		case "serfr1","brixx1","discrep":
			out := []SERFR1Row{}
			for _,r := range rows { out = append(out, r.(SERFR1Row)) }
			params["Rows"] = out
			rep = "serfr1"
		case "serfr1complaints":
			out := []SCRow{}
			for _,r := range rows { out = append(out, r.(SCRow)) }
			params["Rows"] = out
		case "skimmer":
			out := []SkimRow{}
			for _,r := range rows { out = append(out, r.(SkimRow)) }
			params["Rows"] = out
		case "brixxviolations":
			out := []BrixxRow{}
			for _,r := range rows { out = append(out, r.(BrixxRow)) }
			params["Rows"] = out
		}
		
		if err := templates.ExecuteTemplate(w, "report-"+rep, params); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

// }}}

// Some canned reports
// {{{ cannedDiscrepHandler

func cannedDiscrepHandler(w http.ResponseWriter, r *http.Request) {
	s,e := date.WindowForYesterday()
	e = e.Add(-1 * time.Second)

	opt := ReportOptions{}

	format := "list"
	if r.FormValue("csv") != "" { format = "csv" }

	reportWriter (appengine.NewContext(r), w, s,e, opt, "discrep", format)
}

// }}}
// {{{ cannedSerfr1Handler

func cannedSerfr1Handler(w http.ResponseWriter, r *http.Request) {
	s,e := date.WindowForYesterday()
	e = e.Add(-1 * time.Second)

	opt := ReportOptions{
		ClassB_OnePerFlight: true,
	}

	format := "list"
	if r.FormValue("csv") != "" { format = "csv" }

	reportWriter (appengine.NewContext(r), w, s,e, opt, "serfr1", format)
}

// }}}
// {{{ cannedClassBHandler

func cannedClassBHandler(w http.ResponseWriter, r *http.Request) {
	s,e := date.WindowForYesterday()
	e = e.Add(-1 * time.Second)

	opt := ReportOptions{
		ClassB_OnePerFlight: true,
	}

	format := "list"
	if r.FormValue("csv") != "" { format = "csv" }

	reportWriter (appengine.NewContext(r), w, s,e, opt, "classb", format)
}

// }}}
// {{{ cannedAdsbClassBHandler

func cannedAdsbClassBHandler(w http.ResponseWriter, r *http.Request) {
	s,e := date.WindowForYesterday()
	e = e.Add(-1 * time.Second)

	opt := ReportOptions{}
	format := "list"
	if r.FormValue("csv") != "" { format = "csv" }

	reportWriter (appengine.NewContext(r), w, s,e, opt, "adsbclassb", format)
}

// }}}
// {{{ cannedSerfr1ComplaintsHandler

func cannedSerfr1ComplaintsHandler(w http.ResponseWriter, r *http.Request) {
	s,e := date.WindowForYesterday()
	e = e.Add(-1 * time.Second)

	format := "list"
	if r.FormValue("csv") != "" { format = "csv" }

	reportWriter(appengine.NewContext(r), w, s,e, ReportOptions{}, "serfr1complaints", format)
}

// }}}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
