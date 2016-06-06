
package backend

import (
	"encoding/csv"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"
	
	"appengine"
	"appengine/taskqueue"
	"appengine/urlfetch"

	"github.com/skypies/util/date"
	"github.com/skypies/util/histogram"
	"github.com/skypies/util/widget"
	
	"github.com/skypies/complaints/complaintdb"
	"github.com/skypies/complaints/complaintdb/types"

	fdb "github.com/skypies/flightdb2"
)

func init() {
	http.HandleFunc("/report/summary", summaryReportHandler)
	http.HandleFunc("/report/users", userReportHandler)
	http.HandleFunc("/report/community", communityReportHandler)
	http.HandleFunc("/report/month", monthHandler)
	http.HandleFunc("/report/debug", debugHandler)
}

// {{{ monthHandler

// http://stop.jetnoise.net/month?year=2015&month=9&day=1&num=10
// http://stop.jetnoise.net/month?year=2015&month=9  // via /task

// Where is the version of this that does GCS via batch ?
func monthHandler(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.Timeout(appengine.NewContext(r), 180*time.Second)

	year,err := strconv.ParseInt(r.FormValue("year"), 10, 64)
	if err != nil {
		http.Error(w, "need arg 'year' (2015)", http.StatusInternalServerError)
		return
	}
	month,err := strconv.ParseInt(r.FormValue("month"), 10, 64)
	if err != nil {
		http.Error(w, "need arg 'month' (1-12)", http.StatusInternalServerError)
		return
	}

	day,err := strconv.ParseInt(r.FormValue("day"), 10, 64)
	if err != nil {
		// Presume we should enqueue this for batch
		taskUrl := fmt.Sprintf("/backend/monthdump?year=%d&month=%d", year, month)
		t := taskqueue.NewPOSTTask(taskUrl, map[string][]string{
			"year": {r.FormValue("year")},
			"month": {r.FormValue("month")},
		})
		if _,err := taskqueue.Add(ctx, t, "batch"); err != nil {
			ctx.Errorf("monthHandler: enqueue: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(fmt.Sprintf("OK\nHave enqueued for batch {%s}\n", taskUrl)))
		return
	}

	num,err := strconv.ParseInt(r.FormValue("num"), 10, 64)
	if err != nil {
		http.Error(w, "need arg 'num' (31 - 'day')", http.StatusInternalServerError)
		return
	}
	now := date.NowInPdt()
	firstOfMonth := time.Date(int(year), time.Month(month), 1, 0,0,0,0, now.Location())
	s := firstOfMonth.AddDate(0,0,int(day-1))
	e := s.AddDate(0,0,int(num)).Add(-1 * time.Second)

	ctx.Infof("Yow: START : %s", s)
	ctx.Infof("Yow: END   : %s", e)

	cdb := complaintdb.ComplaintDB{C: ctx}

	filename := s.Format("complaints-20060102") + e.Format("-20060102.csv")
	w.Header().Set("Content-Type", "application/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	cols := []string{
		"CallerCode", "Name", "Address", "Zip", "Email", "HomeLat", "HomeLong", 
		"UnixEpoch", "Date", "Time(PDT)",
		"Notes", "Flightnumber", "ActivityDisturbed", "AutoSubmit",
	}
	csvWriter := csv.NewWriter(w)
	csvWriter.Write(cols)
	
	iter := cdb.NewIter(cdb.QueryInSpan(s,e))
	for {
		c,err := iter.NextWithErr();
		if err != nil {
			http.Error(w, fmt.Sprintf("Zip iterator failed: %v", err), http.StatusInternalServerError)
			return
		} else if c == nil {
			break  // We've hit EOF
		}
		
		r := []string{
			c.Profile.CallerCode, c.Profile.FullName, c.Profile.Address,
			c.Profile.StructuredAddress.Zip, c.Profile.EmailAddress,
			fmt.Sprintf("%.4f",c.Profile.Lat), fmt.Sprintf("%.4f",c.Profile.Long),
			fmt.Sprintf("%d", c.Timestamp.UTC().Unix()),
			c.Timestamp.Format("2006/01/02"),
			c.Timestamp.Format("15:04:05"),
			c.Description, c.AircraftOverhead.FlightNumber, c.Activity,
			fmt.Sprintf("%v",c.Profile.CcSfo),
		}

		//r = []string{c.Timestamp.Format("15:04:05")}

		if err := csvWriter.Write(r); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	csvWriter.Flush()
}

// }}}
// {{{ outputCSV

func outputCSV(w *csv.Writer, p types.ComplainerProfile, c types.Complaint) {
	zip := regexp.MustCompile("^.*(\\d{5}(-\\d{4})?).*$").ReplaceAllString(p.Address, "$1")

	r := []string{
		p.CallerCode, p.FullName, p.Address, zip, p.EmailAddress,
		fmt.Sprintf("%.4f",p.Lat), fmt.Sprintf("%.4f",p.Long),
		fmt.Sprintf("%d", c.Timestamp.UTC().Unix()),
		c.Timestamp.Format("2006/01/02"),
		c.Timestamp.Format("15:04:05"),
		c.Description, c.AircraftOverhead.FlightNumber, c.Activity,
	}

	if err := w.Write(r); err != nil {
		// ?
	}
}

// }}}

// {{{ DayWindows

func DayWindows(s,e time.Time) [][]time.Time {
	out := [][]time.Time{}
	s = s.Add(-1*time.Second) // Tip s into previous day, so that it counts as an 'intermediate'
	for _,tMidnight := range date.IntermediateMidnights(s,e) {
		out = append(out, []time.Time{tMidnight, tMidnight.AddDate(0,0,1).Add(-1*time.Second) })
	}
	return out
}

// }}}

// {{{ summaryReportHandler

// stop.jetnoise.net/report/summary?date=day&day=2016/05/04&peeps=1

func summaryReportHandler(w http.ResponseWriter, r *http.Request) {
	if r.FormValue("date") == "" {
		var params = map[string]interface{}{
			"Title": "Summary of disturbance reports",
			"FormUrl": "/report/summary",
			"Yesterday": date.NowInPdt().AddDate(0,0,-1),
		}
		if err := templates.ExecuteTemplate(w, "date-report-form", params); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	start,end,_ := widget.FormValueDateRange(r)

	ctx := appengine.Timeout(appengine.NewContext(r), 9000*time.Second)
	cdb := complaintdb.ComplaintDB{C: ctx}

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "(t=%s)\n", time.Now())
	fmt.Fprintf(w, "Summary of disturbance reports:\n From [%s]\n To   [%s]\n", start, end)
	
	var countsByHour [24]int
	countsByDate := map[string]int{}
	countsByAirline := map[string]int{}
	countsByEquip := map[string]int{}
	countsByCity := map[string]int{}
	countsByAirport := map[string]int{}

	countsByProcedure := map[string]int{}        // complaint counts, per arrival/departure procedure
	flightCountsByProcedure := map[string]int{}  // how many flights flew that procedure overall
	proceduresByCity := map[string]map[string]int{} // For each city, breakdown by procedure
	
	uniquesAll := map[string]int{}
	uniquesByDate := map[string]map[string]int{}
	uniquesByCity := map[string]map[string]int{}

	// An iterator expires after 60s, no matter what; so carve up into short-lived iterators
	n := 0
	for _,dayWindow := range DayWindows(start,end) {

		// Get condensed flight data (for :NORCAL:)
		flightsWithComplaintsButNoProcedureToday := map[string]int{}
		cfMap,err := GetProcedureMap(r,dayWindow[0],dayWindow[1])
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for _,cf := range cfMap {
			if cf.Procedure.String() != "" { flightCountsByProcedure[cf.Procedure.String()]++ }
		}
		
		iter := cdb.NewIter(cdb.QueryInSpan(dayWindow[0],dayWindow[1]))

		for {
			c,err := iter.NextWithErr();
			if err != nil {
				http.Error(w, fmt.Sprintf("iterator failed at %s: %v", time.Now(), err),
					http.StatusInternalServerError)
				return
			} else if c == nil {
				break // we're all done with this iterator
			}

			n++
			uniquesAll[c.Profile.EmailAddress]++
			countsByHour[c.Timestamp.Hour()]++

			d := c.Timestamp.Format("2006.01.02")
			countsByDate[d]++
			if uniquesByDate[d] == nil { uniquesByDate[d] = map[string]int{} }
			uniquesByDate[d][c.Profile.EmailAddress]++

			if airline := c.AircraftOverhead.IATAAirlineCode(); airline != "" {
				countsByAirline[airline]++
				//dayCallsigns[c.AircraftOverhead.Callsign]++

				if cf,exists := cfMap[c.AircraftOverhead.FlightNumber]; exists && cf.Procedure.String()!=""{
					countsByProcedure[cf.Procedure.String()]++
				} else {
					countsByProcedure["procedure unknown"]++
					flightsWithComplaintsButNoProcedureToday[c.AircraftOverhead.FlightNumber]++
				}
				
				whitelist := map[string]int{"SFO":1, "SJC":1, "OAK":1}
				if _,exists := whitelist[c.AircraftOverhead.Destination]; exists {
					countsByAirport[fmt.Sprintf("%s arrival", c.AircraftOverhead.Destination)]++
				} else if _,exists := whitelist[c.AircraftOverhead.Origin]; exists {
					countsByAirport[fmt.Sprintf("%s departure", c.AircraftOverhead.Origin)]++
				} else {
					countsByAirport["airport unknown"]++ // overflights, and/or empty airport fields
				}
			} else {
				countsByAirport["flight unidentified"]++
				countsByProcedure["flight unidentified"]++
			}

			if city := c.Profile.GetStructuredAddress().City; city != "" {
				countsByCity[city]++
				if uniquesByCity[city] == nil { uniquesByCity[city] = map[string]int{} }
				uniquesByCity[city][c.Profile.EmailAddress]++

				if proceduresByCity[city] == nil { proceduresByCity[city] = map[string]int{} }
				if flightnumber := c.AircraftOverhead.FlightNumber; flightnumber != "" {
					if cf,exists := cfMap[flightnumber]; exists && cf.Procedure.String()!=""{
						proceduresByCity[city][cf.Procedure.Name]++
					}
				}
			}
			if equip := c.AircraftOverhead.EquipType; equip != "" {
				countsByEquip[equip]++
			}
		}

		unknowns := len(flightsWithComplaintsButNoProcedureToday)
		flightCountsByProcedure["procedure unknown"] = unknowns
		
		//for k,_ := range dayCallsigns { fmt.Fprintf(w, "** %s\n", k) }
	}

	// Generate histogram(s)
	histByUser := histogram.Histogram{ValMax:200, NumBuckets:50}
	for _,v := range uniquesAll {
		histByUser.Add(histogram.ScalarVal(v))
	}
	
	fmt.Fprintf(w, "\nTotals:\n Days                : %d\n"+
		" Disturbance reports : %d\n People reporting    : %d\n",
		len(countsByDate), n, len(uniquesAll))

	fmt.Fprintf(w, "\nComplaints per user, histogram (0-200):\n %s\n", histByUser)

/*
	fmt.Fprintf(w, "\nDisturbance reports, counted by procedure type:\n")
	for _,k := range keysByKeyAsc(countsByProcedure) {
		avg := 0.0
		if flightCountsByProcedure[k] > 0 {
			avg = float64(countsByProcedure[k]) / float64(flightCountsByProcedure[k])
		}
		fmt.Fprintf(w, " %-20.20s: %5d (%4d such flights; %3.0f complaints/flight)\n",
			k, countsByProcedure[k], flightCountsByProcedure[k], avg)	
	}
*/

	fmt.Fprintf(w, "\nDisturbance reports, counted by airport:\n")
	for _,k := range keysByKeyAsc(countsByAirport) {
		fmt.Fprintf(w, " %-20.20s: %5d\n", k, countsByAirport[k])
	}

	fmt.Fprintf(w, "\nDisturbance reports, counted by City (where known):\n")
	for _,k := range keysByIntValDesc(countsByCity) {
		
		//nSerfr := proceduresByCity[k]["SERFR2"]
		//fmt.Fprintf(w, " %-40.40s: %5d (%4d people reporting) (%3d SERFR, %3d non-SERFR)\n",
		//	k, countsByCity[k], len(uniquesByCity[k]), nSerfr, (countsByCity[k]-nSerfr) )
		fmt.Fprintf(w, " %-40.40s: %5d (%4d people reporting)\n",
			k, countsByCity[k], len(uniquesByCity[k]))
	}
	fmt.Fprintf(w, "\nDisturbance reports, counted by date:\n")
	for _,k := range keysByKeyAsc(countsByDate) {
		fmt.Fprintf(w, " %s: %5d (%4d people reporting)\n", k, countsByDate[k], len(uniquesByDate[k]))
	}

	fmt.Fprintf(w, "\nDisturbance reports, counted by aircraft equipment type (where known):\n")
	for _,k := range keysByIntValDesc(countsByEquip) {
		if countsByEquip[k] < 5 { break }
		fmt.Fprintf(w, " %-40.40s: %5d\n", k, countsByEquip[k])
	}

	fmt.Fprintf(w, "\nDisturbance reports, counted by Airline (where known):\n")
	for _,k := range keysByIntValDesc(countsByAirline) {
		if countsByAirline[k] < 5 || len(k) > 2 { continue }
		fmt.Fprintf(w, " %s: %6d\n", k, countsByAirline[k])
	}

	fmt.Fprintf(w, "\nDisturbance reports, counted by hour of day (across all dates):\n")
	for i,n := range countsByHour {
		fmt.Fprintf(w, " %02d: %5d\n", i, n)
	}

	if r.FormValue("peeps") != "" {
		fmt.Fprintf(w, "\nDisturbance reports, counted by user:\n")
		for _,k := range keysByIntValDesc(uniquesAll) {
			fmt.Fprintf(w, " %-60.60s: %5d\n", k, uniquesAll[k])
		}
	}

	fmt.Fprintf(w, "(t=%s)\n", time.Now())
}

// }}}
// {{{ communityReportHandler

var cityCols = []string{
	"Unknown", "Aptos", "Atherton", "Bakersfield", "Ben Lomond", "Berkeley", "Boulder Creek", "Brisbane", "Capitola", "Carmel Valley",
	"Clovis", "Emerald Hills", "Felton", "La Selva Beach", "Los Altos", "Los Altos Hills", "Los Gatos", "Menlo Park", "Mountain View",
	"Oakland", "Pacifica", "Palo Alto", "Portola Valley", "Redwood City", "San Bruno", "San Francisco", "Santa Cruz", "Saratoga",
	"Scotts Valley", "Soquel", "South San Francisco", "Stanford", "Sunnyvale", "Watsonville", "Woodside",
}

// Start: Sat 2015/08/08
// End  : Fri 2006/02/12

// Final row: Sat 2016/02/13 -- Fri 2016/02/19

func communityReportHandler(w http.ResponseWriter, r *http.Request) {
	if r.FormValue("date") == "" {
		var params = map[string]interface{}{
			"Title": "Community breakdown of disturbance reports",
			"FormUrl": "/report/community",
			"Yesterday": date.NowInPdt().AddDate(0,0,-1),
		}
		if err := templates.ExecuteTemplate(w, "date-report-form", params); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	start,end,_ := widget.FormValueDateRange(r)

	ctx := appengine.Timeout(appengine.NewContext(r), 9000*time.Second)
	cdb := complaintdb.ComplaintDB{C: ctx}

	// Use most-recent city info for all the users, not what got cached per-complaint
	userCities,err := cdb.GetEmailCityMap()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	filename := start.Format("community-20060102") + end.Format("-20060102.csv")
	w.Header().Set("Content-Type", "application/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	counts := map[string]map[string]int{}  // {datestr}{city}
	users := map[string]map[string]int{}   // {datestr}{city}
	
	// An iterator expires after 60s, no matter what; so carve up into short-lived iterators
	n := 0

	currCounts := map[string]int{}
	currUsers  := map[string]map[string]int{}
	currN      := 0
	
	for _,dayWindow := range DayWindows(start,end) {
		//daycounts := map[string]int{}             // {city}
		//dayusers := map[string]map[string]int{}   // {city}{email}

		q := cdb.QueryInSpan(dayWindow[0],dayWindow[1])
		q = q.Project("Profile.StructuredAddress.City", "Profile.EmailAddress")
		iter := cdb.NewIter(q)
		for {
			c,err := iter.NextWithErr();
			if err != nil {
				http.Error(w, fmt.Sprintf("iterator failed at %s: %v", time.Now(), err),
					http.StatusInternalServerError)
				return
			} else if c == nil {
				break // we're all done with this iterator
			}
			n++

			//email,city := c.Profile.EmailAddress,c.Profile.StructuredAddress.City
			email := c.Profile.EmailAddress
			city := userCities[email]
			if city == "" { city = "Unknown" }
			
			if currUsers[city] == nil { currUsers[city] = map[string]int{} }
			currUsers[city][email]++
			currCounts[city]++			
		}
		currN++  // number of days processed since last flush.

		// End of a day; should we flush the counters ?
		flushStr := ""
		if true || r.FormValue("byweek") != "" {
			if currN == 7 {
				flushStr = dayWindow[0].Format("2006.01.02")
			}
		} else {
			flushStr = dayWindow[0].Format("2006.01.02")
		}

		if flushStr != "" {
			counts[flushStr] = currCounts
			users[flushStr] = map[string]int{}
			for city,_ := range currUsers {
				users[flushStr][city] = len(currUsers[city])
			}
			currCounts = map[string]int{}
			currUsers  = map[string]map[string]int{}
			currN      = 0
		}
	}

	cols := []string{"Date"}
	cols = append(cols, cityCols...)
	csvWriter := csv.NewWriter(w)
	csvWriter.Write(cols)

	for _,datestr := range keysByKeyAscNested(counts) {
		row := []string{datestr}
		for _,town := range cityCols {
			n := counts[datestr][town]
			row = append(row, fmt.Sprintf("%d", n))
		}
		csvWriter.Write(row)
	}

	csvWriter.Write(cols)
	for _,datestr := range keysByKeyAscNested(users) {
		row := []string{datestr}
		for _,town := range cityCols {
			n := users[datestr][town]
			row = append(row, fmt.Sprintf("%d", n))
		}
		csvWriter.Write(row)
	}

	csvWriter.Flush()
	
	//fmt.Fprintf(w, "(t=%s, n=%d)\n", time.Now(), n)
}

// }}}
// {{{ userReportHandler

func userReportHandler(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.Timeout(appengine.NewContext(r), 9000*time.Second)
	cdb := complaintdb.ComplaintDB{C: ctx}

	profiles,err := cdb.GetAllProfiles()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if true {
		nOK,nNotOK,nDefault := 0,0,0
		for _,p := range profiles {
			switch {
			case p.DataSharing<0: nNotOK++
			case p.DataSharing>0: nOK++
			case p.DataSharing==0: nDefault++
			}
		}
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "nOK=%d, nDefault=%d, nNotOk=%d\n", nOK, nDefault, nNotOK)
		return
	}
	
	filename := date.NowInPdt().Format("users-as-of-20060102.csv")
	w.Header().Set("Content-Type", "application/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	cols := []string{"EMAIL", "NAME", "CALLERCODE", "STREET", "CITY", "ZIP", "ALLINONELINE"}
	csvWriter := csv.NewWriter(w)
	csvWriter.Write(cols)

	for _,p := range profiles {
		street := p.StructuredAddress.Street
		if p.StructuredAddress.Number != "" {
			street = p.StructuredAddress.Number + " " + street
		}
		row := []string{
			p.EmailAddress,
			p.FullName,
			p.CallerCode,
			street,
			p.StructuredAddress.City,
			p.StructuredAddress.Zip,
			p.Address,
		}
		csvWriter.Write(row)
	}

	csvWriter.Flush()
}

// }}}

// {{{ ReadEncodedData

func ReadEncodedData(resp *http.Response, encoding string, data interface{}) error {
	switch encoding {
	case "gob": return gob.NewDecoder(resp.Body).Decode(data)
	default:    return json.NewDecoder(resp.Body).Decode(data)
	}
}

// }}}
// {{{ GetProcedureMap

func GetProcedureMap(r *http.Request, s,e time.Time) (map[string]fdb.CondensedFlight,error) {
	ret := map[string]fdb.CondensedFlight{}

	return ret, nil
	
	client := urlfetch.Client(appengine.Timeout(appengine.NewContext(r), 16 * time.Second))
	
	encoding := "gob"	
	url := fmt.Sprintf("http://fdb.serfr1.org/api/procedures?encoding=%s&tags=:NORCAL:&s=%d&e=%d",
		encoding, s.Unix(), e.Unix())

	condensedFlights := []fdb.CondensedFlight{}

	if resp,err := client.Get(url); err != nil {
		return ret,err
	} else {
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return ret,fmt.Errorf("Bad status for %s: %v", url, resp.Status)
		} else if err := ReadEncodedData(resp, encoding, &condensedFlights); err != nil {
			return ret,err
		}
	}

	for _,cf := range condensedFlights {
		ret[cf.BestFlightNumber] = cf
	}
	
	return ret,nil
}

// }}}

// {{{ debugHandler

func debugHandler(w http.ResponseWriter, r *http.Request) {
	s,e := date.WindowForYesterday()
	s = s.Add(-24 * time.Hour)
	e = e.Add(-24 * time.Hour)

	tStart := time.Now()
	procMap,err := GetProcedureMap(r,s,e)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	str := "OK!\n\n"
	str += fmt.Sprintf("* fetch+decode: %s\n* entries: %d\n\n", time.Since(tStart), len(procMap))
	
	//for k,v := range procMap { str += fmt.Sprintf("%-10.10s %s\n", k, v) }
	
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(str))	
}

// }}}


// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
