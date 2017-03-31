// This file has handlers for zip-code reports.
package backend

import (
	"fmt"
	"net/http"
	"sort"

	"github.com/skypies/complaints/complaintdb"
	"github.com/skypies/util/date"
	"github.com/skypies/util/widget"
)

func init() {
	http.HandleFunc("/report/zip", zipHandler)
}

func zipHandler(w http.ResponseWriter, r *http.Request) {
	if r.FormValue("date") == "" {
		var params = map[string]interface{}{
			"Yesterday": date.NowInPdt().AddDate(0,0,-1),
		}
		if err := templates.ExecuteTemplate(w, "zip-report-form", params); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	ctx := req2ctx(r)
	cdb := complaintdb.NewDB(ctx)
	
	zip := r.FormValue("zip")
	s,e,_ := widget.FormValueDateRange(r)	

	var countsByHour [24]int
	countsByDate := map[string]int{}
	var uniquesByHour [24]map[string]int
	uniquesByDate := map[string]map[string]int{}
	uniquesAll := map[string]int{}

	q := cdb.NewComplaintQuery().ByTimespan(s,e).ByZip(zip)
	iter := cdb.NewComplaintIterator(q)
	//iter := cdb.NewIter(cdb.QueryInSpanInZip(s,e,zip))

	for iter.Iterate(ctx) {
		c := iter.Complaint()

		h := c.Timestamp.Hour()
		countsByHour[h]++
		if uniquesByHour[h] == nil { uniquesByHour[h] = map[string]int{} }
		uniquesByHour[h][c.Profile.EmailAddress]++

		d := c.Timestamp.Format("2006.01.02")
		countsByDate[d]++
		if uniquesByDate[d] == nil { uniquesByDate[d] = map[string]int{} }
		uniquesByDate[d][c.Profile.EmailAddress]++

		uniquesAll[c.Profile.EmailAddress]++
	}
	if iter.Err() != nil {
		http.Error(w, fmt.Sprintf("Zip iterator failed: %v", iter.Err()),
			http.StatusInternalServerError)
		return
	}
		
	dateKeys := []string{}
	for k,_ := range countsByDate { dateKeys = append(dateKeys, k) }
	sort.Strings(dateKeys)

	data := [][]string{}

	data = append(data, []string{"Date", "NumComplaints", "UniqueComplainers"})
	for _,k := range dateKeys {
		data = append(data, []string{
			k,
			fmt.Sprintf("%d",countsByDate[k]),
			fmt.Sprintf("%d",len(uniquesByDate[k])),
		})
	}
	data = append(data, []string{"------"})

	data = append(data, []string{"HourAcrossAllDays", "NumComplaints", "UniqueComplainers"})
	for i,v := range countsByHour {
		data = append(data, []string{
			fmt.Sprintf("%02d:00",i),
			fmt.Sprintf("%d",v),
			fmt.Sprintf("%d",len(uniquesByHour[i])),
		})
	}
	data = append(data, []string{"------"})
	data = append(data, []string{"UniqueComplainersAcrossAllDays", fmt.Sprintf("%d", len(uniquesAll))})
		
	var params = map[string]interface{}{ "Data": data }
	if err := templates.ExecuteTemplate(w, "report", params); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
