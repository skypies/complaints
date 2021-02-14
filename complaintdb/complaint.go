package complaintdb

import (
	"time"

	"github.com/skypies/util/date"

	"github.com/skypies/geo"
	"github.com/skypies/complaints/complaintdb/types"
)

const (
	kComplaintVersion = 2 // Obsolete concept
	kComplaintCoalesceThreshold = 45
)

// {{{ ComplaintsAreEquivalent

// Some users should be coalesced less conservatively *cough*
var rateLimitAddrs = map[string]int{
	"syco1234@gmail.com": 1,
	"test@example.com": 1,
}

func ComplaintsAreEquivalent(this, next types.Complaint) bool {
	fn1 := this.AircraftOverhead.FlightNumber
	fn2 := next.AircraftOverhead.FlightNumber
	d1 := this.Description
	d2 := next.Description

	_,rateLimit := rateLimitAddrs[this.Profile.EmailAddress]

	thresh := kComplaintCoalesceThreshold
	if rateLimit {
		thresh = 5*60
	}
	
	if !rateLimit {
		// If there are different and non-empty descriptions, *never* coaleasce
		if d1 != "" && d2 != "" && d1 != d2 { return false }
	}

	// Else - if same (non-empty) flightnumber, coalesce (regardless of gap between them)
	if fn1 == fn2 && fn1 != "" { return true }
	
	// Else, if time has passed, do not coalesce
	if next.Timestamp.Sub(this.Timestamp) > (time.Duration(thresh)*time.Second) {
		return false
	}

	if rateLimit {
		return true
	}
	
	// So: not much time has passed; the descrips weren't explicitly distinct; and flights differ.
	if d1 != d2 { return false }  // one is empty, but that's enough reason not to coalesce
	if fn1 == fn2 { return true } // identical descriptions & flights; coalesce
	if fn1 == "" { return true }  // identical descriptions, but new has non-empty flight; coalesce

	return false
}

// }}}
// {{{ FixupComplaint

func FixupComplaint(c *types.Complaint, keyStr string) {
	// 0. Snag the key, so we can refer to this object later
	c.DatastoreKey = keyStr //key.Encode()

	// 1. GAE datastore helpfully converts timezones to UTC upon storage; fix that
	c.Timestamp = date.InPdt(c.Timestamp)

	// 2. Compute the flight details URL, if within 24 days
	age := date.NowInPdt().Sub(c.Timestamp)
	if age < time.Hour*24 {
		// c.AircraftOverhead.Fr24Url = c.AircraftOverhead.PlaybackUrl()

		c.AircraftOverhead.Fr24Url = "http://flightaware.com/live/flight/" +
			c.AircraftOverhead.Callsign
		// Or: http://flightaware.com/live/flight/UAL337/history/20151215/ [0655Z/KLAX/KSFO]
		// date is UTC of departure time; might be tricky to guess :/
	}

	// 3. Compute distances, if we have an aircraft
	if c.AircraftOverhead.FlightNumber != "" {
		a := c.AircraftOverhead
		aircraftPos := geo.Latlong{a.Lat,a.Long}
		observerPos := geo.Latlong{c.Profile.Lat, c.Profile.Long}
		c.Dist2KM = observerPos.Dist(aircraftPos)
		c.Dist3KM = observerPos.Dist3(aircraftPos, a.Altitude)
	}
}

// }}}
// {{{ Overwrite

// Overwrite user-entered data (and timestamp) into the base complaint.
func Overwrite(this, from *types.Complaint) {
	orig := *this  // Keep a temp copy
	*this = *from  // Overwrite everything

	// Restore a few key fields from the original
	this.DatastoreKey = orig.DatastoreKey

	// If the orig had a description but new doesn't, don't lose it
	if this.Description == "" && orig.Description != "" {
		this.Description = orig.Description
	}
}

// }}}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
