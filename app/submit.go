package complaints

import (
	"fmt"
	"net/http"
	"time"

	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/taskqueue"
	"google.golang.org/appengine/urlfetch"
	"golang.org/x/net/context"

	"github.com/skypies/util/date"

	"github.com/skypies/complaints/bksv"
	"github.com/skypies/complaints/complaintdb"
	"github.com/skypies/complaints/complaintdb/types"
)

func init() {
	http.HandleFunc("/bksv/submit-user",    bksvSubmitUserHandler)
	http.HandleFunc("/bksv/scan-yesterday", bksvScanYesterdayHandler)

	http.HandleFunc("/bksv/submit-complaint",    bksvSubmitComplaintHandler)
	http.HandleFunc("/bksv/scan-yesterday2",     bksvScanYesterdayHandler2)
}	

// {{{ bksvScanYesterdayHandler

// Examine all users. If they had any complaints, throw them in the queue.
func bksvScanYesterdayHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	cdb := complaintdb.NewComplaintDB(r)
	var cps = []types.ComplainerProfile{}
	cps, err := cdb.GetAllProfiles()
	if err != nil {
		log.Errorf(c, " /bksv/scan-yesterday: getallprofiles: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	start,end := date.WindowForYesterday()
	bksv_ok := 0
	
	for _,cp := range cps {
		// if cp.CcSfo == false { continue }  // We do not care about this value.

		var complaints = []types.Complaint{}
		complaints, err = cdb.GetComplaintsInSpanByEmailAddress(cp.EmailAddress, start, end)
		if err != nil {
			log.Errorf(c, " /bksv/scan-yesterday: getbyemail(%s): %v", cp.EmailAddress, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		} 
		if len(complaints) > 0 {
			t := taskqueue.NewPOSTTask("/bksv/submit-user", map[string][]string{
				"user": {cp.EmailAddress},
			})
			if _,err := taskqueue.Add(c, t, "submitreports"); err != nil {
				log.Errorf(c, " /bksv/scan-yesterday: enqueue: %v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			bksv_ok++
		}
	}
	log.Infof(c, "enqueued %d bksv", bksv_ok)
	w.Write([]byte(fmt.Sprintf("OK, enqueued %d", bksv_ok)))
}

// }}}
// {{{ bksvSubmitUserHandler

func bksvSubmitUserHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	cdb := complaintdb.NewComplaintDB(r)
	start,end := date.WindowForYesterday()
	bksv_ok,bksv_not_ok := 0,0

	email := r.FormValue("user")

	if cp,err := cdb.GetProfileByEmailAddress(email); err != nil {
		log.Errorf(c," /bksv/submit-user(%s): getprofile: %v", email, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	
	} else if complaints,err := cdb.GetComplaintsInSpanByEmailAddress(email, start, end); err != nil {
		log.Errorf(c, " /bksv/submit-user(%s): getcomplaints: %v", email, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return

	} else {
		for i,complaint := range complaints {
			time.Sleep(time.Millisecond * 200)
			if debug,err := bksv.PostComplaint(urlfetch.Client(c), *cp, complaint); err != nil {
				//cdb.C.Infof("pro: %v", cp)
				//cdb.C.Infof("comp: %#v", complaint)
				cdb.C.Errorf("BKSV posting error: %v", err)
				cdb.C.Infof("BKSV Debug\n------\n%s\n------\n", debug)
				bksv_not_ok++
			} else {
				if (i == 0) { cdb.C.Infof("BKSV [OK] Debug\n------\n%s\n------\n", debug) }
				bksv_ok++
			}
		}
	}

	log.Infof(c, "bksv for %s, %d/%d", email, bksv_ok, bksv_not_ok)
	if (bksv_not_ok > 0) {
		log.Errorf(c, "bksv for %s, %d/%d", email, bksv_ok, bksv_not_ok)
	}
	w.Write([]byte("OK"))
}

// }}}

// {{{ bksvScanYesterdayHandler2

var scanDryRun = false

// &today=1  (scan today, not yesterday)

// Examine all users. If they had any complaints, throw them in the queue.
func bksvScanYesterdayHandler2(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	cdb := complaintdb.NewComplaintDB(r)

	tStart := time.Now()
	start,end := date.WindowForYesterday()

	if r.FormValue("today") != "" {
		start,end = date.WindowForToday()  // For testing only
	}
	
	keys,err := cdb.GetComplaintKeysInSpan(start,end)
	if err != nil {
		log.Errorf(c, " /bksv/scan-yesterday: getcomplaintkeysinspan: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	str := ""
	
	i := 0
	for _,key := range keys {
		t := taskqueue.NewPOSTTask("/bksv/submit-complaint", map[string][]string{
			"complaintkey": {key.Encode()},
		})
		if !scanDryRun {
			if _,err := taskqueue.Add(c, t, "submitreports"); err != nil {
				log.Errorf(c, " /bksv/scan-yesterday: enqueue: %v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		
		i++
		if i<5 {
			str += fmt.Sprintf("* /bksv/submit-complaint?complaintkey=%s\n", key.Encode())
		} else {
//			break
		}
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(fmt.Sprintf("OK, enqueued %d (took %s)\n%s", i, time.Since(tStart), str)))
}

// }}}
// {{{ bksvSubmitComplaintHandler

// stop.jetnoise.net/bksv/submit-complaint?complaintkey=asdasdsdasdasdasdasdasda

func bksvSubmitComplaintHandler(w http.ResponseWriter, r *http.Request) {
	ctx,_ := context.WithTimeout(appengine.NewContext(r), 60*time.Second)
	client := urlfetch.Client(ctx)
	cdb := complaintdb.NewComplaintDB(r)

	complaint, err := cdb.GetAnyComplaintByKey(r.FormValue("complaintkey"))
	if err != nil {
		http.Error(w, fmt.Sprintf("GetAnyComplaintByKey '%s': %v", r.FormValue("complaintkey"), err),
			http.StatusInternalServerError)
		return
	}

	if complaint.Submission.Outcome == types.SubmissionAccepted {
		http.Error(w, fmt.Sprintf("Complaint already submitted ! (%s)", complaint),
			http.StatusInternalServerError)
		return
	}

	submission, postErr := bksv.PostComplaint3(client, *complaint)
	complaint.Submission = *submission // Overwrite the whole embedded object

	// Persist the complaint, even if post failed, to save the Submission details
	if updateErr := cdb.UpdateAnyComplaint(*complaint); updateErr != nil {
		http.Error(w, fmt.Sprintf("PostComplaint - final persist failed: %v (%v)", updateErr, postErr),
			http.StatusInternalServerError)
		return
	}
	
	if postErr != nil {
		http.Error(w, fmt.Sprintf("PostComplaint '%s': %v\n\n%s",
			r.FormValue("complaintkey"), postErr, complaint.Submission.Log),
			http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("OK\n--\n"+submission.Log+"\n--\n"))
}

// }}}


// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
