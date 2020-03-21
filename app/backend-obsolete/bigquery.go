package backend

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"cloud.google.com/go/bigquery"

	// "google.golang.org/ appengine"
	// "google.golang.org/ appengine/log"
	// "google.golang.org/ appengine/taskqueue"

	"github.com/skypies/util/date"
	"github.com/skypies/util/gcp/gcs"
	"github.com/skypies/util/gcp/tasks"
	"github.com/skypies/util/widget"

	"github.com/skypies/complaints/complaintdb"
)

func init() {
	http.HandleFunc("/backend/publish-complaints", publishComplaintsHandler)
	http.HandleFunc("/backend/publish-all-complaints", publishAllComplaintsHandler)
}

var(
	bigqueryProject = "serfr0-1000" // Should figure this out from current context, somehow
	bigqueryDataset = "public"
	bigqueryTableName = "comp"

	cloudtasksLocation = "us-central1" // This is "us-central" in appengine-land, needs a 1 for cloud tasks
)

// {{{ publishAllComplaintsHandler

// /backend/publish-all-complaints?date=range&range_from=2015/08/09&range_to=2015/08/10
//  ?skipload=1  (optional, skip loading them into bigquery

// Writes them all into a batch queue
func publishAllComplaintsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := req2ctx(r)
	str := ""

	s,e,_ := widget.FormValueDateRange(r)
	days := date.IntermediateMidnights(s.Add(-1 * time.Second),e) // decrement start, to include it
	taskurl := "/backend/publish-complaints"

	taskClient,err := tasks.GetClient(ctx)
	if err != nil {
		log.Printf("publishAllComplaintsHandler: GetClient: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	for i,day := range days {
		dayStr := day.Format("2006.01.02")

		uri := fmt.Sprintf("%s?datestring=%s", taskurl, dayStr)
		if r.FormValue("skipload") != "" {
			uri += "&skipload=" + r.FormValue("skipload")
		}

		// Give ourselves time to get all these tasks posted, and stagger them out a bit
		delay := time.Minute + time.Duration(i)*15*time.Second

		if _,err := tasks.SubmitAETask(ctx, taskClient, bigqueryProject, cloudtasksLocation, "batch", delay, uri, url.Values{}); err != nil {
			log.Printf("publishAllComplaintsHandler: enqueue: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

/*
		t := taskqueue.NewPOSTTask(thisUrl, map[string][]string{})
		// Give ourselves time to get all these tasks posted, and stagger them out a bit
		t.Delay = time.Minute + time.Duration(i)*15*time.Second
		
		if _,err := taskqueue.Add(ctx, t, "batch"); err != nil {
			log.Printf("publishAllComplaintsHandler: enqueue: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
*/

		str += " * posting for " + uri + "\n"
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(fmt.Sprintf("OK, enqueued %d\n--\n%s", len(days), str)))
}

// }}}
// {{{ publishComplaintsHandler

// http://backend-dot-serfr0-1000.appspot.com/backend/publish-complaints?datestring=yesterday
// http://backend-dot-serfr0-1000.appspot.com/backend/publish-complaints?datestring=2015.09.15

//  [&skipload=1]

// As well as writing the data into a file in Cloud Storage, it will submit a load
// request into BigQuery to load that file.

func publishComplaintsHandler(w http.ResponseWriter, r *http.Request) {
	tStart := time.Now()

	foldername := "serfr0-bigquery"

	datestring := r.FormValue("datestring")
	if datestring == "yesterday" {
		datestring = date.NowInPdt().AddDate(0,0,-1).Format("2006.01.02")
	}

	filename := "anon-"+datestring+".json"
	log.Printf("Starting /backend/publish-complaints: %s", filename)
	
	n,err := writeAnonymizedGCSFile(r, datestring, foldername,filename)
	if err != nil {
		log.Printf("/backend/publish-complaints: %s, err: %v", filename, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("%d entries written to gs://%s/%s\n", n, foldername, filename)
	str := fmt.Sprintf("%d entries written to gs://%s/%s\n", n, foldername, filename)

	if r.FormValue("skipload") == "" {
		if err := submitLoadJob(r, foldername, filename); err != nil {
			http.Error(w, "submitLoadJob failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		str += "file submitted to BigQuery for loading\n"
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(fmt.Sprintf("OK! (took %s)\n\n%s", time.Since(tStart), str)))
}

// }}}

// {{{ writeAnonymizedGCSFile

// Returns number of records written (which is zero if the file already exists)
func writeAnonymizedGCSFile(r *http.Request, datestring, foldername,filename string) (int,error) {
	ctx := req2ctx(r)
	cdb := complaintdb.NewDB(ctx)

	// Get a list of users that as of right now, have opted out of data sharing.
	optOutUsers := map[string]int{}
	q := cdb.NewProfileQuery().
		Project("EmailAddress").
		Filter("DataSharing =", -1).
		Limit(-1)
	profiles,err := cdb.LookupAllProfiles(q)
	if err != nil {
		return 0, fmt.Errorf("get optout users: %v", err)
	} else {
		for _,cp := range profiles {
			optOutUsers[cp.EmailAddress]++
		}
	}
	
	if exists,err := gcs.Exists(ctx, foldername, filename); err != nil {
		return 0,err
	} else if exists {
		return 0,nil
	}
	
	gcsHandle,err := gcs.OpenRW(ctx, foldername, filename, "application/json")
	if err != nil {
		return 0,err
	}

	encoder := json.NewEncoder(gcsHandle.IOWriter())
	
	s := date.Datestring2MidnightPdt(datestring)
	e := s.AddDate(0,0,1).Add(-1 * time.Second) // +23:59:59 (or 22:59 or 24:59 when going in/out DST)

	n := 0
	// An iterator expires after 60s, no matter what; so carve up into short-lived iterators
	for _,dayWindow := range DayWindows(s,e) {
		iter := cdb.NewComplaintIterator(cdb.NewComplaintQuery().ByTimespan(dayWindow[0],dayWindow[1]))

		for iter.Iterate(ctx) {
			c := iter.Complaint()

			// If the user is currently opted out, ignore their data
			if _,exists := optOutUsers[c.Profile.EmailAddress]; exists {
				continue
			}
			
			n++
			ac := complaintdb.AnonymizeComplaint(c)

			if err := encoder.Encode(ac); err != nil {
				return 0,err
			}
		}
		if iter.Err() != nil {
			return 0,fmt.Errorf("iterator [%s,%s] failed at %s: %v",
				dayWindow[0],dayWindow[1], time.Now(), iter.Err())
		}
	}

	if err := gcsHandle.Close(); err != nil {
		return 0, err
	}

	log.Printf("GCS bigquery file '%s' successfully written", filename)

	return n,nil
}

// }}}
// {{{ submitLoadJob

// https://sourcegraph.com/github.com/GoogleCloudPlatform/gcloud-golang/-/blob/examples/bigquery/load/main.go
func submitLoadJob(r *http.Request, gcsfolder, gcsfile string) error {
	ctx := req2ctx(r)

	client,err := bigquery.NewClient(ctx, bigqueryProject)
	if err != nil {
		return fmt.Errorf("Creating bigquery client: %v", err)
	}

	myDataset := client.Dataset(bigqueryDataset)
	destTable := myDataset.Table(bigqueryTableName)

	gcsSrc := bigquery.NewGCSReference(fmt.Sprintf("gs://%s/%s", gcsfolder, gcsfile))
	gcsSrc.SourceFormat = bigquery.JSON
	gcsSrc.AllowJaggedRows = true

	loader := destTable.LoaderFrom(gcsSrc)
	loader.CreateDisposition = bigquery.CreateNever
	job,err := loader.Run(ctx)	
	if err != nil {
		return fmt.Errorf("Submission of load job: %v", err)
	}
/*	
	tableDest := &bigquery.Table{
		ProjectID: bigqueryProject,
		DatasetID: bigqueryDataset,
		TableID:   bigqueryTableName,
	}
	
	job,err := client.Copy(ctx, tableDest, gcsSrc, bigquery.WriteAppend)
	if err != nil {
		return fmt.Errorf("Submission of load job: %v", err)
	}
*/
	time.Sleep(5 * time.Second)
	
	if status, err := job.Status(ctx); err != nil {
		return fmt.Errorf("Failure determining status: %v", err)
	} else if err := status.Err(); err != nil {
		detailedErrStr := ""
		for i,innerErr := range status.Errors {
			detailedErrStr += fmt.Sprintf(" [%2d] %v\n", i, innerErr)
		}
		log.Printf("BiqQuery LoadJob error: %v\n--\n%s", err, detailedErrStr)
		return fmt.Errorf("Job error: %v\n--\n%s", err, detailedErrStr)
	} else {
		log.Printf("BiqQuery LoadJob status: done=%v, state=%s, %s",
			status.Done(), status.State, status)
	}
	
	return nil
}

// }}}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}

