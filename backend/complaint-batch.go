package backend

import (
	"fmt"
	"net/http"
	
	"google.golang.org/appengine/taskqueue"

	"github.com/skypies/util/date"

	"github.com/skypies/complaints/complaintdb"
)

func init() {
	//http.HandleFunc("/backend/cdb-batch", upgradeHandler)
	//http.HandleFunc("/backend/cdb-batch-user", fixupthing)
	//http.HandleFunc("/backend/purge", purgeuserHandler)
}

// {{{ upgradeHandler

// Grab all users, and enqueue them for batch processing
func upgradeHandler(w http.ResponseWriter, r *http.Request) {
	ctx := req2ctx(r)
	cdb := complaintdb.NewDB(ctx)

	cps,err := cdb.LookupAllProfiles(cdb.NewProfileQuery())
	if err != nil {
		cdb.Errorf("upgradeHandler: getprofiles: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for _,cp := range cps {
		t := taskqueue.NewPOSTTask("/backend/cdb-batch-user", map[string][]string{
			"email": {cp.EmailAddress},
		})
		if _,err := taskqueue.Add(cdb.Ctx(), t, "batch"); err != nil {
			cdb.Errorf("upgradeHandler: enqueue: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	cdb.Infof("enqueued %d batch", len(cps))
	w.Write([]byte(fmt.Sprintf("OK, enqueued %d", len(cps))))
}

// }}}
// {{{ fixupthing

// Fixup the three days where email addresses got stamped upon
func fixupthing(w http.ResponseWriter, r *http.Request) {
	ctx := req2ctx(r)
	cdb := complaintdb.NewDB(ctx)

	email := r.FormValue("email")
	str := fmt.Sprintf("(lookup for %s)\n", email)

	// Add/remove seconds to ensure the specified dates are included as 'intermediate'
	for _,window := range date.DateRangeToPacificTimeWindows("2016/04/14","2016/04/16") {
		s,e := window[0],window[1]
		//str += fmt.Sprintf("--{ %s, %s }--\n", s,e)

		complaints,err := cdb.LookupAll(cdb.CQByEmail(email).ByTimespan(s,e))
	
		if err != nil {
			cdb.Errorf("fixupthing/%s: GetAll failed: %v", email, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		nOK,nBad := 0,0
		for _,comp := range complaints {
			if comp.Profile.EmailAddress == email {
				nOK++
			} else {
				nBad++

				comp.Profile.EmailAddress = email
				if err := cdb.UpdateComplaint(comp, email); err != nil {
					cdb.Errorf("fixupthing/%s: update-complaint failed: %v", email, err)
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
		}

		str += fmt.Sprintf("%s: ok: %4d, bad: %4d\n", s, nOK, nBad)
	}
	
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(fmt.Sprintf("OK, fixupthing\n%s", str)))
}

// }}}

// {{{ purgeuserHandler

// /backend/purge?email=foo@bar&forrealz=1

func purgeuserHandler(w http.ResponseWriter, r *http.Request) {
	ctx := req2ctx(r)
	cdb := complaintdb.NewDB(ctx)
	email := r.FormValue("email")

	str := fmt.Sprintf("(purgeuser for %s)\n", email)

	keyers, err := cdb.LookupAllKeys(cdb.CQByEmail(email))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	str += fmt.Sprintf("purge: %d complaints found\n", len(keyers))

	if r.FormValue("forrealz") == "1" {
		maxRm := 400
		for len(keyers)>maxRm {
			if err := cdb.DeleteAllKeys(keyers[0:maxRm-1]); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			keyers = keyers[maxRm:]
		}
		str += "all deleted :O\n"
	}
	
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(fmt.Sprintf("OK, purge\n%s", str)))
}

// }}}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
