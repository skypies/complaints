package complaintdb

import (
	"time"

	"github.com/skypies/geo"
	"github.com/skypies/util/date"
	"github.com/skypies/util/dsprovider"

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
// {{{ cdb.GetProfileLocations

// uniqueUsers: if true, only one result per unique user; else one result per complaint.
// icaoid: use empty string to get all complaints; else limits to that aircraft

func (cdb ComplaintDB)GetProfileLocations() ([]geo.Latlong, error) {
	ret := []geo.Latlong{}

	q := cdb.NewProfileQuery().
		//Project("Lat","Long"). // WTF; this limits the resultset to 280 results, not 5300 ??
		Limit(-1)
	
	profiles,err := cdb.LookupAllProfiles(q)
	if err != nil {
		return ret,err
	}

	cdb.Infof("We saw %d locations", len(profiles))
	
	for _,cp := range profiles {
		// Round off the position data to avoid exposing address
		pos := ApproximatePosition(geo.Latlong{Lat:cp.Lat, Long:cp.Long})
		if ! pos.IsNil() {
			ret = append(ret, pos)
		}
	}

	return ret,nil
}

// }}}

// {{{ cdb.getDailyCountsByEmailAdress

func (cdb ComplaintDB) getDailyCountsByEmailAdress(ea string) ([]types.CountItem, error) {
	cdb.Debugf("gDCBEA_001", "starting")
	gs,_ := cdb.LoadGlobalStats()
	cdb.Debugf("gDCBEA_002", "global stats loaded")
	stats := map[string]*DailyCount{}
	maxDays := 60 // As many rows as we care about

	if gs != nil {
		for i,dc := range gs.Counts {
			if i >= maxDays { break }
			stats[date.Datestring2MidnightPdt(dc.Datestring).Format("Jan 02")] = &gs.Counts[i]
		}
	}
	cdb.Debugf("gDCBEA_003", "global stats munged; loading daily")
	
	dailys,err := cdb.GetDailyCounts(ea)
	if err != nil {
		return []types.CountItem{}, err
	}

	counts := []types.CountItem{}

	cdb.Debugf("gDCBEA_004", "daily stats loaded (%d dailys, %d stats)", len(dailys), len(stats))
	for i,daily := range dailys {
		if i >= maxDays { break }
		item := types.CountItem{
			Key: daily.Timestamp().Format("Jan 02"),
			Count: daily.NumComplaints,
		}
		if dc,exists := stats[item.Key]; exists {
			item.TotalComplainers = dc.NumComplainers
			item.TotalComplaints = dc.NumComplaints
			item.IsMaxComplainers = dc.IsMaxComplainers
			item.IsMaxComplaints = dc.IsMaxComplaints
		}
		counts = append(counts, item)
	}
	cdb.Debugf("gDCBEA_005", "daily stats munged (%d counts)", len(counts))

	return counts, nil
}

// }}}
// {{{ cdb.GetAllByEmailAddress

func (cdb ComplaintDB) GetAllByEmailAddress(ea string, everything bool) (*types.ComplaintsAndProfile, error) {
	var cap types.ComplaintsAndProfile

	cdb.Debugf("GABEA_001", "cdb.GetAllByEmailAddress starting (everything=%v)", everything)
	
	if cp,err := cdb.MustLookupProfile(ea); err == dsprovider.ErrNoSuchEntity {
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

func (cdb ComplaintDB)CountComplaintsAndUniqueUsersIn(s,e time.Time) (int, int, error) {
	// What we'd like to do, is to do Project("Profile.EmailAddress").Distinct().ByTimespan().
	// But we can't, because ...
	//  1. ByTimespan does Filter("Timestamp") ...
	//  2. so we need to include "Timestamp" in the Project() args ...
	//  3. but Distinct() acts on all projected fields, and the timestamps defeat grouping
	// So we need to count users manually.
	q := cdb.NewComplaintQuery().Project("Profile.EmailAddress").ByTimespan(s,e)

	complaints,err := cdb.LookupAll(q)
	if err != nil {
		return 0, 0, err
	}

	users := map[string]int{}
	for _,c := range complaints {
		users[c.Profile.EmailAddress]++
	}

	return len(complaints), len(users), nil
}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
