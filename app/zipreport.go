// This file has handlers for zip-code reports.
package complaints

import (
	"fmt"
	"net/http"
	"sort"

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
	s,e,_ := FormValueDateRange(r)	

	var countsByHour [24]int
	countsByDate := map[string]int{}
	var uniquesByHour [24]map[string]int
	uniquesByDate := map[string]map[string]int{}
	uniquesAll := map[string]int{}

	// ??
	// for iter := cdb.NewIter(QueryInSpanInZip(s,e,zip)); !iter.EOF; c := iter.Next() {
	iter := cdb.NewIter(cdb.QueryInSpanInZip(s,e,zip))
	for {
		c := iter.Next();
		if c == nil { break }

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
	data = append(data, []string{"------"})
	data = append(data, []string{"All uniques", fmt.Sprintf("%d", len(uniquesAll))})
		
	var params = map[string]interface{}{ "Data": data }
	if err := templates.ExecuteTemplate(w, "report", params); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
