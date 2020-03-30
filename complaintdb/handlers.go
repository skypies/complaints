package complaintdb

import(
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"golang.org/x/net/context"
	// "google.golang.org/ appengine"

	"github.com/skypies/util/date"
	"github.com/skypies/util/widget"
	"github.com/skypies/complaints/complaintdb/types"
)

func req2ctx(r *http.Request) context.Context {
	ctx,_ := context.WithTimeout(r.Context(), 9 * time.Minute)
	return ctx
}

// {{{ ComplaintDebugHandler

// /cdb/comp/debug?key=asdadasdasdasdasdasdsadasdsadasdasdasdasdasdas
func ComplaintDebugHandler(w http.ResponseWriter, r *http.Request) {
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

	str := ""
	/*
	addr,err := c.Profile.FetchStructuredAddress()
	str += fmt.Sprintf("======// Weird address nonsense //======\n\nErr: %v\nStored addr: %#v\n\nString addr: %q\n\nFetched addr: %#v", err, c.Profile.StructuredAddress, c.Profile.Address, addr)
*/
	str += "\n\n======/// Complaint lookup ///=====\n\n"
	str += fmt.Sprintf("* %s\n* %s\n\n", r.FormValue("key"), c)

	if len(c.Submission.Response) > 0 {
	var jsonMap map[string]interface{}
		if err := json.Unmarshal(c.Submission.Response, &jsonMap); err != nil {
			return
		}
		indentedBytes,_ := json.MarshalIndent(jsonMap, "", "  ")
		str += "\n======/// Submission response ///======\n\n"
		srr,txt := c.Submission.ClassifyRejection()
		str += fmt.Sprintf("\n\n== rejection: %s\n\n%s\n\n", srr, txt)
		str += string(indentedBytes)
	}

	str += "\n======/// Complaint object ///======\n\n"+string(jsonText)+"\n"

	str += "\n======/// Aircraft ID debug ///======\n\n"+idDebug
	str += "\n======/// Submission Log ///======\n\n"+subLog

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("OK\n\n"+str))

}

// }}}
// {{{ SubmissionsDebugHandler

