package complaintdb

import(
	"fmt"
	"sort"
	"time"

	"github.com/skypies/util/date"
	"github.com/skypies/util/histogram"
)


// Yay, sorting things is so easy in go
func keysByIntValDesc(m map[string]int) []string {
	// Invert the map
	inv := map[int][]string{}
	for k,v := range m { inv[v] = append(inv[v], k) }

	// List the unique vals
	vals := []int{}
	for k,_ := range inv { vals = append(vals, k) }

	// Sort the vals
	sort.Sort(sort.Reverse(sort.IntSlice(vals)))

	// Now list the keys corresponding to each val
	keys := []string{}
	for _,val := range vals {
		for _,key := range inv[val] {
			keys = append(keys, key)
		}
	}

	return keys
}

func keysByKeyAsc(m map[string]int) []string {
	// List the unique vals
	keys := []string{}
	for k,_ := range m { keys = append(keys, k) }
	sort.Strings(keys)
	return keys
}

func keysByKeyAscNested(m map[string]map[string]int) []string {
	// List the unique vals
	keys := []string{}
	for k,_ := range m { keys = append(keys, k) }
	sort.Strings(keys)
	return keys
}

// {{{ SummaryReport

func (cdb *ComplaintDB)SummaryReport(start,end time.Time, countByUser bool, zipFilter map[string]int) (string,error) {
	str := ""
	str += fmt.Sprintf("(t=%s)\n", time.Now())
	str += fmt.Sprintf("Summary of disturbance reports:\n From [%s]\n To   [%s]\n", start, end)

	if len(zipFilter) > 0 {
		str += fmt.Sprintf("\nOnly including reports from these ZIP codes: %v\n", zipFilter)
	}
	
	var countsByHour [24]int
	countsByDate := map[string]int{}
	countsByAirline := map[string]int{}
	countsByEquip := map[string]int{}
	countsByCity := map[string]int{}
	countsByZip := map[string]int{}
	countsByAirport := map[string]int{}

	countsByProcedure := map[string]int{}        // complaint counts, per arrival/departure procedure
	flightCountsByProcedure := map[string]int{}  // how many flights flew that procedure overall
	// proceduresByCity := map[string]map[string]int{} // For each city, breakdown by procedure
	
	uniquesAll := map[string]int{}
	uniquesPerDay := map[string]int{} // Each entry is a count for one unique user, for one day
	uniquesByDate := map[string]map[string]int{}
	uniquesByCity := map[string]map[string]int{}
	uniquesByZip := map[string]map[string]int{}

	uniquesPerDayByCity := map[string]map[string]int{} // [cityname][user:date] == daily_total

	userHistsByCity := map[string]*histogram.Histogram{}
	
	// An iterator expires after 60s, no matter what; so carve up into short-lived iterators
	n := 0
	for _,dayWindow := range date.WindowsForRange(start,end) {

		// Get condensed flight data (for :NORCAL:)
		flightsWithComplaintsButNoProcedureToday := map[string]int{}

		//cfMap,err := GetProcedureMap(r,dayWindow[0],dayWindow[1])
		//if err != nil {
		//	return str,err
		//}
		//for _,cf := range cfMap {
		//	if cf.Procedure.String() != "" { flightCountsByProcedure[cf.Procedure.String()]++ }
		//}
		//cdb.Infof("fetched %d flight procedures", len(cfMap))

		q := cdb.NewComplaintQuery().ByTimespan(dayWindow[0],dayWindow[1])
		iter := cdb.NewComplaintIterator(q)
		iter.PageSize = 1000
		cdb.Infof("running summary across %s-%s", dayWindow[0],dayWindow[1])
		
		for iter.Iterate(cdb.Ctx()) {
			c := iter.Complaint()

			// If we're filtering on ZIP codes, do it here (datastore Filters can't handle OR)
			if len(zipFilter) > 0 {
				if _,exists := zipFilter[c.Profile.GetStructuredAddress().Zip]; !exists {
					continue
				}
			}
			
			n++
			d := c.Timestamp.Format("2006.01.02")

			uniquesAll[c.Profile.EmailAddress]++
			uniquesPerDay[c.Profile.EmailAddress + ":" + d]++
			countsByHour[c.Timestamp.Hour()]++
			countsByDate[d]++
			if uniquesByDate[d] == nil { uniquesByDate[d] = map[string]int{} }
			uniquesByDate[d][c.Profile.EmailAddress]++

			if airline := c.AircraftOverhead.IATAAirlineCode(); airline != "" {
				countsByAirline[airline]++
				//dayCallsigns[c.AircraftOverhead.Callsign]++

				//if cf,exists := cfMap[c.AircraftOverhead.FlightNumber]; exists && cf.Procedure.String()!=""{
				//	countsByProcedure[cf.Procedure.String()]++
				//} else {
				//	countsByProcedure["procedure unknown"]++
				//	flightsWithComplaintsButNoProcedureToday[c.AircraftOverhead.FlightNumber]++
				//}

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

			if zip := c.Profile.GetStructuredAddress().Zip; zip != "" {
				countsByZip[zip]++
				if uniquesByZip[zip] == nil { uniquesByZip[zip] = map[string]int{} }
				uniquesByZip[zip][c.Profile.EmailAddress]++
			}

			if city := c.Profile.GetStructuredAddress().City; city != "" {
				countsByCity[city]++

				if uniquesByCity[city] == nil { uniquesByCity[city] = map[string]int{} }
				uniquesByCity[city][c.Profile.EmailAddress]++

				if uniquesPerDayByCity[city] == nil { uniquesPerDayByCity[city] = map[string]int{} }
				uniquesPerDayByCity[city][c.Profile.EmailAddress + ":" + d]++

//				if proceduresByCity[city] == nil { proceduresByCity[city] = map[string]int{} }
//				if flightnumber := c.AircraftOverhead.FlightNumber; flightnumber != "" {
//					if cf,exists := cfMap[flightnumber]; exists && cf.Procedure.String()!=""{
//						proceduresByCity[city][cf.Procedure.Name]++
//					} else {
//						proceduresByCity[city]["proc?"]++
//					}
//				} else {
//					proceduresByCity[city]["flight?"]++
//				}
			}
			if equip := c.AircraftOverhead.EquipType; equip != "" {
				countsByEquip[equip]++
			}

		}
		if iter.Err() != nil {
			return str, fmt.Errorf("iterator [%s,%s] failed at %s: %v",
				dayWindow[0],dayWindow[1], time.Now(), iter.Err())
		}

		unknowns := len(flightsWithComplaintsButNoProcedureToday)
		flightCountsByProcedure["procedure unknown"] += unknowns
		
		//for k,_ := range dayCallsigns { fmt.Fprintf(w, "** %s\n", k) }
	}

	// Generate histogram(s)
	histByUser := histogram.Histogram{ValMax:200, NumBuckets:50}
	for _,v := range uniquesPerDay {
		histByUser.Add(histogram.ScalarVal(v))
	}

	for _,city := range keysByIntValDesc(countsByCity) {
		if userHistsByCity[city] == nil {
			userHistsByCity[city] = &histogram.Histogram{ValMax:200, NumBuckets:50}
		}
		for _,n := range uniquesPerDayByCity[city] {
			userHistsByCity[city].Add(histogram.ScalarVal(n))
		}
	}
	
	str += fmt.Sprintf("\nTotals:\n Days                : %d\n"+
		" Disturbance reports : %d\n People reporting    : %d\n",
		len(countsByDate), n, len(uniquesAll))

	str += fmt.Sprintf("\nComplaints per user, histogram (0-200):\n %s\n", histByUser)
	if false {
		str += fmt.Sprintf("\n[BETA: no more than 80%% accurate!] Disturbance reports, "+
			"counted by procedure type, breaking out vectored flights "+
			"(e.g. PROCEDURE/LAST-ON-PROCEDURE-WAYPOINT):\n")
		for _,k := range keysByKeyAsc(countsByProcedure) {
			avg := 0.0
			if flightCountsByProcedure[k] > 0 {
				avg = float64(countsByProcedure[k]) / float64(flightCountsByProcedure[k])
			}
			str += fmt.Sprintf(" %-20.20s: %6d (%5d such flights with complaints; %3.0f complaints/flight)\n",
				k, countsByProcedure[k], flightCountsByProcedure[k], avg)	
		}
	}
	
	str += fmt.Sprintf("\nDisturbance reports, counted by airport:\n")
	for _,k := range keysByKeyAsc(countsByAirport) {
		str += fmt.Sprintf(" %-20.20s: %6d\n", k, countsByAirport[k])
	}

	str += fmt.Sprintf("\nDisturbance reports, counted by City (where known):\n")
	for _,k := range keysByIntValDesc(countsByCity) {
		str += fmt.Sprintf(" %-40.40s: %5d (%4d people reporting)\n",
			k, countsByCity[k], len(uniquesByCity[k]))
	}

	str += fmt.Sprintf("\nDisturbance reports, counted by Zip (where known):\n")
	for _,k := range keysByIntValDesc(countsByZip) {
		str += fmt.Sprintf(" %-40.40s: %5d (%4d people reporting)\n",
			k, countsByZip[k], len(uniquesByZip[k]))
	}

	str += fmt.Sprintf("\nDisturbance reports, as per-user-per-day histograms, by City (where known):\n")
	for _,k := range keysByIntValDesc(countsByCity) {
		str += fmt.Sprintf(" %-40.40s: %s\n", k, userHistsByCity[k])
	}

	/*
	str += fmt.Sprintf("\nDisturbance reports, counted by City & procedure type (where known):\n")
	for _,k := range keysByIntValDesc(countsByCity) {		
		pStr := fmt.Sprintf("SERFR: %.0f%%, non-SERFR: %.0f%%, flight unknown: %.0f%%",
			100.0 * (float64(proceduresByCity[k]["SERFR2"]) / float64(countsByCity[k])),
			100.0 * (float64(proceduresByCity[k]["proc?"]) / float64(countsByCity[k])),
			100.0 * (float64(proceduresByCity[k]["flight?"]) / float64(countsByCity[k])))
		str += fmt.Sprintf(" %-40.40s: %5d (%4d people reporting) (%s)\n",
			k, countsByCity[k], len(uniquesByCity[k]), pStr)
	}
*/
	
	str += fmt.Sprintf("\nDisturbance reports, counted by date:\n")
	for _,k := range keysByKeyAsc(countsByDate) {
		str += fmt.Sprintf(" %s: %5d (%4d people reporting)\n", k, countsByDate[k], len(uniquesByDate[k]))
	}

	str += fmt.Sprintf("\nDisturbance reports, counted by aircraft equipment type (where known):\n")
	for _,k := range keysByIntValDesc(countsByEquip) {
		if countsByEquip[k] < 5 { break }
		str += fmt.Sprintf(" %-40.40s: %5d\n", k, countsByEquip[k])
	}

	str += fmt.Sprintf("\nDisturbance reports, counted by Airline (where known):\n")
	for _,k := range keysByIntValDesc(countsByAirline) {
		if countsByAirline[k] < 5 || len(k) > 2 { continue }
		str += fmt.Sprintf(" %s: %6d\n", k, countsByAirline[k])
	}

	str += fmt.Sprintf("\nDisturbance reports, counted by hour of day (across all dates):\n")
	for i,n := range countsByHour {
		str += fmt.Sprintf(" %02d: %5d\n", i, n)
	}

	if countByUser {
		str += fmt.Sprintf("\nDisturbance reports, counted by user:\n")
		for _,k := range keysByIntValDesc(uniquesAll) {
			str += fmt.Sprintf(" %-60.60s: %5d\n", k, uniquesAll[k])
		}
	}

	str += fmt.Sprintf("(t=%s)\n", time.Now())

	return str,nil
}

// }}}

/*
// {{{ ReadEncodedData

func ReadEncodedData(resp *http.Response, encoding string, data interface{}) error {
	switch encoding {
	case "gob": return gob.NewDecoder(resp.Body).Decode(data)
	default:    return json.NewDecoder(resp.Body).Decode(data)
	}
}

// }}}
// {{{ GetProcedureMap

// Call out to the flight database, and get back a condensed summary of the flights (flightnumber,
// times, waypoints) which flew to/from a NORCAL airport (SFO,SJC,OAK) for the time range (a day?)
func GetProcedureMap(r *http.Request, s,e time.Time) (map[string]fdb.CondensedFlight,error) {
	ret := map[string]fdb.CondensedFlight{}

	// This procedure map stuff is expensive, and brittle; so disable by default.
	if r.FormValue("getProcedures") == "" {
		return ret, nil
	}
	
	client := req2client(r)
	
	encoding := "gob"	
	url := fmt.Sprintf("http://fdb.serfr1.org/api/procedures?encoding=%s&tags=:NORCAL:&s=%d&e=%d",
		encoding, s.Unix(), e.Unix())

	condensedFlights := []fdb.CondensedFlight{}

	if resp,err := client.Get(url); err != nil {
		return ret,err
	} else {
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return ret,fmt.Errorf("Bad status fetching proc map for %s: %v", url, resp.Status)
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
*/

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
