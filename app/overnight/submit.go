package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/context"

	"github.com/skypies/util/date"
	"github.com/skypies/util/gcp/tasks"
	"github.com/skypies/util/widget"

	"github.com/skypies/complaints/bksv"
	"github.com/skypies/complaints/complaintdb"
	"github.com/skypies/complaints/complaintdb/types"
)

var(
	CascadableUrlParams = []string{"force", "rejects"}

	// Should really put these vars somewhere more sensible
	LocationID = "us-central1" // This is "us-central" in appengine-land, needs a 1 for cloud tasks
	ProjectID = "serfr0-1000"
	QueueName = "submitreports"
)

// {{{ bksvScanDateRangeHandler

// /some/url
//   &date=range&range_from=2016/01/21&range_to=2016/01/26
//  [&force=1]    force resubmits
//  [&rejects=1]  only submit complaints currently tagged as rejected

// Get all the keys for the time range, and queue them for submission.
func bksvScanDateRangeHandler(w http.ResponseWriter, r *http.Request) {
	ctx := req2ctx(r)
	cdb := complaintdb.NewDB(ctx)

	taskClient,err := tasks.GetClient(ctx)
	if err != nil {
		cdb.Errorf(" bksvScanTimeRange: GetClient: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s,e,_ := widget.FormValueDateRange(r)
	str := fmt.Sprintf("daterangehandler\n\n** s: %s\n** e: %s\n\n", s, e)

	delay := time.Second * 20 // Ensure we can enqueue all these jobs before they are exploded

	days := date.IntermediateMidnights(s.Add(-1 * time.Second),e) // decrement start, to include it
	for _,day := range days {
		dayStr := day.Format("2006/01/02")
		str += fmt.Sprintf(" * adding %s\n", dayStr)

		uri := bksvStem+"/scan-day"
		params := url.Values{}
		params.Set("day", dayStr)
		// Cascade these params down
		for _,param := range CascadableUrlParams {
			if r.FormValue(param) != "" {
				params.Set(param, r.FormValue(param))
			}
		}

		if _,err := tasks.SubmitAETask(ctx, taskClient, ProjectID, LocationID, QueueName, delay, uri, params); err != nil {
			cdb.Errorf(" bksvScanDateRangeRange: enqueue: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(fmt.Sprintf("OK scandaterange, enqueued %d day tasks\n\n%s", len(days), str)))
}

// }}}
// {{{ bksvScanDayHandler

// /some/url?day=2020/01/12
//  [&force=1] force resubmits

// Get all the keys for the time range, and queue them for submission.
func bksvScanDayHandler(w http.ResponseWriter, r *http.Request) {
	ctx := req2ctx(r)

	// default to yesterday
	start,end := date.WindowForYesterday()

	if r.FormValue("day") != "" {
		day := date.ArbitraryDatestring2MidnightPdt(r.FormValue("day"), "2006/01/02")
		start,end = date.WindowForTime(day)
	}

	end = end.Add(-1 * time.Second)

	bksvScanTimeRange(ctx, w, r, start,end)
}

// }}}

// {{{ bksvScanTimeRange

// Get all the keys for the time range, and queue them for submission. Will generate
// the http response (error or OK)
func bksvScanTimeRange(ctx context.Context, w http.ResponseWriter, r *http.Request, start,end time.Time) {
	cdb := complaintdb.NewDB(ctx)

	q := cdb.NewComplaintQuery().ByTimespan(start,end)
	if r.FormValue("rejects") != "" {
		q = q.BySubmissionOutcome(int(types.SubmissionRejected))
	}

	keyers,err := cdb.LookupAllKeys(q)
	if err != nil {
		cdb.Errorf(" bksvScanTimeRange: LookupAllKeys: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	cdb.Infof(" bksvScanTimeRange: read %d keys", len(keyers))
	
	// The backend is racey, if requests for the same user are sent too
	// close together. It's not a huge problem, as it will succeed on
	// retry. Ideally, we'd explicitly sort the complaints to spread out
	// each user, but all we have are keys, so we don't know which user
	// each one is. The results seem to be loosely ordered by user, So
	// do a quick (repeatable for debugging !!) shuffle.
	rand.Seed(0)
	shuffler := func(i, j int) { keyers[i],keyers[j] = keyers[j],keyers[i] }
	rand.Shuffle(len(keyers), shuffler)
	rand.Shuffle(len(keyers), shuffler)
	rand.Shuffle(len(keyers), shuffler)
	cdb.Infof(" bksvScanTimeRange: shuffling done")
	
	// Give ourselves time to finish submitting, before the deluge; we want this submission
	// loop to have the backend to itself.
	baseDelay := time.Minute * 10

	taskClient,err := tasks.GetClient(ctx)
	if err != nil {
		cdb.Errorf(" bksvScanTimeRange: GetClient: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	i := 0
	for _,keyer := range keyers {
		uri := bksvStem+"/submit-complaint"
		params := url.Values{}
		params.Set("id", keyer.Encode())
		// Cascade these params down
		for _,param := range CascadableUrlParams {
			if r.FormValue(param) != "" {
				params.Set(param, r.FormValue(param))
			}
		}

		delay := baseDelay + time.Millisecond * 250 * time.Duration(i)

		if _,err := tasks.SubmitAETask(ctx, taskClient, ProjectID, LocationID, QueueName, delay, uri, params); err != nil {
			cdb.Errorf(" bksvScanTimeRange: enqueue: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		/*		if i > 10 {
			break
		}*/

		if i % 100 == 0 {
			cdb.Infof(" bksvScanTimeRange: submitted %d", i)
		}

		i++
	}

	cdb.Infof("enqueued %d bksv", len(keyers))
	w.Write([]byte(fmt.Sprintf("OK, enqueued %d\nstart: %s\nend  : %s\n", len(keyers), start, end)))
}

// }}}
// {{{ bksvSubmitComplaintHandler

// ? id=<datastorekey>
func bksvSubmitComplaintHandler(w http.ResponseWriter, r *http.Request) {
	// NOTE - short timeout on the context. No point waiting 9 minutes.
	ctx, cancel := context.WithTimeout(req2ctx(r), 20 * time.Second)
	defer cancel()

	cdb := complaintdb.NewDB(ctx)
	client := cdb.HTTPClient()

	complaint, err := cdb.LookupKey(r.FormValue("id"), "")
	if err != nil {
		cdb.Errorf("BKSV bad lookup for id %s: %v", r.FormValue("id"), err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// FIXME - remove this hack at some point.
	// Hack; we manually fixed up a ton of profiles on 2020.01.28.
	// Complaints that were stored before this time may have crappy
	// address data, so re-pull the profile and copy it over.
	goodAddressesDate := time.Date(2020, time.February, 01, 0, 0, 0, 0, time.UTC)
	if complaint.Timestamp.Before(goodAddressesDate) {
		if cp,err := cdb.MustLookupProfile(complaint.Profile.EmailAddress); err == nil {
			complaint.Profile = *cp
		}
	}

	// Don't POST if this complaint has already been accepted.
	if r.FormValue("force") == "" && complaint.Submission.Outcome == types.SubmissionAccepted {
		return
	}
	
	sub,postErr := bksv.PostComplaint(client, *complaint)
	if postErr != nil {
		cdb.Errorf("BKSV posting error: %v", postErr)
		cdb.Infof("BKSV Debug\n------\n%s\n------\n", sub)
	}

	//cdb.Infof("BKSV [OK] Debug\n------\n%s\n------\n", sub.Log)

	// Store the submission outcome, even if the post failed
	complaint.Submission = *sub
	if err := cdb.PersistComplaint(*complaint); err != nil {
		cdb.Errorf("BKSV, peristing outcome failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if postErr != nil {
		http.Error(w, postErr.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Write([]byte("OK"))
}

// }}}


// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
