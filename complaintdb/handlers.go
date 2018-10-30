package complaintdb

import(
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/appengine"

	"github.com/skypies/util/date"
	"github.com/skypies/util/widget"
	"github.com/skypies/complaints/complaintdb/types"
)

func init() {
	http.HandleFunc("/cdb/comp/debug", complaintDebugHandler)

	// This lives in the backend service now
	//http.HandleFunc("/cdb/yesterday/debug", YesterdayDebugHandler)
}

func req2ctx(r *http.Request) context.Context {
	ctx,_ := context.WithTimeout(appengine.NewContext(r), 9 * time.Minute)
	return ctx
}

// {{{ complaintDebugHandler

// /cdb/comp/debug?key=asdadasdasdasdasdasdsadasdsadasdasdasdasdasdas
func complaintDebugHandler(w http.ResponseWriter, r *http.Request) {
	ctx := req2ctx(r)
	cdb := NewDB(ctx)

	c, err := cdb.LookupKey(r.FormValue("key"), "")
	if err != nil {
		http.Error(w, fmt.Sprintf("LookupKey '%s': %v", r.FormValue("key"), err),
			http.StatusInternalServerError)
		return
	}
	
	subLog := c.Submission.Log
	idDebug := c.Debug

	c.Submission.Log = "..."
	c.Debug = "..."
	jsonText,_ := json.MarshalIndent(c, "", "  ")

	str := "======/// Complaint lookup ///=====\n\n"
	str += fmt.Sprintf("* %s\n* %s\n\n", r.FormValue("key"), c)

	if len(c.Submission.Response) > 0 {
	var jsonMap map[string]interface{}
		if err := json.Unmarshal(c.Submission.Response, &jsonMap); err != nil {
			return
		}
		indentedBytes,_ := json.MarshalIndent(jsonMap, "", "  ")
		str += "\n======/// Submission response ///======\n\n"+string(indentedBytes)+"\n--\n"
	}

	str += "\n======/// Complaint object ///======\n\n"+string(jsonText)+"\n"
	str += "\n======/// Aircraft ID debug ///======\n\n"+idDebug
	str += "\n======/// Submission Log ///======\n\n"+subLog

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("OK\n\n"+str))

}

// }}}
// {{{ SubmissionsDebugHandler

// View a day's worth of complaint submissions; defaults to yesterday
//   ?all=1 (will list every single complaint - DO NOT USE)
//   ?offset=N (how many days to go back from today; 1 == yesterday)
//   ?datestring=2018.01.20 (this day in particular)

func SubmissionsDebugHandler(w http.ResponseWriter, r *http.Request) {
	ctx := req2ctx(r)
	cdb := NewDB(ctx)

	start,end := date.WindowForToday()

	offset := 1
	if r.FormValue("offset") != "" {
		offset = int(widget.FormValueInt64(r, "offset"))
	}
	start,end = start.AddDate(0,0,-1*offset), end.AddDate(0,0,-1*offset)

	if r.FormValue("datestring") != "" {
		start,end = date.WindowForTime(date.Datestring2MidnightPdt(r.FormValue("datestring")))
	}

	keyers,err := cdb.LookupAllKeys(cdb.NewComplaintQuery().ByTimespan(start,end))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	max_good,max_problems := 5,200
	counts := map[string]int{}
	problems := []types.Complaint{}
	good := []types.Complaint{}
	retries := []types.Complaint{}

	for _,keyer := range keyers {
		c,err := cdb.LookupKey(keyer.Encode(), "")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		counts[fmt.Sprintf("[A] Status: %s", c.Submission.Outcome)]++
		if r.FormValue("all") != "" || c.Submission.WasFailure() || c.Submission.Attempts > 1 {
			if len(problems) < max_problems {
				problems = append(problems, *c)
			}
		}
		if len(good) < max_good && c.Submission.Outcome == types.SubmissionAccepted {
			good = append(good, *c)
		}
		if c.Submission.Outcome == types.SubmissionAccepted {
			counts[fmt.Sprintf("[B] AttemptsToSucceed: %02d", c.Submission.Attempts)]++
			counts[fmt.Sprintf("[C] SecondsForSuccess: %02d", int(c.D.Seconds()))]++
			
			if c.Submission.Attempts > 1 {
				retries = append(retries, *c)
			}
		}
	}

	str := "<html><body>\n"
	str += fmt.Sprintf("<pre>Start: %s\nEnd  : %s\n</pre>\n", start, end)
	str += "<table border=0>\n"

	countkeys := []string{}
	for k,_ := range counts { countkeys = append(countkeys, k) }
	sort.Strings(countkeys)
	for _,k := range countkeys {
		str += fmt.Sprintf("<tr><td><b>%s</b></td><td>%d</td></tr>\n", k, counts[k])
	}
	str += "</table>\n"

	url := "https://stop.jetnoise.net/cdb/comp/debug"
	
	str += "<p>\n"
	for _,c := range good {
		str += fmt.Sprintf(" <a href=\"%s?key=%s\" target=\"_blank\">Good</a>: %s",url,c.DatastoreKey,c)
		str += "<br/>\n"
	}
	str += "</p>\n"

	str += "<p>\n"
	for _,c := range problems {
		str += fmt.Sprintf(" <a href=\"%s?key=%s\" target=\"_blank\">Prob</a>: %s",url,c.DatastoreKey,c)
		str += "<br/>\n"
	}
	str += "</p>\n"

	str += "<p>\n"
	for _,c := range retries {
		str += fmt.Sprintf(" <a href=\"%s?key=%s\" target=\"_blank\">Retry</a>: %s",url,c.DatastoreKey,c)
		str += "<br/>\n"
	}
	str += "</p>\n"

	str += "</body></html>\n"
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(str))

}

// }}}

// {{{ touchAllProfilesHandler

func touchAllProfilesHandler(w http.ResponseWriter, r *http.Request) {
	cdb := NewDB(req2ctx(r))
	tStart := time.Now()

	profiles, err := cdb.LookupAllProfiles(cdb.NewProfileQuery())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for _,cp := range profiles {
		if err := cdb.PersistProfile(cp); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(fmt.Sprintf("OK backend! (%d profiles touched, tool %s)\n\n",
		len(profiles), time.Since(tStart))))
}

// }}}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
