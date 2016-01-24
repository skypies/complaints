package complaints

import (
	"fmt"
	"net/http"
	"time"
	
	"appengine"

	"github.com/skypies/util/date"

	"github.com/skypies/complaints/complaintdb"
	"github.com/skypies/complaints/complaintdb/types"
	"github.com/skypies/complaints/fr24"
	"github.com/skypies/complaints/sessions"
)
// {{{ kActivities = []string

var (
	kActivities = []string{
		"",
		"Conversation",
		"Hearing",
		"Meal time",
		"Quality of life",
		"Outdoors",
		"Reading",
		"Radio",
		"Telephone",
		"Television",
		"Sleep",
		"Study",
		"Work at home",
		"Other",
	}
)

// }}}

func init() {
	http.HandleFunc("/button", buttonHandler)
	http.HandleFunc("/add-complaint", addComplaintHandler)
	http.HandleFunc("/add-historical-complaint", addHistoricalComplaintHandler)
	http.HandleFunc("/update-complaint", updateComplaintHandler)
	http.HandleFunc("/delete-complaints", deleteComplaintsHandler)
	http.HandleFunc("/complaint-updateform", complaintUpdateFormHandler)
}

// {{{ form2Complaint

// /add-complaint?loudness=timestamp_epoch=1441214141&flight=UA123

func form2Complaint(r *http.Request) types.Complaint {
	c := types.Complaint{
		Description: r.FormValue("content"),
		Timestamp:   time.Now(), // No point setting a timezone, it gets reset to UTC
		HeardSpeedbreaks: FormValueCheckbox(r, "speedbrakes"),
		Loudness:  int(FormValueInt64(r, "loudness")),
		Activity:  r.FormValue("activity"),
	}

	// This field is set during updates (it identifies a complaint to update)
	if r.FormValue("datastorekey") != "" {
		c.DatastoreKey = r.FormValue("datastorekey")
	}

	// These fields are set directly in CGI args, for historical population
	if r.FormValue("timestamp_epoch") != "" {
		c.Timestamp = time.Unix(FormValueInt64(r,"timestamp_epoch"), 0)
	}
	if r.FormValue("flight") != "" {
		c.AircraftOverhead.FlightNumber = r.FormValue("flight")
	}

	return c
}

// }}}

// {{{ buttonHandler

func buttonHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	cdb := complaintdb.ComplaintDB{C: c}
	resp := "OK"
	cc := r.FormValue("c")

	complaint := types.Complaint{
		Timestamp:   time.Now(), // No point setting a timezone, it gets reset to UTC
	}
	
	if err := cdb.ComplainByCallerCode(cc, &complaint); err != nil {
		resp = fmt.Sprintf("fail for %s: %s\n", cc, err)
	}
	
	w.Write([]byte(fmt.Sprintf("%s for %s\n", resp, cc)))
}

// }}}
// {{{ complaintUpdateFormHandler

func complaintUpdateFormHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	session := sessions.Get(r)
	if session.Values["email"] == nil {
		c.Errorf("session was empty; no cookie ?")
		http.Error(w, "session was empty; no cookie ? is this browser in privacy mode ?",
			http.StatusInternalServerError)
		return
	}
	email := session.Values["email"].(string)

	cdb := complaintdb.ComplaintDB{C: c}
	key := r.FormValue("k")

	if complaint, err := cdb.GetComplaintByKey(key, email); err != nil {
		c.Errorf("updateform, getComplaint: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		c.Infof("Loaded complaint: %+v", complaint)
		var params = map[string]interface{}{
			"ActivityList": kActivities,  // lives in add-complaint
			"DefaultFlightNumber": complaint.AircraftOverhead.FlightNumber,
			"DefaultTimestamp": complaint.Timestamp,
			"DefaultActivity": complaint.Activity,
			"DefaultLoudness": complaint.Loudness,
			"DefaultSpeedbrakes": complaint.HeardSpeedbreaks,
			"DefaultDescription": complaint.Description,
			"C": complaint,
		}
	
		if err := templates.ExecuteTemplate(w, "complaint-updateform", params); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}	

// }}}
// {{{ addComplaintHandler

func addComplaintHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	session := sessions.Get(r)
	if session.Values["email"] == nil {
		c.Errorf("session was empty; no cookie ?")
		http.Error(w, "session was empty; no cookie ? is this browser in privacy mode ?",
			http.StatusInternalServerError)
		return
	}
	email := session.Values["email"].(string)
	
	cdb := complaintdb.ComplaintDB{C: c}
	complaint := form2Complaint(r)
	//complaint.Timestamp = complaint.Timestamp.AddDate(0,0,-3)
	err := cdb.ComplainByEmailAddress(email, &complaint)
	if err != nil {
		c.Errorf("cdb.Complain failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

// }}}
// {{{ addHistoricalComplaintHandler

func addHistoricalComplaintHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	session := sessions.Get(r)
	if session.Values["email"] == nil {
		c.Errorf("session was empty; no cookie ?")
		http.Error(w, "session was empty; no cookie ? is this browser in privacy mode ?",
			http.StatusInternalServerError)
		return
	}
	email := session.Values["email"].(string)
	
	cdb := complaintdb.ComplaintDB{C: c}
	complaint := form2Complaint(r)

	err := cdb.AddHistoricalComplaintByEmailAddress(email, &complaint)
	if err != nil {
		c.Errorf("cdb.HistoricalComplain failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write([]byte(fmt.Sprintf("Added OK\n")))
}

// }}}
// {{{ updateComplaintHandler

func updateComplaintHandler(w http.ResponseWriter, r *http.Request) {

	c := appengine.NewContext(r)
	session := sessions.Get(r)
	if session.Values["email"] == nil {
		c.Errorf("session was empty; no cookie ?")
		http.Error(w, "session was empty; no cookie ? is this browser in privacy mode ?",
			http.StatusInternalServerError)
		return
	}
	email := session.Values["email"].(string)

	cdb := complaintdb.ComplaintDB{C: c}
	new := form2Complaint(r)
	newFlightNumber := r.FormValue("manualflightnumber")
	newTimeString := r.FormValue("manualtimestring")

	if orig, err := cdb.GetComplaintByKey(new.DatastoreKey, email); err != nil {
		c.Errorf("updateform, getComplaint: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)

	} else {
		// Overlay our new values
		orig.Description = new.Description
		orig.Loudness = new.Loudness
		orig.Activity = new.Activity
		orig.HeardSpeedbreaks = new.HeardSpeedbreaks

		// If we're manually changing a flightnumber, wipe out all the other flight data
		if newFlightNumber != orig.AircraftOverhead.FlightNumber {
			orig.AircraftOverhead = fr24.Aircraft{FlightNumber: newFlightNumber}
		}

		// Compose a new timestamp, by inserting hew HH:MM:SS fragment into the old timestamp (date+nanoseconds)
		newTimestamp,err2 := date.ParseInPdt("2006.01.02 .999999999 15:04:05",
			orig.Timestamp.Format("2006.01.02 .999999999 ") + newTimeString)
		if err2 != nil {
			http.Error(w, err2.Error(), http.StatusInternalServerError)
		}
		orig.Timestamp = newTimestamp

		err := cdb.UpdateComplaint(*orig, email)
		if err != nil {
			c.Errorf("cdb.UpdateComplaint failed: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

// }}}
// {{{ deleteComplaintsHandler

func deleteComplaintsHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	session := sessions.Get(r)
	if session.Values["email"] == nil {
		c.Errorf("session was empty; no cookie ?")
		http.Error(w, "session was empty; no cookie ? is this browser in privacy mode ?",
			http.StatusInternalServerError)
		return
	}
	email := session.Values["email"].(string)
	
	r.ParseForm()
	// This is so brittle; need to move away from display text
	if (r.FormValue("act") == "CANCEL") {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	} else if (r.FormValue("act") == "UPDATE/EDIT LIST") {
		http.Redirect(w, r, "/edit", http.StatusFound)
		return
	}
	
	keyStrings := []string{}
	for k,_ := range r.Form {
		if len(k) < 50 { continue }
		keyStrings = append(keyStrings, k)
	}
	c.Infof("Deleting %d complaints for %s", len(keyStrings), email)

	cdb := complaintdb.ComplaintDB{C: c}
	err := cdb.DeleteComplaints(keyStrings, email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)	
}

// }}}


// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
