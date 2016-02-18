package complaints

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"sort"
	"time"
	
	"appengine"

	"github.com/skypies/util/date"
	
	"github.com/skypies/complaints/complaintdb"
	"github.com/skypies/complaints/sessions"
	"github.com/skypies/util/widget"
)

func init() {
	http.HandleFunc("/download-complaints", downloadHandler)
	http.HandleFunc("/personal-report", personalReportHandler)
}

// These guys *should* be in backend, but they depend on user sessions, which segfault
// because of being in another module or something.

// {{{ keysByIntValDesc, keysByKeyAsc

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

	// Sort the vals
	sort.Sort(sort.StringSlice(keys))

	return keys
}

// }}}

// {{{ downloadHandler

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	session := sessions.Get(r)
	if session.Values["email"] == nil {
		http.Error(w, "session was empty; no cookie ? is this browser in privacy mode ?",
			http.StatusInternalServerError)
		return
	}

	c := appengine.Timeout(appengine.NewContext(r), 60*time.Second)

	cdb := complaintdb.ComplaintDB{C: c}
	cap, err := cdb.GetAllByEmailAddress(session.Values["email"].(string), true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = cap

	filename := date.NowInPdt().Format("complaints-20060102.csv")
	w.Header().Set("Content-Type", "application/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	cols := []string{
		"Date", "Time(PDT)", "Notes", "Speedbrakes", "Loudness", "Activity",
		"Flightnumber", "Origin", "Destination", "Speed(Knots)", "Altitude(Feet)",
		"Lat", "Long", "Registration", "Callsign",
		"VerticalSpeed(FeetPerMin)", "Dist2(km)", "Dist3(km)",
	}
	
	csvWriter := csv.NewWriter(w)
	csvWriter.Write(cols)

	for _,c := range cap.Complaints {
		a := c.AircraftOverhead
		speedbrakes := ""
		if c.HeardSpeedbreaks { speedbrakes = "y" }
		r := []string{
			c.Timestamp.Format("2006/01/02"),
			c.Timestamp.Format("15:04:05"),
			c.Description, speedbrakes, fmt.Sprintf("%d", c.Loudness), c.Activity,
			a.FlightNumber, a.Origin, a.Destination,
			fmt.Sprintf("%.0f",a.Speed), fmt.Sprintf("%.0f",a.Altitude),
			fmt.Sprintf("%.5f", a.Lat), fmt.Sprintf("%.5f", a.Long),
			a.Registration, a.Callsign, fmt.Sprintf("%.0f",a.VerticalSpeed),
			fmt.Sprintf("%.1f", c.Dist2KM), fmt.Sprintf("%.1f", c.Dist3KM),
		}

		if err := csvWriter.Write(r); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
	csvWriter.Flush()
}

// }}}
// {{{ personalReportHandler

func personalReportHandler(w http.ResponseWriter, r *http.Request) {
	session := sessions.Get(r)
	if session.Values["email"] == nil {
		http.Error(w, "session was empty; no cookie ?", http.StatusInternalServerError)
		return
	}
	email := session.Values["email"].(string)

	if r.FormValue("date") == "" {
		var params = map[string]interface{}{
			"Yesterday": date.NowInPdt().AddDate(0,0,-1),
		}
		if err := templates.ExecuteTemplate(w, "personal-report-form", params); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}


	start,end,_ := widget.FormValueDateRange(r)

	ctx := appengine.Timeout(appengine.NewContext(r), 60*time.Second)
	cdb := complaintdb.ComplaintDB{C: ctx}

	w.Header().Set("Content-Type", "text/plain")
	// w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", "sc.txt"))
	fmt.Fprintf(w, "Personal disturbances report for <%s>:\n From [%s]\n To   [%s]\n",
		email, start, end)

	complaintStrings := []string{}
	var countsByHour [24]int
	countsByDate := map[string]int{}
	countsByAirline := map[string]int{}
	countsByAirport := map[string]int{}

	iter := cdb.NewIter(cdb.QueryInSpanByEmailAddress(start,end,email))
	n := 0
	for {
		c := iter.Next();
		if c == nil { break }

		str := fmt.Sprintf("Time: %s, Loudness:%d, Speedbrakes:%v, Flight:%6.6s, Notes:%s",
			c.Timestamp.Format("2006.01.02 15:04:05"), c.Loudness, c.HeardSpeedbreaks,
			c.AircraftOverhead.FlightNumber, c.Description)

		n++
		complaintStrings = append(complaintStrings, str)

		countsByHour[c.Timestamp.Hour()]++
		countsByDate[c.Timestamp.Format("2006.01.02")]++
		countsByAirport[c.AircraftOverhead.Origin]++
		countsByAirport[c.AircraftOverhead.Destination]++
		if airline := c.AircraftOverhead.IATAAirlineCode(); airline != "" {
			countsByAirline[airline]++
		}
	}

	fmt.Fprintf(w, "\nTotal number of disturbance reports, over %d days:  %d\n",
		len(countsByDate), n)

	fmt.Fprintf(w, "\nDisturbance reports, counted by Airline (where known):\n")
	for _,k := range keysByIntValDesc(countsByAirline) {
		fmt.Fprintf(w, " %s: % 4d\n", k, countsByAirline[k])
	}

	fmt.Fprintf(w, "\nDisturbance reports, counted by Airport (where known):\n")
	for _,k := range keysByIntValDesc(countsByAirport) {
		if (k != "") {
			fmt.Fprintf(w, " %s: % 4d\n", k, countsByAirport[k])
		}
	}

	fmt.Fprintf(w, "\nDisturbance reports, counted by date:\n")
	for _,k := range keysByKeyAsc(countsByDate) {
		fmt.Fprintf(w, " %s: % 4d\n", k, countsByDate[k])
	}
	fmt.Fprintf(w, "\nDisturbance reports, counted by hour of day (across all dates):\n")
	for i,n := range countsByHour {
		fmt.Fprintf(w, " %02d: % 4d\n", i, n)
	}
	fmt.Fprintf(w, "\nFull dump of all disturbance reports:\n\n")
	for _,s := range complaintStrings {
		fmt.Fprint(w, s+"\n")
	}
}

// }}}


// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
