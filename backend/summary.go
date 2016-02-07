package backend

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"
	
	"appengine"
	"appengine/taskqueue"

	"github.com/skypies/util/date"
	"github.com/skypies/util/widget"
	
	"github.com/skypies/complaints/complaintdb"
	"github.com/skypies/complaints/complaintdb/types"
)

func init() {
	http.HandleFunc("/report/summary", summaryReportHandler)
	http.HandleFunc("/report/month", monthHandler)
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

func summaryReportHandler(w http.ResponseWriter, r *http.Request) {
	if r.FormValue("date") == "" {
		var params = map[string]interface{}{
			"Yesterday": date.NowInPdt().AddDate(0,0,-1),
		}
		if err := templates.ExecuteTemplate(w, "summary-report-form", params); err != nil {
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

	uniquesAll := map[string]int{}
	uniquesByDate := map[string]map[string]int{}
	uniquesByCity := map[string]map[string]int{}
	
	// An iterator expires after 60s, no matter what; so carve up into short-lived iterators
	n := 0
	for _,dayWindow := range DayWindows(start,end) {
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
			}

			if city := c.Profile.GetStructuredAddress().City; city != "" {
				countsByCity[city]++
				if uniquesByCity[city] == nil { uniquesByCity[city] = map[string]int{} }
				uniquesByCity[city][c.Profile.EmailAddress]++
			}
			if equip := c.AircraftOverhead.EquipType; equip != "" {
				countsByEquip[equip]++
			}
		}
	}

	fmt.Fprintf(w, "\nTotals:\n Days                : %d\n"+
		" Disturbance reports : %d\n People reporting    : %d\n",
		len(countsByDate), n, len(uniquesAll))

	fmt.Fprintf(w, "\nDisturbance reports, counted by City (where known):\n")
	for _,k := range keysByIntValDesc(countsByCity) {
		fmt.Fprintf(w, " %-40.40s: %5d (%4d people reporting)\n", k, countsByCity[k],
			len(uniquesByCity[k]))
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
	fmt.Fprintf(w, "(t=%s)\n", time.Now())
}

// }}}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
