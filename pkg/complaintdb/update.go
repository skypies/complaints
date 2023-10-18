package complaintdb

import (
	"fmt"

	"github.com/skypies/geo"
	"github.com/skypies/util/date"

	"github.com/skypies/pi/airspace"
	"github.com/skypies/flightdb/fr24"

	"github.com/skypies/complaints/pkg/flightid"
)

// {{{ cdb.complainByProfile

func (cdb ComplaintDB) complainByProfile(cp ComplainerProfile, c *Complaint) error {
	client := cdb.HTTPClient()
	overhead := flightid.Aircraft{}

	// Check we're not over a daily cap for this user
	cdb.Debugf("cbe_010", "doing rate limit check for %s", cp.EmailAddress)
	s,e := date.WindowForToday()
	//if prevKeys,err := cdb.GetComplaintKeysInSpanByEmailAddress(s,e,cp.EmailAddress); err != nil {
	if prevKeys,err := cdb.LookupAllKeys(cdb.CQByEmail(cp.EmailAddress).ByTimespan(s,e)); err != nil {
		return err
	} else if len(prevKeys) >= KMaxComplaintsPerDay {
		return fmt.Errorf("Too many complaints filed today by %s", cp.EmailAddress)
	} else {
		cdb.Debugf("cbe_011", "rate limit check passed (%d); calling FindOverhead", len(prevKeys))
	}
	
	elev := cp.ElevationFeet()
	pos := geo.Latlong{cp.Lat,cp.Long}

	algoName := cp.SelectorAlgorithm
	if (c.Description == "ANYANY") { algoName = "random" }
	algo := flightid.NewSelector(algoName)

	if as,err := fr24.GRPCFetchAirspace(pos.Box(64,64)); err != nil {
		cdb.Errorf("FindOverhead failed for %s: %v", cp.EmailAddress, err)
	} else {
		oh,deb := flightid.IdentifyOverhead(as,pos,elev,algo)
		c.Debug = deb
		if oh != nil {
			overhead = *oh
			c.AircraftOverhead = overhead
		}
	}

	cdb.Debugf("cbe_020", "FindOverhead returned")
	
	// Contrast with the skypi pathway
	if cp.CallerCode == "WOR004" || cp.CallerCode == "WOR005" {
		asFdb,_ := airspace.Fetch(client, "", "fdb", pos.Box(60,60))
		oh3,deb3 := flightid.IdentifyOverhead(asFdb,pos,elev,algo)
		if oh3 == nil { oh3 = &flightid.Aircraft{} }
		newdebug := c.Debug + "\n*** v2 / fdb testing\n" + deb3 + "\n"
		headline := ""

		asAex,_ := airspace.Fetch(client, "", "aex", pos.Box(60,60))
		oh4,deb4 := flightid.IdentifyOverhead(asAex,pos,elev,algo)
		newdebug += "\n*** v3 / AdsbExchange testing\n" + deb4 + "\n"
		_=oh4
		
		if overhead.FlightNumber != oh3.FlightNumber {
			headline = fmt.Sprintf("** * * DIFFERS * * **\n")
		} else {
			// Agree ! Copy over the Fdb IdSpec, and pretend we're living in the future
			headline = fmt.Sprintf("**---- Agrees ! ----**\n")
			c.AircraftOverhead.Id = oh3.Id
		}
		headline += fmt.Sprintf(" * skypi: %s\n * orig : %s\n", oh3, overhead)
		
		c.Debug = headline + newdebug
	}
	
	c.Profile = cp // Copy the profile fields into every complaint
	
	// Too much like the last complaint by this user ? Just update that one.
	cdb.Debugf("cbe_030", "retrieving prev complaint")

	if prev, err := cdb.LookupFirst(cdb.CQByEmail(cp.EmailAddress).Order("-Timestamp")); err != nil {
		cdb.Errorf("complainByProfile/GetNewest: %v", err)
	} else if prev != nil && ComplaintsShouldBeMerged(*prev, *c) {
		cdb.Debugf("cbe_031", "returned, equiv; about to UpdateComlaint()")
		// The two complaints are in fact one complaint. Overwrite the old one with data from new one.
		Overwrite(prev, c)
		err := cdb.UpdateComplaint(*prev, cp.EmailAddress)
		cdb.Debugf("cbe_032", "updated in place (all done)")
		return err
	}

	cdb.Debugf("cbe_033", "returned, distinct/first; about to put()")
	err := cdb.PersistComplaint(*c)
	cdb.Debugf("cbe_034", "new entity added (all done)")

	return err
}

// }}}

// {{{ cdb.ComplainByEmailAddress

func (cdb ComplaintDB) ComplainByEmailAddress(ea string, c *Complaint) error {
	var cp *ComplainerProfile
	var err error

	cdb.Debugf("cbe_001", "ComplainByEmailAddress starting")
	cp, err = cdb.MustLookupProfile(ea)
	if err != nil { return err }
	cdb.Debugf("cbe_002", "profile obtained")

	return cdb.complainByProfile(*cp, c)
}

// }}}
// {{{ cdb.ComplainByCallerCode

func (cdb ComplaintDB) ComplainByCallerCode(cc string, c *Complaint) error {
	if profiles, err := cdb.LookupAllProfiles(cdb.NewProfileQuery().ByCallerCode(cc)); err != nil {
		return err
	} else if len(profiles) != 1 {
		return fmt.Errorf("ComplainByCallerCode: %d profiles for id='%s'", len(profiles))
	} else {
		return cdb.complainByProfile(profiles[0], c)
	}
}

// }}}
// {{{ cdb.ComplainByButtonId

func (cdb ComplaintDB) ComplainByButtonId(id string, c *Complaint) error {
	if profiles, err := cdb.LookupAllProfiles(cdb.NewProfileQuery().ByButton(id)); err != nil {
		return err
	} else if len(profiles) != 1 {
		return fmt.Errorf("ComplainByButtonId: %d profiles for id='%s'", len(profiles))
	} else {
		return cdb.complainByProfile(profiles[0], c)
	}
}

// }}}
// {{{ cdb.AddHistoricalComplaintByEmailAddress

func (cdb ComplaintDB) AddHistoricalComplaintByEmailAddress(ea string, c *Complaint) error {
	var cp *ComplainerProfile
	var err error

	cp, err = cdb.MustLookupProfile(ea)
	if err != nil { return err }

	c.Profile = *cp

	return cdb.PersistComplaint(*c)
}

// }}}

// {{{ cdb.UpdateComplaint

// If owner is not nil, then complaint must be owned by it
func (cdb ComplaintDB) UpdateComplaint(c Complaint, owner string) error {
	keyer,err := cdb.Provider.DecodeKey(c.DatastoreKey)
	if err != nil { return fmt.Errorf("UpdateComplaint/DecodeKey: %v", err) }

	parentKeyer := cdb.Provider.KeyParent(keyer)
	if parentKeyer == nil { return fmt.Errorf("UpdateComplaint: key <%v> had no parent", keyer) }

	if owner != "" && !cdb.admin && cdb.Provider.KeyName(parentKeyer) != owner {
		return fmt.Errorf("UpdateComplaint: key <%v> owned by %s, not %s",
			keyer, cdb.Provider.KeyName(parentKeyer), owner)
	}

	return cdb.PersistComplaint(c)
}

// }}}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
