package complaintdb

import (
	"fmt"
	"sort"
	"time"

	"google.golang.org/appengine/datastore"

	"github.com/skypies/geo"
	"github.com/skypies/util/date"

	"github.com/skypies/complaints/complaintdb/types"
)

// {{{ cdb.getComplaintsByQueryFromDatastore

func (cdb ComplaintDB)getComplaintsByQueryFromDatastore(q *datastore.Query) ([]*datastore.Key, []types.Complaint, error) {
	
	cdb.Debugf("gCBQFD_200", "getComplaintsByQueryFromDatastore")

	var data = []types.Complaint{}

	cdb.Debugf("gCBQFD_201", "calling GetAll() ...")
	keys, err := q.GetAll(cdb.Ctx(), &data)
	cdb.Debugf("gCBQFD_202", "... call done (n=%d)", len(keys))

	// We tolerate missing fields, because the DB is full of old objects with dead fields
	if err != nil {
		if mismatchErr,ok := err.(*datastore.ErrFieldMismatch); ok {
			_=mismatchErr
			// cdb.Debugf("gCBQFD_203", "missing field: %v", mismatchErr)
		} else {			
			return nil, nil, fmt.Errorf("gCBQFD: %v", err)
		}
	}

	return keys, data, nil
}

// }}}
// {{{ cdb.getComplaintsByQuery

func (cdb ComplaintDB)getComplaintsByQuery(q *datastore.Query, memKey string) ([]types.Complaint, error) {

	keys,complaints,err := cdb.getComplaintsByQueryFromDatastore(q)
	//keys,complaints,err := cdb.getMaybeCachedComplaintsByQuery(q,memKey)
	if err != nil { return nil, err}
	
	// Data fixups !
	for i, _ := range complaints {
		FixupComplaint(&complaints[i], keys[i])
	}

	sort.Sort(types.ComplaintsByTimeDesc(complaints))

	return complaints, nil
}

// }}}

// {{{ cdb.GetRecentComplaintsByEmailAddress

func (cdb ComplaintDB) GetRecentComplaintsByEmailAddress(ea string, n int) ([]types.Complaint, error) {
	q := datastore.
		NewQuery(kComplaintKind).
		Ancestor(cdb.emailToRootKey(ea)).
		Order("-Timestamp").
		Limit(n)

	return cdb.getComplaintsByQuery(q,"")
}

// }}}
// {{{ cdb.GetComplaintsByEmailAddress

func (cdb ComplaintDB) GetComplaintsByEmailAddress(ea string) ([]types.Complaint, error) {
	q := datastore.
		NewQuery(kComplaintKind).
		Ancestor(cdb.emailToRootKey(ea)).
		Order("Timestamp")

	cdb.Infof(" ##== all-comp")

	return cdb.getComplaintsByQuery(q,"")
}

// }}}
// {{{ cdb.GetComplaintsInSpanByEmailAddress

func (cdb ComplaintDB) GetComplaintsInSpanByEmailAddress(ea string, start,end time.Time) ([]types.Complaint, error) {

	//cdb.Infof(" ##== comp-in-span [%s  -->  %s]", start, end)
	memKey := ""
	todayStart,_ := date.WindowForToday()
	if (end.Before(todayStart) || end.Equal(todayStart)) {
		memKey = fmt.Sprintf("comp-in-span:%s:%d-%d", ea, start.Unix(), end.Unix())
		//cdb.Infof(" ##== comp-in-span cacheable [%s]", memKey)
	}	

	q := cdb.QueryInSpanByEmailAddress(start, end, ea)

	return cdb.getComplaintsByQuery(q,memKey)
}

// }}}
// {{{ cdb.GetOldestComplaintByEmailAddress

func (cdb ComplaintDB) GetOldestComplaintByEmailAddress(ea string) (*types.Complaint, error) {
	q := datastore.
		NewQuery(kComplaintKind).
		Ancestor(cdb.emailToRootKey(ea)).
		Order("Timestamp").
		Limit(1)

	if complaints,err := cdb.getComplaintsByQuery(q,""); err != nil {
		return nil, err
	} else if len(complaints) == 0 {
		return nil,nil
	} else {
		return &complaints[0], nil
	}
}

// }}}
// {{{ cdb.GetNewestComplaintByEmailAddress

func (cdb ComplaintDB) GetNewestComplaintByEmailAddress(ea string) (*types.Complaint, error) {
	q := datastore.
		NewQuery(kComplaintKind).
		Ancestor(cdb.emailToRootKey(ea)).
		Order("-Timestamp").
		Limit(1)

	if complaints,err := cdb.getComplaintsByQuery(q,""); err != nil {
		return nil, err
	} else if len(complaints) == 0 {
		return nil,nil
	} else {
		return &complaints[0], nil
	}
}

