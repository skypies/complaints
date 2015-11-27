package complaintdb

import (
	"fmt"
	"net/http"
	"time"
	
	"appengine"
	"appengine/datastore"
	"appengine/taskqueue"

	"github.com/skypies/complaints/complaintdb/types"
)

func init() {
	// http.HandleFunc("/batch/upgrade", upgradeHandler)
	// http.HandleFunc("/batch/upgradeuser", upgradeUserHandler)
}


// Grab all users, and enqueue them for batch processing
func upgradeHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	cdb := ComplaintDB{C:c, Memcache:false}

	var cps = []types.ComplainerProfile{}
	cps, err := cdb.GetAllProfiles()
	if err != nil {
		c.Errorf("upgradeHandler: getallprofiles: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for _,cp := range cps {
		b64 := ""
		if b64,err = cp.Base64Encode(); err != nil {
			c.Errorf("upgradeHandler: profile encode:", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		t := taskqueue.NewPOSTTask("/batch/upgradeuser", map[string][]string{
			"profile": {b64},
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

// Upgrade the set of complaints for each user.
func upgradeUserHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.Timeout(appengine.NewContext(r), 30*time.Second)
	cdb := ComplaintDB{C:c, Memcache:false}

	p := types.ComplainerProfile{}
	if err := p.Base64Decode(r.FormValue("profile")); err != nil {
		c.Errorf("upgradeUserHandler: decode failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get *all* the complaints for this person, unfiltered.
	var data = []types.Complaint{}
	q := datastore.
		NewQuery(kComplaintKind).
		Ancestor(cdb.emailToRootKey(p.EmailAddress)).
		Order("Timestamp")
	keys, err := q.GetAll(c, &data)
	if err != nil {
		c.Errorf("upgradeUserHandler/%s: GetAll failed: %v", p.EmailAddress, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	nDeleted, nUpdated := 0,0
	str := ""
	for i,complaint := range data {
		FixupComplaint(&complaint, keys[i]) // Put the key where cdb.* expect to find it

		deleteMe := i<len(data)-1 && ComplaintsAreEquivalent(data[i], data[i+1])
		noProfile := complaint.Profile.EmailAddress == ""
		//str += fmt.Sprintf(" [%03d] [DEL=%5v,PRO=%5v] %s\n", i, deleteMe, noProfile, complaint)

		if true {
			if deleteMe {
				if err := cdb.DeleteComplaints([]string{complaint.DatastoreKey}, p.EmailAddress); err != nil {
					c.Errorf("upgradeUserHandler/%s: deletecomplaints failed: %v", p.EmailAddress, err)
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				nDeleted++
				
			} else if noProfile {
				complaint.Profile = p
				if err := cdb.UpdateComplaint(complaint, p.EmailAddress); err != nil {
					c.Errorf("upgradeUserHandler/%s: updatecomplaints failed: %v", p.EmailAddress, err)
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				nUpdated++
			}
		}
	}
	
	str += fmt.Sprintf(" *** upgraded {%s} [tot=%d, del=%d, upd=%d]\n",
		p.EmailAddress, len(data), nDeleted, nUpdated)
	c.Infof(" -- Upgrade for %s --\n%s", p.EmailAddress, str)
	w.Write([]byte(fmt.Sprintf("OK, upgraded %s\n", p.EmailAddress)))
}
