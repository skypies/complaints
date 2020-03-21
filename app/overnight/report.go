package main

import(
	"net/http"
	
	"github.com/skypies/util/date"
	"github.com/skypies/util/widget"
	
	"github.com/skypies/complaints/complaintdb"
)

func init() {
	//http.HandleFunc("/overnight/report/summary", summaryArbitraryReportHandler)
	//http.HandleFunc("/overnight/report/users", userReportHandler)
	//http.HandleFunc("/overnight/report/community", communityReportHandler)
}

// {{{ summaryReportHandler

// stop.jetnoise.net/report/summary?date=day&day=2016/05/04&peeps=1

func summaryReportHandler(w http.ResponseWriter, r *http.Request) {
	ctx := req2ctx(r)
	cdb := complaintdb.NewDB(ctx)

	if r.FormValue("date") == "" {
		var params = map[string]interface{}{
			"Title": "Summary of disturbance reports",
			"FormUrl": "/overnight/report/summary",
			"Yesterday": date.NowInPdt().AddDate(0,0,-1),
		}
		//params["Message"] = "Please do not scrape this form. Instead, get in touch !"
		if err := templates.ExecuteTemplate(w, "date-report-form", params); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// See old code in backend/summary.go if you need to bring this back
	//if isBanned(r) {
	//	http.Error(w, "bad user", http.StatusUnauthorized)
	//	return
	//}
	
	start,end,_ := widget.FormValueDateRange(r)
	countByUser := r.FormValue("peeps") != ""
	zipFilter := map[string]int{}
	
	str,err := cdb.SummaryReport(start, end, countByUser, zipFilter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(str))	
}

// }}}

/*
// {{{ communityReportHandler

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

	ctx := req2ctx(r)
	cdb := complaintdb.NewDB(ctx)

	str := "OK\n\n"
	str += fmt.Sprintf("* start: %s\n* end  : %s\n", start, end)

	allCities := map[string]int{}

	numC := map[string]map[string]int{}             // numC["2016.01.01"]["Soquel"] = 213
	uniqU := map[string]map[string]map[string]int{} // numU["2016.01.01"]["Soquel"]["a@b.c"] = 1

	tStart := time.Now()
	
	for _,dayWindow := range DayWindows(start,end) {
		// Use a low-level project query, instead of an iterator, as it is faster
		q := cdb.NewComplaintQuery().
			Filter("Timestamp >= ", dayWindow[0]).
			Filter("Timestamp < ", dayWindow[1]).
			Project("Timestamp","Profile.EmailAddress","Profile.StructuredAddress.City")

		complaints,err := cdb.RawLookupAll(q)
		if err != nil {
			http.Error(w, fmt.Sprintf("iterator [%s,%s] failed after %s: %v",
				dayWindow[0],dayWindow[1], time.Since(tStart), err),
				http.StatusInternalServerError)
			return
		}
		str += fmt.Sprintf("* daywindow [%s,%s] found %d\n", dayWindow[0], dayWindow[1], len(complaints))

		for _,c := range complaints {
			d := c.Timestamp.Format("2006.01.02")
			city := c.Profile.StructuredAddress.City

			if numC[d] == nil { numC[d] = map[string]int{} }
			if uniqU[d] == nil { uniqU[d] = map[string]map[string]int{} }
			if uniqU[d][city] == nil { uniqU[d][city] = map[string]int{} }

			numC[d][city] += 1
			uniqU[d][city][c.Profile.EmailAddress] = 1
			allCities[city] += 1

			numC[d]["_All"] += 1
			if uniqU[d]["_All"] == nil { uniqU[d]["_All"] = map[string]int{} }
			uniqU[d]["_All"][c.Profile.EmailAddress] = 1
			allCities["_All"] += 1
		}
	}

	if false {
		str += fmt.Sprintf("\n\n* elapsed: %s\n* numCities: %d\n* numDays: %d\n",
			time.Since(tStart), len(allCities), len(numC))
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(str))	
		return
	}
	
	filename := start.Format("community-20060102") + end.Format("-20060102.csv")
	w.Header().Set("Content-Type", "application/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	cityCols := keysByKeyAsc(allCities)
	cols := []string{"Date"}
	cols = append(cols, cityCols...)
	csvWriter := csv.NewWriter(w)
	csvWriter.Write(cols)

	for _,datestr := range keysByKeyAscNested(numC) {
		row := []string{datestr}
		for _,city := range cityCols {
			n := numC[datestr][city]
			row = append(row, fmt.Sprintf("%d", n))
		}
		csvWriter.Write(row)
	}

	csvWriter.Write(cols)
	for _,datestr := range keysByKeyAscNested(numC) {
		row := []string{datestr}
		for _,city := range cityCols {
			n := len(uniqU[datestr][city])
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
	ctx := req2ctx(r)
	cdb := complaintdb.NewDB(ctx)

	profiles,err := cdb.LookupAllProfiles(cdb.NewProfileQuery())
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
*/

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
