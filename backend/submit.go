package backend

import (
	"fmt"
	"net/http"
	"time"

	"google.golang.org/appengine/taskqueue"
	"golang.org/x/net/context"

	"github.com/skypies/util/date"

	"github.com/skypies/complaints/bksv"
	"github.com/skypies/complaints/complaintdb"
	"github.com/skypies/complaints/complaintdb/types"
)

func init() {
	http.HandleFunc("/backend/bksv/submit-user",    bksvSubmitUserHandler)
	http.HandleFunc("/backend/bksv/scan-yesterday", bksvScanYesterdayHandler)
}	

// {{{ bksvScanYesterdayHandler

// Examine all users. If they had any complaints, throw them in the queue.
func bksvScanYesterdayHandler(w http.ResponseWriter, r *http.Request) {
	ctx := req2ctx(r)
	cdb := complaintdb.NewDB(ctx)
	cps, err := cdb.LookupAllProfiles(cdb.NewProfileQuery())
	if err != nil {
		cdb.Errorf(" /backend/bksv/scan-yesterday: getprofiles: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	start,end := date.WindowForYesterday()
	bksv_ok := 0
	
	for _,cp := range cps {
		// if cp.CcSfo == false { continue }  // We do not care about this value.

		var complaints = []types.Complaint{}

		complaints, err = cdb.LookupAll(cdb.CQByEmail(cp.EmailAddress).ByTimespan(start,end))
		if err != nil {
			cdb.Errorf(" /backend/bksv/scan-yesterday: getbyemail(%s): %v", cp.EmailAddress, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		} 
		if len(complaints) > 0 {
			t := taskqueue.NewPOSTTask("/backend/bksv/submit-user", map[string][]string{
				"user": {cp.EmailAddress},
			})
			if _,err := taskqueue.Add(cdb.Ctx(), t, "submitreports"); err != nil {
				cdb.Errorf(" /backend/bksv/scan-yesterday: enqueue: %v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			bksv_ok++
		}
	}
	cdb.Infof("enqueued %d bksv", bksv_ok)
	w.Write([]byte(fmt.Sprintf("OK, enqueued %d", bksv_ok)))
}

// }}}
// {{{ bksvSubmitUserHandler

func bksvSubmitUserHandler(w http.ResponseWriter, r *http.Request) {
	ctx := req2ctx(r)
	cdb := complaintdb.NewDB(ctx)

	start,end := date.WindowForYesterday()
	bksv_ok,bksv_not_ok := 0,0

	email := r.FormValue("user")

	if cp,err := cdb.MustLookupProfile(email); err != nil {
		cdb.Errorf(" /bksv/submit-user(%s): getprofile: %v", email, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	
	} else if complaints,err := cdb.LookupAll(cdb.CQByEmail(email).ByTimespan(start,end)); err != nil{
		cdb.Errorf(" /bksv/submit-user(%s): getcomplaints: %v", email, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return

	} else {
		for i,complaint := range complaints {
			// Give the remote service a chance to recover before hitting the user again
			time.Sleep(time.Millisecond * 200)

			// Create a fresh context, with a tight deadline, just for this
			// one submission. (If we use the context/cdb at the top of the
			// function, then the timeout has to handle up to 200
			// submissions for a single user.)
			shortCtx, cancel := context.WithTimeout(ctx, 10 * time.Second)
			defer cancel()
			client := complaintdb.NewDB(shortCtx).HTTPClient()

			if debug,err := bksv.PostComplaint(client, *cp, complaint); err != nil {
				//cdb.Infof("pro: %v", cp)
				//cdb.Infof("comp: %#v", complaint)
				cdb.Errorf("BKSV posting error: %v", err)
				cdb.Infof("BKSV Debug\n------\n%s\n------\n", debug)
				bksv_not_ok++
			} else {
				if (i == 0) { cdb.Infof("BKSV [OK] Debug\n------\n%s\n------\n", debug) }
				bksv_ok++
			}
		}
	}

	cdb.Infof("bksv for %s, %d/%d", email, bksv_ok, bksv_not_ok)
	if (bksv_not_ok > 0) {
		cdb.Errorf("bksv for %s, %d/%d", email, bksv_ok, bksv_not_ok)
	}
	w.Write([]byte("OK"))
}

// }}}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
