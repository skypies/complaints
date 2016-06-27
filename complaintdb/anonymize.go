package complaintdb

import (
	"crypto/sha512"
	"fmt"

	"github.com/skypies/geo"
	"github.com/skypies/util/date"

	"github.com/skypies/complaints/complaintdb/types"
)

func profile2fingerprint(p types.ComplainerProfile) string {
	data := []byte(p.EmailAddress)
	return fmt.Sprintf("%x", sha512.Sum512_256(data))
}

func AnonymizeComplaint(c *types.Complaint) *types.AnonymizedComplaint {
	if c == nil { return nil }

	postalAddress := c.Profile.GetStructuredAddress()

	ac := types.AnonymizedComplaint{
		Timestamp: c.Timestamp,
		Speedbrakes: c.HeardSpeedbreaks,
		Loudness: c.Loudness,
		Activity: c.Activity,

		User: profile2fingerprint(c.Profile),
		City: postalAddress.City,
		Zip: postalAddress.Zip,

		// Denormalized fields
		DatePST: date.InPdt(c.Timestamp).Format("2006-01-02"), // Format is same as BQ's DATE() func
		HourPST: date.InPdt(c.Timestamp).Hour(),
		
		// All of these fields might be nil.
		FlightNumber: c.AircraftOverhead.FlightNumber,
		AirlineCode: c.AircraftOverhead.IATAAirlineCode(),
	
		Origin: c.AircraftOverhead.Origin,
		Destination: c.AircraftOverhead.Destination,
		EquipType: c.AircraftOverhead.EquipType,

		Latlong: geo.Latlong{c.AircraftOverhead.Lat, c.AircraftOverhead.Long},
		PressureAltitude: c.AircraftOverhead.Altitude,
		Groundspeed: c.AircraftOverhead.Speed,
	}

	if ac.HasIdenitifiedAircraft() {
		ac.FlightKey = fmt.Sprintf("%s-%s", ac.FlightNumber,
			date.InPdt(ac.Timestamp).Format("20060102"))
	}

	return &ac
}
