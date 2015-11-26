package complaintdb

import (
	"time"

	"appengine/datastore"

	"github.com/skypies/date"

	"github.com/skypies/complaints/complaintdb/types"
)

const (
	kComplaintVersion = 2
	kComplaintCoalesceThreshold = 45
)

// {{{ ComplaintsAreEquivalent

func ComplaintsAreEquivalent(this, next types.Complaint) bool {
  // If not close together, or different text, do not coalesce
	if next.Timestamp.Sub(this.Timestamp) > (time.Duration(kComplaintCoalesceThreshold) * time.Second) {
		return false
	}
  if this.Description != next.Description { return false }

	fn1 := this.AircraftOverhead.FlightNumber
	fn2 := next.AircraftOverhead.FlightNumber
	if fn1 == fn2 { return true } // same flight (or no flight)
	if fn1 == "" { return true } // next must have a flight, so this should be coalesced

	return false
}

// }}}
// {{{ FixupComplaint

func FixupComplaint(c *types.Complaint, key *datastore.Key) {
	// 0. Snag the key, so we can refer to this object later
	c.DatastoreKey = key.Encode()

	// 1. GAE datastore helpfully converts timezones to UTC upon storage; fix that
	c.Timestamp = date.InPdt(c.Timestamp)

	// 2. Compute the flight details URL, if within 7 days
	age := date.NowInPdt().Sub(c.Timestamp)
	if age < time.Hour*24*7 {
		c.AircraftOverhead.Fr24Url = c.AircraftOverhead.PlaybackUrl()
	}
}

// }}}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