// This handler runs in the backend app
// View a day's worth of complaint submissions; defaults to yesterday
//   ?all=1 (will list every single complaint - DO NOT USE)
//   ?offset=N (how many days to go back from today; 1 == yesterday)
//   ?datestring=2018.01.20 (this day in particular)
func SubmissionsDebugHandler(w http.ResponseWriter, r *http.Request) {
	ctx := req2ctx(r)
	cdb := NewDB(ctx)

	start,end := date.WindowForToday()

	hack := int(widget.FormValueInt64(r, "hack"))
	
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

	// Why do these never show up ? fscking stackdriver
	//fmt.Fprintf(os.Stderr, "Here is a fmt.Fprintf(stderr) ffs")
	//log.Printf("Here is a log.Printf ffs")
	//cdb.Infof("Here is an info string ffs")
	/*
	str2 := fmt.Sprintf("Pah00\n\ns: %s\ne: %s\n%d keyers\n\n", start, end, len(keyers))
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(str2))
	return
*/

	max_good,max_problems,max_retries := 5,200,200
	counts := map[string]int{}
	problems := []types.Complaint{}
	good := []types.Complaint{}
	retries := []types.Complaint{}

	n := 0
	for _,keyer := range keyers {
		if hack > 0 && n > hack {
			continue
		}

		c,err := cdb.LookupKey(keyer.Encode(), "")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		n++
		if n % 100 == 0 {
			fmt.Printf("submissions debug: read %d/%d\n", n, len(keyers))
		}

		counts[fmt.Sprintf("[A] Status: %s", c.Submission.Outcome)]++
		// if r.FormValue("all") != "" || c.Submission.WasFailure() || c.Submission.Attempts > 1 {
		if r.FormValue("all") != "" || c.Submission.WasFailure() {
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
			
			if c.Submission.Attempts > 1 && len(retries) < max_retries {
				retries = append(retries, *c)
			}
		}
	}

	str := "<html><body>\n"
	str += fmt.Sprintf("<pre>Start: %s\nEnd  : %s\n</pre>\n", start, end)
	str += "<table border=0>\n"

	sort.Slice(problems, func (i,j int) bool {
		return problems[i].Profile.EmailAddress < problems[j].Profile.EmailAddress
	})
	
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
// {{{ SubmissionsDebugHandler2

// View submission errors over a date range.
//   ?date=range&range_from=2016/01/21&range_to=2016/01/26
//  [?csv=1]
func SubmissionsDebugHandler2(w http.ResponseWriter, r *http.Request) {
	ctx := req2ctx(r)
	cdb := NewDB(ctx)

	start,end,_ := widget.FormValueDateRange(r)

	q := cdb.NewComplaintQuery().
		BySubmissionOutcome(int(types.SubmissionRejected)).
		ByTimespan(start,end)

	if r.FormValue("csv") == "1" {
		submissionDebugCSV(cdb,w,q,start,end)
		return
	}
	
	keyers,err := cdb.LookupAllKeys(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	max_problems := 4000
	counts := map[string]int{}
	problems := []types.Complaint{}

	for _,keyer := range keyers {

		c,err := cdb.LookupKey(keyer.Encode(), "")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		srr,_ := c.Submission.ClassifyRejection()
		counts[fmt.Sprintf("[A] Status: %s", c.Submission.Outcome)]++
		counts[fmt.Sprintf("[B] rejection: %s", srr)]++

		// if r.FormValue("all") != "" || c.Submission.WasFailure() || c.Submission.Attempts > 1 {
		if r.FormValue("all") != "" || c.Submission.WasFailure() {
			if len(problems) < max_problems {
				problems = append(problems, *c)
			}
		}
	}

	str := "<html><body>\n"
	str += fmt.Sprintf("<pre>Start: %s\nEnd  : %s\n</pre>\n", start, end)
	str += "<table border=0>\n"

	sort.Slice(problems, func (i,j int) bool {
		return problems[i].Profile.EmailAddress < problems[j].Profile.EmailAddress
	})
	
	countkeys := []string{}
	for k,_ := range counts { countkeys = append(countkeys, k) }
	sort.Strings(countkeys)
	for _,k := range countkeys {
		str += fmt.Sprintf("<tr><td><b>%s</b></td><td>%d</td></tr>\n", k, counts[k])
	}
	str += "</table>\n"

	url := "https://stop.jetnoise.net/tmp/cdb/comp/debug"
	
	str += "<p>\n"
	for _,c := range problems {
		str += fmt.Sprintf(" <a href=\"%s?key=%s\" target=\"_blank\">Prob</a>: %s",url,c.DatastoreKey,c)
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

// {{{ submissionDebugCSV

func submissionDebugCSV(cdb ComplaintDB, w http.ResponseWriter, q *CQuery, s,e time.Time) {
	cols := []string{
		// Column names above are incorrect, but BKSV are used to them.
		"CallerCode", "Name", "Address", "Zip", "Email",
		"HomeLat", "HomeLong", "UnixEpoch", "Date", "Time(PDT)",
		"Notes", "ActivityDisturbed", "Flightnumber", "Notes",
		"RejectionReason", "ErrorText",
		// These are additional columns, for the error report
	}

	f := func(c *types.Complaint) []string {
		srr,errtxt := c.Submission.ClassifyRejection()

		r := []string{
			c.Profile.CallerCode,
			c.Profile.FullName,
			c.Profile.Address,
			c.Profile.StructuredAddress.Zip,
			c.Profile.EmailAddress,

			fmt.Sprintf("%.4f",c.Profile.Lat),
			fmt.Sprintf("%.4f",c.Profile.Long),
			fmt.Sprintf("%d", c.Timestamp.UTC().Unix()),
			c.Timestamp.Format("2006/01/02"),
			c.Timestamp.Format("15:04:05"),

			c.Description,
			c.AircraftOverhead.FlightNumber,
			c.Activity,
			fmt.Sprintf("%v",c.Profile.CcSfo),

			srr.String(),
			errtxt,
		}
		return r
	}

	filename := s.Format("submission-errors-20060102") + e.Format("-20060102.csv")
	w.Header().Set("Content-Type", "application/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	cdb.FormattedWriteCQueryToCSV(q, w, cols, f)
}

// }}}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
