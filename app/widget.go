// This file contains a few routines for parsing form values
package complaints

import (
	"net/http"
	"strconv"
	"time"
	"github.com/skypies/date"
)

// {{{ FormValueDateRange

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

// }}}
// {{{ FormValueInt64

func FormValueInt64(r *http.Request, name string) int64 {
	val,_ := strconv.ParseInt(r.FormValue(name), 10, 64)
	return val
}

// }}}
// {{{ FormValueFloat64

func FormValueFloat64(w http.ResponseWriter, r *http.Request, name string) float64 {
	
	if val,err := strconv.ParseFloat(r.FormValue(name), 64); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return -1
	} else {
		return val
	}
}

// }}}
// {{{ FormValueCheckbox

func FormValueCheckbox(r *http.Request, name string) bool {
	if r.FormValue(name) != "" {
		return true
	}
	return false
}

// }}}


// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
