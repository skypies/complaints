// This file has handlers for zip-code reports.
package complaints

import (
	"fmt"
	"net/http"
	"sort"
	"time"

	"appengine"

	"github.com/skypies/complaints/complaintdb"
	"github.com/skypies/date"
)

func init() {
	http.HandleFunc("/zip", zipFormHandler)
	http.HandleFunc("/zip/results", zipResultsHandler)
}

func zipFormHandler(w http.ResponseWriter, r *http.Request) {
	var params = map[string]interface{}{
		"Yesterday": date.NowInPdt().AddDate(0,0,-1),
	}
	if err := templates.ExecuteTemplate(w, "report-zip-form", params); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func zipResultsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	cdb := complaintdb.ComplaintDB{C: ctx, Memcache:false}

	zip := r.FormValue("zip")

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

	if data,err := cdb.GetComplaintsInSpanByZip(s,e,zip); err != nil {
		ctx.Errorf("GetComplaintsInSpanByZip(%s,%s,%s): %v", s,e,zip,err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return

	} else {
		var countsByHour [24]int
		countsByDate := map[string]int{}
		var uniquesByHour [24]map[string]int
		uniquesByDate := map[string]map[string]int{}

		for _,c := range data {
			h := c.Timestamp.Hour()
			countsByHour[h]++
			if uniquesByHour[h] == nil { uniquesByHour[h] = map[string]int{} }
			uniquesByHour[h][c.Profile.EmailAddress]++

			d := c.Timestamp.Format("2006.01.02")
			countsByDate[d]++
			if uniquesByDate[d] == nil { uniquesByDate[d] = map[string]int{} }
			uniquesByDate[d][c.Profile.EmailAddress]++
		}
		dateKeys := []string{}
		for k,_ := range countsByDate { dateKeys = append(dateKeys, k) }
		sort.Strings(dateKeys)

		data := [][]string{}
		for i,v := range countsByHour {
			data = append(data, []string{
				fmt.Sprintf("%d",i),
				fmt.Sprintf("%d",v),
				fmt.Sprintf("%d",len(uniquesByHour[i])),
			})
		}

		data = append(data, []string{"------"})		
		for _,k := range dateKeys {
			data = append(data, []string{
				k,
				fmt.Sprintf("%d",countsByDate[k]),
				fmt.Sprintf("%d",len(uniquesByDate[k])),
			})
		}
		
		var params = map[string]interface{}{ "Data": data }
		if err := templates.ExecuteTemplate(w, "report", params); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}
