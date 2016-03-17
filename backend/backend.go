package backend

import (
	"errors"
	"html/template"
	"net/http"
	"regexp"
	"sort"
	"time"

	"github.com/skypies/util/date"
)

func init() {
	http.HandleFunc("/", noopHandler)
	http.HandleFunc("/_ah/start", noopHandler)
	http.HandleFunc("/_ah/stop", noopHandler)
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
