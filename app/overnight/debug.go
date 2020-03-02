package main

import(
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/skypies/util/widget"
	"github.com/skypies/complaints/complaintdb"
	"github.com/skypies/complaints/complaintdb/types"
)

// {{{ SubmissionsDebugHandler

// View submission errors over a date range.
//   ?date=range&range_from=2016/01/21&range_to=2016/01/26
//  [?rejects=1] - only report on rejects
//  [?csv=1]
func SubmissionsDebugHandler(w http.ResponseWriter, r *http.Request) {
	ctx := req2ctx(r)
	cdb := complaintdb.NewDB(ctx)

	start,end,_ := widget.FormValueDateRange(r)

	q := cdb.NewComplaintQuery().ByTimespan(start,end)

	if r.FormValue("rejects") != "" {
		q = q.BySubmissionOutcome(int(types.SubmissionRejected))
	}
	
	if r.FormValue("csv") == "1" {
		submissionDebugCSV(cdb,w,q,start,end)
		return
	}
	
	keyers,err := cdb.LookupAllKeys(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	max_good,max_problems := 5,4000
	counts := map[string]int{}
	good := []types.Complaint{}
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

		if c.Submission.WasFailure() {
			if len(problems) < max_problems {
				problems = append(problems, *c)
			}
		}

		if c.Submission.Outcome == types.SubmissionAccepted {
			counts[fmt.Sprintf("[C] AttemptsToSucceed: %02d", c.Submission.Attempts)]++
			counts[fmt.Sprintf("[D] SecondsForSuccess: %02d", int(c.D.Seconds()))]++

			if len(good) < max_good{
				good = append(good, *c)
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

	url := "https://stop.jetnoise.net/overnight/submissions/debugcomp"

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

	str += "</body></html>\n"
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(str))
}

// }}}

// {{{ submissionDebugCSV

func submissionDebugCSV(cdb complaintdb.ComplaintDB, w http.ResponseWriter, q *complaintdb.CQuery, s,e time.Time) {
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
