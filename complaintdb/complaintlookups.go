package complaintdb

import (
	"time"

	"google.golang.org/appengine/datastore"

	"github.com/skypies/geo"
	"github.com/skypies/util/date"

	"github.com/skypies/complaints/complaintdb/types"
)

// {{{ cdb.GetComplaintPositionsInSpanByIcao

// uniqueUsers: if true, only one result per unique user; else one result per complaint.
// icaoid: use empty string to get all complaints; else limits to that aircraft

func (cdb ComplaintDB)GetComplaintPositionsInSpanByIcao(start,end time.Time, uniqueUsers bool, icaoid string) ([]geo.Latlong, error) {
	ret := []geo.Latlong{}

	q := cdb.NewComplaintQuery().
		ByTimespan(start,end).
		Project("Profile.Lat","Profile.Long")

	if uniqueUsers {
		// This is really horribly inefficient :(

		// Can't simply add Distinct() to this, as it forces us to add Timestamp into the projection,
		// which renders Distinct() kinda useless, as the timestamps are always distinct :/
		// So we have to filter afterwards, which is horrible.
		q = cdb.NewComplaintQuery().
			ByTimespan(start,end).
			Project("Profile.Lat","Profile.Long","Profile.EmailAddress")
	}
	
	if icaoid != "" {
		q = q.ByIcaoId(icaoid)
	}

	q = q.Limit(-1)

	// Could do this here ... but maybe semantics clearer higher up the stack.
	// _,data,err := cdb.getMaybeCachedComplaintsByQuery(q, "my_keyname")
	
	results,err := cdb.LookupAll(q)
	if err != nil {
		return ret,err
	}
	
	seen := map[string]int{}
	for _,c := range results {
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
		if c,err := cdb.LookupAll(cdb.CQByEmail(ea)); err != nil {
			return nil,err
		} else {
			cdb.Debugf("GABEA_003", "EVERYTHING retrieved")
			cap.Complaints = c
		}

	} else {
		// Just today
		s,e := date.WindowForToday()
		if c,err := cdb.LookupAll(cdb.CQByEmail(ea).ByTimespan(s,e)); err != nil {
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


// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
