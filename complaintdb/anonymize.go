package complaintdb

import (
	"crypto/sha512"
	"fmt"

	//"golang.org/x/crypto/bcrypt"

	"github.com/skypies/geo"
	"github.com/skypies/util/date"

	"github.com/skypies/complaints/config"
	"github.com/skypies/complaints/complaintdb/types"
)

func profile2fingerprint(p types.ComplainerProfile) string {
	// We use a single fixed secret salt, to prevent the guessing of hashes when given an email
	// address.
	salt := config.Get("anonymizer.salt")
	if salt == "" { return "" } // refuse to add unique fingerprints if we don't have salt

	data := []byte(salt + p.EmailAddress)
	return fmt.Sprintf("%x", sha512.Sum512_256(data))

	/*	bcrypt is too expensive when dumping all complaints
	if hash,err := bcrypt.GenerateFromPassword(data, bcrypt.DefaultCost); err != nil {
		return ""
	} else {
		return fmt.Sprintf("%x", hash)
	}
*/
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
