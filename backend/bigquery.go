package backend

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	oldappengine "appengine"
	"google.golang.org/cloud/bigquery"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/taskqueue"

	"github.com/skypies/util/date"
	"github.com/skypies/util/gcs"
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
)

// {{{ publishAllComplaintsHandler

// /backend/publish-all-complaints?date=range&range_from=2015/08/09&range_to=2015/08/10

// Writes them all into a batch queue
func publishAllComplaintsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	str := ""

	s,e,_ := widget.FormValueDateRange(r)
	days := date.IntermediateMidnights(s.Add(-1 * time.Second),e) // decrement start, to include it
	url := "/backend/publish-complaints"

	for _,day := range days {
		dayStr := day.Format("2006/01/02")
		t := taskqueue.NewPOSTTask(fmt.Sprintf("%s?date=day&day=%s", url, dayStr),
			map[string][]string{})

		if _,err := taskqueue.Add(ctx, t, "batch"); err != nil {
			log.Errorf(ctx, "publishAllComplaintsHandler: enqueue: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		str += " * posting for " + dayStr + "\n"
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(fmt.Sprintf("OK, enqueued %d\n--\n%s", len(days), str)))
}

// }}}
// {{{ publishComplaintsHandler

// Do this one day at a time
// http://backend-dot-serfr0-1000.appspot.com/backend/publish-complaints?date=day&day=2015/09/15
// http://backend-dot-serfr0-1000.appspot.com/backend/publish-complaints?date=yesterday

// As well as writing the data into a file in Cloud Storage, it will submit a load
// request into BigQuery to load that file.

func publishComplaintsHandler(w http.ResponseWriter, r *http.Request) {
	tStart := time.Now()

	// Althought this routine is general, and will dump files for any date range, this
	// check enforces it drops just one day's worth in each file.
	if r.FormValue("date") != "day" && r.FormValue("date") != "yesterday" {
		http.Error(w, "need date widget values for a single day !", http.StatusInternalServerError)
		return
	}
	s,e,_ := widget.FormValueDateRange(r)

	ctx := appengine.NewContext(r)
	log.Infof(ctx, "Starting /backend/publish-complaints: %s", s)

	foldername := "serfr0-bigquery"
	filename := s.Format("bigquery-20060102") + e.Format("-20060102.json")

	n,err := writeAnonymizedGCSFile(r, s,e, foldername,filename)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := submitLoadJob(r, foldername, filename); err != nil {
		http.Error(w, "submitLoadJob failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(fmt.Sprintf("OK!\n%d entries written to gs://%s/%s and job sent - took %s\n",
		n, foldername, filename, time.Since(tStart))))
}

// }}}

// {{{ writeAnonymizedGCSFile

// Returns number of records written (which is zero if the file already exists)
func writeAnonymizedGCSFile(r *http.Request, s,e time.Time, foldername,filename string) (int,error) {
	ctx := appengine.NewContext(r)
	cdb := complaintdb.ComplaintDB{
		C: oldappengine.Timeout(oldappengine.NewContext(r), 599*time.Second),
		Req: r,
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

	n := 0
	// An iterator expires after 60s, no matter what; so carve up into short-lived iterators
	for _,dayWindow := range DayWindows(s,e) {
		iter := cdb.NewLongBatchingIter(cdb.QueryInSpan(dayWindow[0],dayWindow[1]))
		for {
			c,err := iter.NextWithErr();
			if err != nil {
				return 0,fmt.Errorf("iterator [%s,%s] failed at %s: %v",
					dayWindow[0],dayWindow[1], time.Now(), err)
			} else if c == nil {
				break // we're all done with this iterator
			}

			if !c.Profile.DataSharingOK() {
				continue
			}
			
			n++
			ac := complaintdb.AnonymizeComplaint(c)

			if err := encoder.Encode(ac); err != nil {
				return 0,err
			}
		}
	}

	if err := gcsHandle.Close(); err != nil {
		return 0, err
	}

	log.Infof(ctx, "GCS bigquery file '%s' successfully written", filename)

	return n,nil
}

// }}}
// {{{ submitLoadJob

// https://sourcegraph.com/github.com/GoogleCloudPlatform/gcloud-golang/-/blob/examples/bigquery/load/main.go
func submitLoadJob(r *http.Request, gcsfolder, gcsfile string) error {
	ctx := appengine.NewContext(r)

	client,err := bigquery.NewClient(ctx, bigqueryProject)
	if err != nil {
		return fmt.Errorf("Creating bigquery client: %v", err)
	}

	gcsSrc := client.NewGCSReference(fmt.Sprintf("gs://%s/%s", gcsfolder, gcsfile))
	gcsSrc.SourceFormat = bigquery.JSON

	tableDest := &bigquery.Table{
		ProjectID: bigqueryProject,
		DatasetID: bigqueryDataset,
		TableID:   bigqueryTableName,
	}
	
	job,err := client.Copy(ctx, tableDest, gcsSrc, bigquery.WriteAppend)
	if err != nil {
		return fmt.Errorf("Submission of load job: %v", err)
	}

	time.Sleep(5 * time.Second)
	
	if status, err := job.Status(ctx); err != nil {
		return fmt.Errorf("Failure determining status: %v", err)
	} else if err := status.Err(); err != nil {
		detailedErrStr := ""
		for i,innerErr := range status.Errors {
			detailedErrStr += fmt.Sprintf(" [%2d] %v\n", i, innerErr)
		}
		log.Errorf(ctx, "BiqQuery LoadJob error: %v\n--\n%s", err, detailedErrStr)
		return fmt.Errorf("Job error: %v\n--\n%s", err, detailedErrStr)
	} else {
		log.Infof(ctx, "BiqQuery LoadJob status: done=%v, state=%s, %s",
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

