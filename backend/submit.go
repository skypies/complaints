package backend

import (
	"fmt"
	"net/http"
	"time"

	// "google.golang.org/ appengine/taskqueue"
	"golang.org/x/net/context"

	"github.com/skypies/util/date"
	"github.com/skypies/util/gcp/tasks"

	"github.com/skypies/complaints/bksv"
	"github.com/skypies/complaints/complaintdb"
)

var(
	LocationID = "us-central1" // This is "us-central" in appengine-land, needs a 1 for cloud tasks
	ProjectID = "serfr0-1000"
	QueueName = "batch"
)

func init() {
	http.HandleFunc("/backend/bksv/scan-yesterday",   bksvScanYesterdayHandler)
	http.HandleFunc("/backend/bksv/submit-complaint", bksvSubmitComplaintHandler)
}

// {{{ bksvScanYesterdayHandler

// Get all the keys for yesterday's complaints, and queue them for submission
func bksvScanYesterdayHandler(w http.ResponseWriter, r *http.Request) {
	ctx := req2ctx(r)
	cdb := complaintdb.NewDB(ctx)
	start,end := date.WindowForYesterday()

	keyers,err := cdb.LookupAllKeys(cdb.NewComplaintQuery().ByTimespan(start,end))
	if err != nil {
		cdb.Errorf(" /bksv/scan-yesterday: LookupAllKeys: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for _,keyer := range keyers {
		uri := "/bksv/submit-complaint"
		params := url.Values{}
		params.Set("id", keyer.Encode())
	
		if _,err := tasks.SubmitAETask(ctx, ProjectID, LocationID, QueueName, uri, params); err != nil {
			cdb.Errorf(" /backend/bksv/scan-yesterday: enqueue: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	cdb.Infof("enqueued %d bksv", len(keyers))
	w.Write([]byte(fmt.Sprintf("OK, enqueued %d", len(keyers))))
}

// }}}

// {{{ bksvScanYesterdayHandler

// Get all the keys for yesterday's complaints, and queue them for submission
func bksvScanYesterdayHandler(w http.ResponseWriter, r *http.Request) {
	ctx := req2ctx(r)
	cdb := complaintdb.NewDB(ctx)
	start,end := date.WindowForYesterday()

	keyers,err := cdb.LookupAllKeys(cdb.NewComplaintQuery().ByTimespan(start,end))
	if err != nil {
		cdb.Errorf(" /backend/bksv/scan-yesterday: LookupAllKeys: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for _,keyer := range keyers {
		t := taskqueue.NewPOSTTask("/backend/bksv/submit-complaint", map[string][]string{
			"id": {keyer.Encode()},
		})

		if _,err := taskqueue.Add(cdb.Ctx(), t, "submitreports"); err != nil {
			cdb.Errorf(" /backend/bksv/scan-yesterday: enqueue: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	cdb.Infof("enqueued %d bksv", len(keyers))
	w.Write([]byte(fmt.Sprintf("OK, enqueued %d", len(keyers))))
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
