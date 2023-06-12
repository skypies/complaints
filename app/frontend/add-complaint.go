package main

import (
	"fmt"
	"net/http"
	"time"

	"golang.org/x/net/context"

	"github.com/skypies/util/date"
	"github.com/skypies/util/gcp/ds"
	hw "github.com/skypies/util/handlerware"
	"github.com/skypies/util/widget"

	"github.com/skypies/complaints/pkg/complaintdb"
	"github.com/skypies/complaints/pkg/flightid"
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

// {{{ form2Complaint

// /add-complaint?loudness=timestamp_epoch=1441214141&flight=UA123

func form2Complaint(r *http.Request) complaintdb.Complaint {
	c := complaintdb.Complaint{
		Description: r.FormValue("content"),
		Timestamp:   time.Now(), // No point setting a timezone, it gets reset to UTC
		HeardSpeedbreaks: widget.FormValueCheckbox(r, "speedbrakes"),
		Loudness:  int(widget.FormValueInt64(r, "loudness")),
		Activity:  r.FormValue("activity"),
		Browser: complaintdb.Browser{
			UUID: r.FormValue("browser_uuid"),
			Name: r.FormValue("browser_name"),
			Version: r.FormValue("browser_version"),
			Vendor: r.FormValue("browser_vendor"),
			Platform: r.FormValue("browser_platform"),
		},
	}

	// This field is set during updates (it identifies a complaint to update)
	if r.FormValue("datastorekey") != "" {
		c.DatastoreKey = r.FormValue("datastorekey")
	}

	// These fields are set directly in CGI args, for historical population
	if r.FormValue("timestamp_epoch") != "" {
		c.Timestamp = time.Unix(widget.FormValueInt64(r,"timestamp_epoch"), 0)
	}
	if r.FormValue("flight") != "" {
		c.AircraftOverhead.FlightNumber = r.FormValue("flight")
	}

	return c
}

// }}}

// {{{ buttonHandler

// This should be deprecated somehow
func buttonHandler(w http.ResponseWriter, r *http.Request) {
	ctx := req2ctx(r)
	cdb := complaintdb.NewDB(ctx)
	resp := "OK"
	cc := r.FormValue("c")

	complaint := complaintdb.Complaint{
		Timestamp:   time.Now(), // No point setting a timezone, it gets reset to UTC
	}
	
	if err := cdb.ComplainByCallerCode(cc, &complaint); err != nil {
		resp = fmt.Sprintf("fail for %s: %s\n", cc, err)
	}
	
	w.Write([]byte(fmt.Sprintf("%s for %s\n", resp, cc)))
}

// }}}
// {{{ complaintUpdateFormHandler

func complaintUpdateFormHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	cdb := complaintdb.NewDB(ctx)
	sesh,_ := hw.GetUserSession(ctx)

	key := r.FormValue("k")

	if complaint, err := cdb.LookupKey(key, sesh.Email); err != nil {
		cdb.Errorf("updateform, getComplaint: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		cdb.Infof("Loaded complaint: %+v", complaint)
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

func addComplaintHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	cdb := complaintdb.NewDB(ctx)
	
	sesh,_ := hw.GetUserSession(ctx)
	cdb.Debugf("ac_001", "session obtained: tstamp=%s, age=%s", sesh.CreatedAt, time.Since(sesh.CreatedAt))

	complaint := form2Complaint(r)
	//complaint.Timestamp = complaint.Timestamp.AddDate(0,0,-3)
	cdb.Debugf("ac_004", "calling cdb.ComplainByEmailAddress")
	if err := cdb.ComplainByEmailAddress(sesh.Email, &complaint); err != nil {
		cdb.Errorf("cdb.Complain failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	cdb.Debugf("ac_900", "all done, about to redirect")
	http.Redirect(w, r, "/", http.StatusFound)
}

// }}}
// {{{ addHistoricalComplaintHandler

func addHistoricalComplaintHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	cdb := complaintdb.NewDB(ctx)
	sesh,_ := hw.GetUserSession(ctx)	
	complaint := form2Complaint(r)

	if err := cdb.AddHistoricalComplaintByEmailAddress(sesh.Email, &complaint); err != nil {
		cdb.Errorf("cdb.HistoricalComplain failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write([]byte(fmt.Sprintf("Added OK\n")))
}

// }}}
// {{{ updateComplaintHandler

func updateComplaintHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	cdb := complaintdb.NewDB(ctx)
	sesh,_ := hw.GetUserSession(ctx)	

	new := form2Complaint(r)
	newFlightNumber := r.FormValue("manualflightnumber")
	newTimeString := r.FormValue("manualtimestring")

	if orig, err := cdb.LookupKey(new.DatastoreKey, sesh.Email); err != nil {
		cdb.Errorf("updateform, getComplaint: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)

	} else {
		// Overlay our new values
		orig.Description = new.Description
		orig.Loudness = new.Loudness
		orig.Activity = new.Activity
		orig.HeardSpeedbreaks = new.HeardSpeedbreaks

		// If we're manually changing a flightnumber, wipe out all the other flight data
		if newFlightNumber != orig.AircraftOverhead.FlightNumber {
			orig.AircraftOverhead = flightid.Aircraft{FlightNumber: newFlightNumber}
		}

		// Compose a new timestamp, by inserting hew HH:MM:SS fragment into the old timestamp (date+nanoseconds)
		newTimestamp,err2 := date.ParseInPdt("2006.01.02 .999999999 15:04:05",
			orig.Timestamp.Format("2006.01.02 .999999999 ") + newTimeString)
		if err2 != nil {
			http.Error(w, err2.Error(), http.StatusInternalServerError)
		}
		orig.Timestamp = newTimestamp

		err := cdb.UpdateComplaint(*orig, sesh.Email)
		if err != nil {
			cdb.Errorf("cdb.UpdateComplaint failed: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

// }}}
// {{{ deleteComplaintsHandler

func deleteComplaintsHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	cdb := complaintdb.NewDB(ctx)
	sesh,_ := hw.GetUserSession(ctx)	
	
	r.ParseForm()
	// This is so brittle; need to move away from display text
	if (r.FormValue("act") == "CANCEL") {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	} else if (r.FormValue("act") == "UPDATE/EDIT LIST") {
		http.Redirect(w, r, "/edit", http.StatusFound)
		return
	}
	
	keyers := []ds.Keyer{}
	for keyStr,_ := range r.Form {
		if len(keyStr) < 50 { continue }
		keyer,_ := cdb.Provider.DecodeKey(keyStr)
		if _,err := cdb.ComplaintKeyOwnedBy(keyer, sesh.Email); err == nil {
			keyers = append(keyers, keyer)
		}
	}
	cdb.Infof("Deleting %d complaints for %s", len(keyers), sesh.Email)

	if err := cdb.DeleteAllKeys(keyers); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)	
}

// }}}
// {{{ viewComplaintHandler

// This handler isn't complete; only renders the debug blob for now.
func viewComplaintHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	cdb := complaintdb.NewDB(ctx)
	sesh,_ := hw.GetUserSession(ctx)

	key := r.FormValue("k")

	if complaint, err := cdb.LookupKey(key, sesh.Email); err != nil {
		cdb.Errorf("updateform, getComplaint: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		cdb.Infof("Loaded complaint: %+v", complaint)
		var params = map[string]interface{}{
			"HideForm": true,
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

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