// }}}
// {{{ cdb.GetComplaintsWithSpeedbrakes

func (cdb ComplaintDB) GetComplaintsWithSpeedbrakes() ([]types.Complaint, error) {
	q := datastore.
		NewQuery(kComplaintKind).
		Filter("HeardSpeedbreaks = ", true).
		Filter("AircraftOverhead.FlightNumber > ", "")

	if complaints,err := cdb.getComplaintsByQuery(q,""); err != nil {
		return nil, err
	} else if len(complaints) == 0 {
		return nil,nil
	} else {
		return complaints, nil
	}
}

// }}}
// {{{ cdb.GetComplaintsInSpan

// DO NOT USE ... the better way is below !
func (cdb ComplaintDB)GetComplaintsInSpan(start,end time.Time) ([]types.Complaint, error) {
	profiles,err := cdb.GetAllProfiles()
	if err != nil { return nil,err }

	allComplaints := []types.Complaint{}
	
	for _,p := range profiles {
		if c,err := cdb.GetComplaintsInSpanByEmailAddress(p.EmailAddress, start, end); err != nil {
			return nil,err
		} else {
			allComplaints = append(allComplaints, c...)
		}
	}

	return allComplaints,nil
}

// }}}
// {{{ cdb.GetComplaintsInSpanNew

// Now the DB is clean, we can do this simple query instead of going user by user
func (cdb ComplaintDB)GetComplaintsInSpanNew(start,end time.Time) ([]types.Complaint, error) {
	cdb.Infof(" ##== comp-in-span [%s  -->  %s]", start, end)
	memKey := ""
	todayStart,_ := date.WindowForToday()
	if (end.Before(todayStart) || end.Equal(todayStart)) {
		memKey = fmt.Sprintf("comp-in-span:__all__:%d-%d", start.Unix(), end.Unix())
		//cdb.Infof(" ##== comp-in-span cacheable [%s]", memKey)
	}	
	
	q := datastore.
		NewQuery(kComplaintKind).
		Filter("Timestamp >= ", start).
		Filter("Timestamp < ", end).
		Order("Timestamp").
		Limit(-1)

	return cdb.getComplaintsByQuery(q,memKey)
}

// }}}

// {{{ cdb.GetComplaintTimesInSpanByFlight

type timeAsc []time.Time
func (a timeAsc) Len() int           { return len(a) }
func (a timeAsc) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a timeAsc) Less(i, j int) bool { return a[i].Before(a[j]) }

func (cdb ComplaintDB)GetComplaintTimesInSpanByFlight(start,end time.Time, flight string) ([]time.Time, error) {
	
	q := datastore.
		NewQuery(kComplaintKind).
		Project("Timestamp").
		Filter("Timestamp >= ", start).
		Filter("Timestamp < ", end).
		Filter("AircraftOverhead.FlightNumber = ", flight).
		Limit(-1)

	var data = []types.Complaint{}
	if _,err := q.GetAll(cdb.Ctx(), &data); err != nil {
		return []time.Time{}, err
	}

	times := []time.Time{}
	for _,c := range data {
		times = append(times, c.Timestamp)
	}

	sort.Sort(timeAsc(times))
	
	return times, nil
}

// }}}
// {{{ cdb.GetComplaintPositionsInSpanByIcao

// uniqueUsers: if true, only one result per unique user; else one result per complaint.
// icaoid: use empty string to get all complaints; else limits to that aircraft

func (cdb ComplaintDB)GetComplaintPositionsInSpanByIcao(start,end time.Time, uniqueUsers bool, icaoid string) ([]geo.Latlong, error) {
	ret := []geo.Latlong{}

	q := datastore.
		NewQuery(kComplaintKind).
		Project("Profile.Lat","Profile.Long").
		Filter("Timestamp >= ", start).
		Filter("Timestamp < ", end)

	if uniqueUsers {
		cdb.Infof("Oho, limiting to unique users, in a HORRIFIC WAY")

		// Can't simply add Distinct() to this, as it forces us to add Timestamp into the projection,
		// which renders Distinct() kinda useless, as the timestamps are always distinct :/
		// So we have to filter afterwards, which is horrible.
		q = datastore.
			NewQuery(kComplaintKind).
			Project("Profile.Lat","Profile.Long","Profile.EmailAddress").
			Filter("Timestamp >= ", start).
			Filter("Timestamp < ", end)
	}
	
	if icaoid != "" {
		cdb.Infof("Oho, adding id2=%s", icaoid)
		q = q.Filter("AircraftOverhead.Id2 = ", icaoid)
	}

	q = q.Limit(-1)

	// Could do this here ... but maybe semantics clearer higher up the stack.
	// _,data,err := cdb.getMaybeCachedComplaintsByQuery(q, "my_keyname")
	
	var data = []types.Complaint{}
	if _,err := q.GetAll(cdb.Ctx(), &data); err != nil {
		return ret,err
	}
	
	seen := map[string]int{}
	for _,c := range data {
		if uniqueUsers {
			if _,exists := seen[c.Profile.EmailAddress]; exists {
				continue
			} else {
				seen[c.Profile.EmailAddress]++
			}
		}

		// Round off the position data to avoid exposing address
		pos := ApproximatePosition(geo.Latlong{Lat:c.Profile.Lat, Long:c.Profile.Long})
		if ! pos.IsNil() {
			ret = append(ret, pos)
		}
	}

	return ret,nil
}

