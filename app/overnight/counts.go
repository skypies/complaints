package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/skypies/util/date"
	"github.com/skypies/util/widget"

	"github.com/skypies/complaints/complaintdb"
)

// {{{ countsHandler

// Note - all counts < 2018.07.23 undercount by 5-10%; they omit the user who opted out
// of overnight emails. (07.23 is the first day using the new logic.)

// ?date=yesterday
// ?date=day&day=2006/01/02
func countsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := req2ctx(r)
	cdb := complaintdb.NewDB(ctx)

	s,e,err := widget.FormValueDateRange(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// The widget returns a day as [s,e) but the DB lookup expects
	// to see it as [s,e],so adjust e
	e = e.Add(time.Second)
	
	if n_complaints, n_users, err := cdb.CountComplaintsAndUniqueUsersIn(s,e); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return

	} else {	
		cdb.AddDailyCount(complaintdb.DailyCount{
			Datestring: date.Time2Datestring(s),
			NumComplaints: n_complaints,
			NumComplainers: n_users,
		})

		w.Header().Set("Content-Type", "text/plain")
		cdb.Infof(fmt.Sprintf("set daily counts for %s to {%d,%d}\n",
			date.Time2Datestring(s), n_complaints, n_users))
		w.Write([]byte(fmt.Sprintf("OK!\nSet %s to {%d,%d}\n",
			date.Time2Datestring(s), n_complaints, n_users)))
	}
}

// }}}


// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
