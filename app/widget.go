// This file contains a few routines for parsing form values
package complaints

import (
	"net/http"
	//"strconv"
	"time"
	"github.com/skypies/date"
)

// This widget assumes the values 'date', 'range_from', and 'range_to'
func FormValueDateRange(r *http.Request) (s,e time.Time, err error) {
	err = nil

	switch r.FormValue("date") {
	case "today":
		s,_ = date.WindowForToday()
		e=s
	case "yesterday":
		s,_ = date.WindowForYesterday()
		e=s
	case "range":
		s = date.ArbitraryDatestring2MidnightPdt(r.FormValue("range_from"), "2006/01/02")
		e = date.ArbitraryDatestring2MidnightPdt(r.FormValue("range_to"), "2006/01/02")
		if s.After(e) { s,e = e,s }
	}
	
	e = e.Add(23*time.Hour + 59*time.Minute + 59*time.Second) // make sure e covers its whole day

	return
}
