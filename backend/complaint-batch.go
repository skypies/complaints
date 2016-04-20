package backend

import (
	"fmt"
	"net/http"
	"time"
	
	"appengine"
	"appengine/datastore"
	"appengine/taskqueue"

	"github.com/skypies/util/date"

	"github.com/skypies/complaints/complaintdb"
	"github.com/skypies/complaints/complaintdb/types"
)

func init() {
	http.HandleFunc("/backend/cdb-batch", upgradeHandler)
	//http.HandleFunc("/backend/cdb-batch-user", upgradeUserHandler)
	//http.HandleFunc("/backend/cdb-batch-user", fixupthing)
}


// {{{ upgradeHandler

// Grab all users, and enqueue them for batch processing
func upgradeHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	cdb := complaintdb.ComplaintDB{C:c, Memcache:false}

	var cps = []types.ComplainerProfile{}
	cps, err := cdb.GetAllProfiles()
	if err != nil {
		c.Errorf("upgradeHandler: getallprofiles: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for _,cp := range cps {
		t := taskqueue.NewPOSTTask("/backend/cdb-batch-user", map[string][]string{
			"email": {cp.EmailAddress},
		})
		if _,err := taskqueue.Add(c, t, "batch"); err != nil {
			c.Errorf("upgradeHandler: enqueue: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	c.Infof("enqueued %d batch", len(cps))
	w.Write([]byte(fmt.Sprintf("OK, enqueued %d", len(cps))))
}

// }}}
// {{{ upgradeUserHandler

// Upgrade the set of complaints for each user.
func upgradeUserHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.Timeout(appengine.NewContext(r), 300*time.Second)
	cdb := complaintdb.ComplaintDB{C:c, Memcache:false}

	email := r.FormValue("email")
	cp,err := cdb.GetProfileByEmailAddress(email)
	if err != nil {
		c.Errorf("upgradeUserHandler/%s: GetProfile failed: %v", email, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	// Get *all* the complaints for this person, unfiltered.
	var data = []types.Complaint{}
	q := datastore.
		NewQuery("ComplaintKind").
		Ancestor(cdb.EmailToRootKey(email)).
		Order("Timestamp")
	keys, err := q.GetAll(c, &data)
	if err != nil {
		c.Errorf("upgradeUserHandler/%s: GetAll failed: %v", email, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	str := fmt.Sprintf("** {%s} town1={%s}, town2={%s}\n",
		email, cp.StructuredAddress.City, cp.GetStructuredAddress().City)

	nGood,nBad := 0,0
	for i,complaint := range data {
		complaintdb.FixupComplaint(&complaint, keys[i]) // Put the key where cdb.* expect to find it

		/*
		str += fmt.Sprintf("BEFORE %s : {%s} {%s} {%s}\n",
			complaint.Timestamp.Format("2006.01.02"),
			cp.GetStructuredAddress().City,
			complaint.Profile.GetStructuredAddress().City,
			complaint.Profile.StructuredAddress.City)
*/		
		if complaint.Profile.GetStructuredAddress().City != cp.GetStructuredAddress().City {
/*
		str += fmt.Sprintf("BEFORE %s : {%s} {%s} {%s}\n",
			complaint.Timestamp.Format("2006.01.02"),
			cp.GetStructuredAddress().City,
			complaint.Profile.GetStructuredAddress().City,
			complaint.Profile.StructuredAddress.City)
*/
			complaint.Profile.StructuredAddress = cp.GetStructuredAddress()
/*
			str += fmt.Sprintf("AFTER  %s : {%s} {%s} {%s}\n\n",
			complaint.Timestamp.Format("2006.01.02"),
			cp.GetStructuredAddress().City,
			complaint.Profile.GetStructuredAddress().City,
			complaint.Profile.StructuredAddress.City)
*/
			if err := cdb.UpdateComplaint(complaint, email); err != nil {
				str += fmt.Sprintf("Oh, error updating: %v\n",err)
			}

			nBad++
		} else {
			nGood++
		}
	}
	
	str += fmt.Sprintf("** processed {%s} [tot=%d, good=%d, bad=%d]\n",
		email, len(data), nGood, nBad)
	c.Infof(" -- Upgrade for %s --\n%s", email, str)
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(fmt.Sprintf("OK, upgraded %s\n%s", email, str)))
}

// }}}

// {{{ fixupthing

// Fixup the three days where email addresses got stamped upon
func fixupthing(w http.ResponseWriter, r *http.Request) {
	c := appengine.Timeout(appengine.NewContext(r), 300*time.Second)
	cdb := complaintdb.ComplaintDB{C:c, Memcache:false}

	email := r.FormValue("email")
	str := fmt.Sprintf("(lookup for %s)\n", email)

	// Add/remove seconds to ensure the specified dates are included as 'intermediate'
	for _,window := range date.DateRangeToPacificTimeWindows("2016/04/14","2016/04/16") {
		s,e := window[0],window[1]
		//str += fmt.Sprintf("--{ %s, %s }--\n", s,e)

		complaints,err := cdb.GetComplaintsInSpanByEmailAddress(email,s,e)
		if err != nil {
			c.Errorf("fixupthing/%s: GetAll failed: %v", email, err)
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
					c.Errorf("fixupthing/%s: update-complaint failed: %v", email, err)
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


// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
