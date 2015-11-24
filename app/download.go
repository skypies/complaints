package complaints

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"regexp"
	
	"appengine"

	"github.com/skypies/date"
	
	"github.com/skypies/complaints/complaintdb"
	"github.com/skypies/complaints/complaintdb/types"
	"github.com/skypies/complaints/sessions"
)

func init() {
	http.HandleFunc("/download-complaints", downloadHandler)
	http.HandleFunc("/backfill", backfillHandler)
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	session := sessions.Get(r)
	if session.Values["email"] == nil {
		http.Error(w, "session was empty; no cookie ? is this browser in privacy mode ?",
			http.StatusInternalServerError)
		return
	}

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
		"VerticalSpeed(FeetPerMin)",
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
		}

		if err := csvWriter.Write(r); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
	csvWriter.Flush()
}

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

func backfillHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	cdb := complaintdb.ComplaintDB{C: c}

	profiles, err := cdb.GetAllProfiles()
	if err != nil { return }

	filename := date.NowInPdt().Format("complaints-backfill.csv")
	w.Header().Set("Content-Type", "application/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	cols := []string{
		"CallerCode", "Name", "Address", "Zip", "Email", "HomeLat", "HomeLong", 
		"UnixEpoch", "Date", "Time(PDT)", "Notes", "ActivityDisturbed", "Flightnumber", "Notes",
	}
	
	csvWriter := csv.NewWriter(w)
	csvWriter.Write(cols)
	
	// Walk backwards in time, until there is no data
	ts,te := date.WindowForYesterday()  // end is the final day we count for; yesterday
	for {
		n := 0
		// c.Infof ("---------- Looking at ts=%s", ts)
		for _,p := range profiles {
			if p.CcSfo == false {
				// c.Infof ("---{ SKIP %s }---", p.EmailAddress)
				continue
			}
			// c.Infof ("---{ %s }---", p.EmailAddress)
			if complaints,err := cdb.GetComplaintsInSpanByEmailAddress(p.EmailAddress,ts,te); err!=nil {
				c.Errorf("Arse;ts=%s, err=%v", ts, err)
			} else {
				n += len(complaints)
				for _,complaint := range complaints {
					outputCSV(csvWriter, p, complaint)
				}
			}
		}
		if (n == 0) { break }
		ts = ts.AddDate(0,0,-1)
		te = te.AddDate(0,0,-1)
	}
	//c.Infof("All done!")
	csvWriter.Flush()
}
