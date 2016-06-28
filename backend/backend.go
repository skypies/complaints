package backend

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"time"

	"google.golang.org/appengine"
	"golang.org/x/net/context"

	"github.com/skypies/util/date"

	"github.com/skypies/complaints/complaintdb"
)

func init() {
	http.HandleFunc("/", noopHandler)
	http.HandleFunc("/_ah/start", noopHandler)
	http.HandleFunc("/_ah/stop", noopHandler)
	http.HandleFunc("/backend/yesterday/debug", complaintdb.YesterdayDebugHandler)
}

var (
	templates = template.Must(template.New("").Funcs(template.FuncMap{
		"add": templateAdd,
		"km2feet": templateKM2Feet,
		"spacify": templateSpacifyFlightNumber,
		"dict": templateDict,
		"formatPdt": templateFormatPdt,
	}).ParseGlob("templates/*"))
)
func templateAdd(a int, b int) int { return a + b }
func templateKM2Feet(x float64) float64 { return x * 3280.84 }
func templateSpacifyFlightNumber(s string) string {
	s2 := regexp.MustCompile("^r:(.+)$").ReplaceAllString(s, "Registration:$1")
	s3 := regexp.MustCompile("^(..)(\\d\\d\\d)$").ReplaceAllString(s2, "$1 $2")
	return regexp.MustCompile("^(..)(\\d\\d)$").ReplaceAllString(s3, "$1  $2")
}
func templateDict(values ...interface{}) (map[string]interface{}, error) {
	if len(values)%2 != 0 { return nil, errors.New("invalid dict call")	}
	dict := make(map[string]interface{}, len(values)/2)
	for i := 0; i < len(values); i+=2 {
		key, ok := values[i].(string)
		if !ok { return nil, errors.New("dict keys must be strings") }
		dict[key] = values[i+1]
	}
	return dict, nil
}
func templateFormatPdt(t time.Time, format string) string {
	return date.InPdt(t).Format(format)
}

func req2ctx(r *http.Request) context.Context {
	ctx,_ := context.WithTimeout(appengine.NewContext(r), 9 * time.Minute)
	return ctx
}

func noopHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("OK, backend noop\n"))
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