// }}}

// {{{ cdb.GetAnyComplaintByKey

func (cdb ComplaintDB) GetAnyComplaintByKey(keyString string) (*types.Complaint, error) {
	k,err := datastore.DecodeKey(keyString)
	if err != nil { return nil,err }

	complaint := types.Complaint{}
	if err := datastore.Get(cdb.Ctx(), k, &complaint); err != nil {
		return nil,err
	}

	FixupComplaint(&complaint, k)
	
	return &complaint, nil
}

// }}}
// {{{ cdb.GetComplaintByKey

func (cdb ComplaintDB) GetComplaintByKey(keyString string, ownerEmail string) (*types.Complaint, error) {
	k,err := datastore.DecodeKey(keyString)
	if err != nil { return nil,err }

	if k.Parent() == nil {
		return nil,fmt.Errorf("Get: key <%v> had no parent", k)
	}
	if !cdb.admin && k.Parent().StringID() != ownerEmail {
		return nil,fmt.Errorf("Get: key <%v> owned by %s, not %s", k, k.Parent().StringID(), ownerEmail)
	}

	complaint := types.Complaint{}
	if err := datastore.Get(cdb.Ctx(), k, &complaint); err != nil {
		return nil,err
	}

	FixupComplaint(&complaint, k)
	
	return &complaint, nil
}

// }}}

// {{{ cdb.GetAllByEmailAddress

func (cdb ComplaintDB) GetAllByEmailAddress(ea string, everything bool) (*types.ComplaintsAndProfile, error) {
	var cap types.ComplaintsAndProfile

	cdb.Debugf("GABEA_001", "cdb.GetAllByEmailAddress starting (everything=%v)", everything)
	
	if cp,err := cdb.GetProfileByEmailAddress(ea); err == datastore.ErrNoSuchEntity {
		return nil,nil  // No such profile exists
	} else if err != nil {
		return nil,err  // A real problem occurred
	} else {
		cdb.Debugf("GABEA_002", "profile retrieved")
		cap.Profile = *cp
	}

	if everything {
		if c,err := cdb.GetComplaintsByEmailAddress(ea); err != nil {
			return nil,err
		} else {
			cdb.Debugf("GABEA_003", "EVERYTHING retrieved")
			cap.Complaints = c
		}

	} else {
		// Just today
		s,e := date.WindowForToday()
		if c,err := cdb.GetComplaintsInSpanByEmailAddress(ea,s,e); err != nil {
			return nil,err
		} else {
			cdb.Debugf("GABEA_004", "WindowForToday retrieved; now getting counts")
			cap.Complaints = c
		}
	}

	if counts,err := cdb.getDailyCountsByEmailAdress(ea); err != nil {
		return nil,err
	} else {
		cdb.Debugf("GABEA_005", "counts retrieved")
		cap.Counts = counts
	}
	
	return &cap, nil
}

// }}}

// {{{ cdb.GetComplaintKeysInSpan

// Now the DB is clean, we can do this simple query instead of going user by user
func (cdb ComplaintDB)GetComplaintKeysInSpan(start,end time.Time) ([]*datastore.Key, error) {
	q := datastore.
		NewQuery(kComplaintKind).
		Filter("Timestamp >= ", start).
		Filter("Timestamp < ", end).
		KeysOnly()

	keys, err := q.GetAll(cdb.Ctx(), nil)
	return keys,err
}

// }}}
// {{{ cdb.GetComplaintKeysInSpanByEmailAddress

func (cdb ComplaintDB)GetComplaintKeysInSpanByEmailAddress(start,end time.Time, ea string) ([]*datastore.Key, error) {
	q := cdb.QueryInSpanByEmailAddress(start, end, ea).KeysOnly()
	keys, err := q.GetAll(cdb.Ctx(), nil)
	return keys,err
}

// }}}


// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
