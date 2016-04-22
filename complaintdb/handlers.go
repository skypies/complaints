package complaintdb

import(
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"github.com/skypies/util/date"
	"github.com/skypies/util/widget"
	"github.com/skypies/complaints/complaintdb/types"
)

func init() {
	http.HandleFunc("/cdb/comp/debug", complaintDebugHandler)
	http.HandleFunc("/cdb/yesterday/debug", yesterdayDebugHandler)
}

// {{{ complaintDebugHandler

// /cdb/comp/debug?key=asdadasdasdasdasdasdsadasdsadasdasdasdasdasdas
func complaintDebugHandler(w http.ResponseWriter, r *http.Request) {
	cdb := NewComplaintDB(r)

	c, err := cdb.GetAnyComplaintByKey(r.FormValue("key"))
	if err != nil {
		http.Error(w, fmt.Sprintf("GetAnyComplaintByKey '%s': %v", r.FormValue("key"), err),
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
// {{{ yesterdayDebugHandler

// ?all=1 (will list thousands !)
// ?offset=d (how many days to go back from today; default is 1d, i.e. yesterday)


func yesterdayDebugHandler(w http.ResponseWriter, r *http.Request) {
	cdb := NewComplaintDB(r)

	offset := 1
	start,end := date.WindowForToday()
	if r.FormValue("offset") != "" {
		offset = int(widget.FormValueInt64(r, "offset"))
	}
	start,end = start.AddDate(0,0,-1*offset), end.AddDate(0,0,-1*offset)

	keys,err := cdb.GetComplaintKeysInSpan(start,end)
	// complaints,err := cdb.GetComplaintsInSpanNew(start,end)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	counts := map[string]int{}
	problems := []types.Complaint{}
	good := []types.Complaint{}
	
	//for _,c := range complaints {
	for _,k := range keys {
		c,err := cdb.GetAnyComplaintByKey(k.Encode())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		counts[fmt.Sprintf("[A] Status: %s", c.Submission.Outcome)]++
		if r.FormValue("all") != "" || c.Submission.Outcome == types.SubmissionFailed {
			problems = append(problems, *c)
		}
		if len(good) < 5 && c.Submission.Outcome == types.SubmissionAccepted {
			good = append(good, *c)
		}
		if c.Submission.Outcome == types.SubmissionAccepted {
			counts[fmt.Sprintf("[B] AttemptsToSucceed: %02d", c.Submission.Attempts)]++
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

	str += "<p>\n"
	for _,c := range good {
		str += fmt.Sprintf(" <a href=\"/cdb/comp/debug?key=%s\" target=\"_blank\">Good</a>: %s",
			c.DatastoreKey, c)
		str += "<br/>\n"
	}
	str += "</p>\n"

 		str += "<p>\n"
	for _,c := range problems {
		str += fmt.Sprintf(" <a href=\"/cdb/comp/debug?key=%s\" target=\"_blank\">Problem</a>: %s",
			c.DatastoreKey, c)
		str += "<br/>\n"
	}
	str += "</p>\n"

	str += "</body></html>\n"
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(str))

}

// }}}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
