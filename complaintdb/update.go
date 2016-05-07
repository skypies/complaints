package complaintdb

import (
	"fmt"

	"appengine/datastore"
	"appengine/urlfetch"

	"github.com/skypies/geo"
	"github.com/skypies/util/date"

	"github.com/skypies/complaints/complaintdb/types"
	"github.com/skypies/complaints/fr24"

	"github.com/skypies/complaints/flightid"
)

// {{{ cdb.complainByProfile

func (cdb ComplaintDB) complainByProfile(cp types.ComplainerProfile, c *types.Complaint) error {
	client := urlfetch.Client(cdb.C)
	fr := fr24.Fr24{Client: client}
	overhead := fr24.Aircraft{}

	// Check we're not over a daily cap for this user
	s,e := date.WindowForToday()
	if prevKeys,err := cdb.GetComplaintKeysInSpanByEmailAddress(s,e,cp.EmailAddress); err != nil {
		return err
	} else if len(prevKeys) >= KMaxComplaintsPerDay {
		return fmt.Errorf("Too many complaints filed today")
	}

	//cdb.C.Infof("adding complaint for [%s] %s", cp.CallerCode, overhead.FlightNumber)

	// abw hack hack
	grabAny := (cp.CallerCode == "QWERTY")
	if debug,err := fr.FindOverhead(geo.Latlong{cp.Lat,cp.Long}, &overhead, grabAny); err != nil {
		cdb.C.Errorf("FindOverhead failed for %s: %v", cp.EmailAddress, err)
	} else {
		c.Debug = debug
	}
	if overhead.Id != "" {
		c.AircraftOverhead = overhead
	}

	if cp.CallerCode == "WOR004" || cp.CallerCode == "WOR005" {
		elev := 0.0
		oh2,deb2,err2 := flightid.FindOverhead(client,geo.Latlong{cp.Lat,cp.Long},elev, grabAny)
		c.Debug += fmt.Sprintf("\n ***** V2 testing : (%v) %s\n\n%s\n", err2, oh2, deb2)
		if oh2 == nil && overhead.FlightNumber != "" {
			t := c.Debug
			c.Debug = " * * * DIFFERS * * *\n\n" + t
		} else if oh2 != nil {
			if overhead.FlightNumber != oh2.FlightNumber {
				t := c.Debug
				c.Debug = " * * * DIFFERS * * *\n\n" + t
			}
		}
	}
		
	c.Version = kComplaintVersion

	c.Profile = cp // Copy the profile fields into every complaint
	
	// Too much like the last complaint by this user ? Just update that one.
	if prev, err := cdb.GetNewestComplaintByEmailAddress(cp.EmailAddress); err != nil {
		cdb.C.Errorf("complainByProfile/GetNewest: %v", err)
	} else if prev != nil && ComplaintsAreEquivalent(*prev, *c) {
		// The two complaints are in fact one complaint. Overwrite the old one with data from new one.
		Overwrite(prev, c)
		return cdb.UpdateComplaint(*prev, cp.EmailAddress)
	}

	key := datastore.NewIncompleteKey(cdb.C, kComplaintKind, cdb.emailToRootKey(cp.EmailAddress))	
	_, err := datastore.Put(cdb.C, key, c)

	// TEMP
/*
	if debug,err := bksv.PostComplaint(client, cp, *c); err != nil {
		cdb.C.Infof("BKSV Debug\n------\n%s\n------\n", debug)
		cdb.C.Infof("BKSV posting error: %v", err)
	} else {
		cdb.C.Infof("BKSV Debug\n------\n%s\n------\n", debug)
	}
*/
	return err
}

// }}}

// {{{ cdb.ComplainByEmailAddress

func (cdb ComplaintDB) ComplainByEmailAddress(ea string, c *types.Complaint) error {
	var cp *types.ComplainerProfile
	var err error
	cp, err = cdb.GetProfileByEmailAddress(ea)
	if err != nil { return err }

	return cdb.complainByProfile(*cp, c)
}

// }}}
// {{{ cdb.ComplainByCallerCode

func (cdb ComplaintDB) ComplainByCallerCode(cc string, c *types.Complaint) error {
	var cp *types.ComplainerProfile
	var err error
	cp, err = cdb.GetProfileByCallerCode(cc)
	if err != nil || cp == nil { return err }

	return cdb.complainByProfile(*cp, c)
}

// }}}
// {{{ cdb.AddHistoricalComplaintByEmailAddress

func (cdb ComplaintDB) AddHistoricalComplaintByEmailAddress(ea string, c *types.Complaint) error {
	var cp *types.ComplainerProfile
	var err error

	cp, err = cdb.GetProfileByEmailAddress(ea)
	if err != nil { return err }

	c.Profile = *cp

	key := datastore.NewIncompleteKey(cdb.C, kComplaintKind, cdb.emailToRootKey(cp.EmailAddress))	
	_, err = datastore.Put(cdb.C, key, c)
	return err
}

// }}}

// {{{ cdb.UpdateAnyComplaint

func (cdb ComplaintDB) UpdateAnyComplaint(complaint types.Complaint) error {
	if k,err := datastore.DecodeKey(complaint.DatastoreKey); err != nil {
		return err

	} else {
		complaint.Version = kComplaintVersion
		_,err := datastore.Put(cdb.C, k, &complaint)
		return err
	}
}

// }}}
// {{{ cdb.UpdateComplaint

func (cdb ComplaintDB) UpdateComplaint(complaint types.Complaint, ownerEmail string) error {
	k,err := datastore.DecodeKey(complaint.DatastoreKey)
	if err != nil { return err }

	if k.Parent() == nil {
		return fmt.Errorf("Update: key <%v> had no parent", k)
	}
	if k.Parent().StringID() != ownerEmail {
		return fmt.Errorf("Update: key <%v> owned by %s, not %s", k, k.Parent().StringID(), ownerEmail)
	}

	complaint.Version = kComplaintVersion
	
	if _, err2 := datastore.Put(cdb.C, k, &complaint); err2 != nil {
		return err2
	}

	return nil
}

// }}}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
