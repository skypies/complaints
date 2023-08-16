package complaintdb

import (
	"time"

	"github.com/skypies/util/date"
	"github.com/skypies/geo"
)

const (
	kComplaintMergeThreshold     =  45 * time.Second
	kComplaintWideMergeThreshold = 300 * time.Second
)

var(
	mergeHappyAddrs = map[string]int{ // Some users should be merged less conservatively
		"syco1234@gmail.com": 1,
		"test@example.com": 1,
	}
)

// {{{ ComplaintsShouldBeMerged

// ComplaintsShouldBeMerged decides whether two complaint objects
// relate to the same underlying action by a user. Sometimes a user
// will mash the button; some users try to automate sending as many
// complaints as they can.
func ComplaintsShouldBeMerged(this, next Complaint) bool {
	thresh := kComplaintMergeThreshold

	// Check to see if the user is suspected to automate, if so give them less benefit of the doubt
	if _, exists := mergeHappyAddrs[this.Profile.EmailAddress]; exists {
		thresh = kComplaintWideMergeThreshold
	}

	// If same (non-empty) flightnumber, merge (regardless of gap between them)
	fn1 := this.AircraftOverhead.FlightNumber
	fn2 := next.AircraftOverhead.FlightNumber
	if fn1 == fn2 && fn1 != "" {
		return true
	}

	// If they are too close together in time, merge
	if next.Timestamp.Sub(this.Timestamp) <= thresh {
		return true
	}

	return false
}

// }}}
// {{{ FixupComplaint

func FixupComplaint(c *Complaint, keyStr string) {
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

	// Some old complaints have junk submission timestamps
	if c.Submission.T.Year() == 0 {
		var zeroTime time.Time
		c.Submission.T = zeroTime
	}
	// They can also have entirely uninitialized response slices
	if c.Submission.Response == nil {
		c.Submission.Response = []byte{}
	}

}

// }}}
// {{{ Overwrite

// Overwrite user-entered data (and timestamp) into the base complaint.
func Overwrite(this, from *Complaint) {
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
