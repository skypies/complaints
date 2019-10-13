package backend

import(
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"golang.org/x/net/context"

	"github.com/skypies/util/date"
)


func req2ctx(r *http.Request) context.Context {
	ctx,_ := context.WithTimeout(r.Context(), 9 * time.Minute)
	return ctx
}

func req2client(r *http.Request) *http.Client {
	return &http.Client{}
}

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

// Gruesome. This pseudo-widget looks at 'year' and 'month', or defaults to the previous month.
// Everything is in Pacific Time.
func FormValueMonthDefaultToPrev(r *http.Request) (month, year int, err error){
	// Default to the previous month
	oneMonthAgo := date.NowInPdt().AddDate(0,-1,0)
	month = int(oneMonthAgo.Month())
	year  = int(oneMonthAgo.Year())

	// Override with specific values, if present
	if r.FormValue("year") != "" {
		if y,err2 := strconv.ParseInt(r.FormValue("year"), 10, 64); err2 != nil {
			err = fmt.Errorf("need arg 'year' (2015)")
			return
		} else {
			year = int(y)
		}
		if m,err2 := strconv.ParseInt(r.FormValue("month"), 10, 64); err2 != nil {
			err = fmt.Errorf("need arg 'month' (1-12)")
			return
		} else {
			month = int(m)
		}
	}

	return
}
