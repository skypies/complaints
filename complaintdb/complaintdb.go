package complaintdb

// {{{ globals

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"net/http"
	"sort"
	"time"

	"appengine"
	"appengine/datastore"
	"appengine/memcache"

	"github.com/skypies/util/date"

	"github.com/skypies/complaints/complaintdb/types"
)

var(
	kComplaintKind = "ComplaintKind"
	kComplainerKind = "ComplainerKind"

	KMaxComplaintsPerDay = 200
)

// }}}

// {{{ type ComplaintDB

type ComplaintDB struct {
	Req *http.Request  // To allow the construction of 'newappengine' contexts, for gaeutil
	C appengine.Context
	Memcache bool
	StartTime time.Time

}

func NewComplaintDB(r *http.Request) ComplaintDB {
	return ComplaintDB{
		Req:      r,
		C:        appengine.Timeout(appengine.NewContext(r), 300 * time.Second),
		Memcache: false,
		StartTime: time.Now(),
	}
}

// }}}

// {{{ cdb.Debugf

func (cdb ComplaintDB)Debugf(step string, fmtstr string, varargs ...interface{}) {
	payload := fmt.Sprintf(fmtstr, varargs...)
	cdb.C.Debugf("[%s] %9.6f %s", step, time.Since(cdb.StartTime).Seconds(), payload)
}

// }}}

// {{{ cdb.getDailyCountsByEmailAdress

func (cdb ComplaintDB) getDailyCountsByEmailAdress(ea string) ([]types.CountItem, error) {
	counts := []types.CountItem{}

	cdb.Debugf("gDailyCounts_001", "starting")
	gs,_ := cdb.LoadGlobalStats()
	cdb.Debugf("gDailyCounts_002", "global stats loaded")
	stats := map[string]*DailyCount{}
	if gs != nil {
		for i,dc := range gs.Counts {
			stats[date.Datestring2MidnightPdt(dc.Datestring).Format("Jan 02")] = &gs.Counts[i]
		}
	}
	cdb.Debugf("gDailyCounts_003", "global stats munged; loading daily")
	
	if dailys,err := cdb.GetDailyCounts(ea); err != nil {
		return counts, err

	} else {
		cdb.Debugf("gDailyCounts_004", "daily stats loaded")
		for _,daily := range dailys {
			// cdb.C.Infof(" -- we have a daily: %#v", daily)
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
		cdb.Debugf("gDailyCounts_004", "daily stats munged")
	}

	return counts, nil
}

// }}}

// {{{ cdb.EmailToRootKey

func (cdb ComplaintDB) emailToRootKey(email string) *datastore.Key {
	return datastore.NewKey(cdb.C, kComplainerKind, email, 0, nil)
}
// Sigh
func (cdb ComplaintDB) EmailToRootKey(email string) *datastore.Key {
	return cdb.emailToRootKey(email)
}

// }}}
// {{{ cdb.GetAllProfiles

func (cdb ComplaintDB) GetAllProfiles() (cps []types.ComplainerProfile, err error) {
	q := datastore.NewQuery(kComplainerKind)
	cps = []types.ComplainerProfile{}
	_, err = q.GetAll(cdb.C, &cps)
	return
}

// }}}
// {{{ cdb.GetEmailCityMap

func (cdb ComplaintDB) GetEmailCityMap() (map[string]string, error) {
	cities := map[string]string{}

	q := datastore.NewQuery(kComplainerKind).Project("EmailAddress", "StructuredAddress.City")
	profiles := []types.ComplainerProfile{}
	if _,err := q.GetAll(cdb.C, &profiles); err != nil {
		return cities, err
	}

	for _,profile := range profiles {
		city := profile.StructuredAddress.City
		if city == "" { city = "Unknown" }
		cities[profile.EmailAddress] = city
	}

	return cities, nil
}

// }}}

// {{{ cdb.DeleteComplaints

func (cdb ComplaintDB) DeleteComplaints(keyStrings []string, ownerEmail string) error {
	keys := []*datastore.Key{}
	for _,s := range keyStrings {
		k,err := datastore.DecodeKey(s)
		if err != nil { return err }

		if k.Parent() == nil {
			return fmt.Errorf("key <%v> had no parent", k)
		}
		if k.Parent().StringID() != ownerEmail {
			return fmt.Errorf("key <%v> owned by %s, not %s", k, k.Parent().StringID(), ownerEmail)
		}
		keys = append(keys, k)
	}
	return datastore.DeleteMulti(cdb.C, keys)
}

// }}}

// {{{ cdb.GetProfileByCallerCode

func (cdb ComplaintDB) GetProfileByCallerCode(cc string) (cp *types.ComplainerProfile, err error) {
	q := datastore.NewQuery(kComplainerKind).Filter("CallerCode =", cc)
	var results = []types.ComplainerProfile{}
	_, err = q.GetAll(cdb.C, &results)
	if err != nil { return }
	if len(results) == 0 { return } // No match
	/*for _,v := range results {
		cdb.C.Infof(">>> RESULT: %v", v)
	}*/
	if len(results) > 1 {
		err = fmt.Errorf ("lookup(%s) found %d results", cc, len(results))
		return
	}

	cp = &results[0]
	return 
}

// }}}
// {{{ cdb.GetProfileByEmailAddress

func (cdb ComplaintDB) GetProfileByEmailAddress(ea string) (*types.ComplainerProfile, error) {
	var cp types.ComplainerProfile
	err := datastore.Get(cdb.C, cdb.emailToRootKey(ea), &cp)
	return &cp, err
}

// }}}
// {{{ cdb.PutProfile

func (cdb ComplaintDB) PutProfile(cp types.ComplainerProfile) error {
	_, err := datastore.Put(cdb.C, cdb.emailToRootKey(cp.EmailAddress), &cp)
	cdb.C.Infof(">>> PutProfile: %v [err=%v]", cp, err)
	return err
}

// }}}

// {{{ cdb.GetComplainersWithinSpan

func (cdb ComplaintDB)GetComplainersWithinSpan(start,end time.Time) ([]string, error) {
	q := datastore.
		NewQuery(kComplaintKind).
		Project("Profile.EmailAddress").//Distinct(). // Sigh, can't do that *and* filter
		Filter("Timestamp >= ", start).
		Filter("Timestamp < ", end).
		Limit(-1)

	var data = []types.Complaint{}
	if _,err := q.GetAll(cdb.C, &data); err != nil {
		return []string{}, err
	}
	
	uniques := map[string]int{}
	for _,c := range data {
		uniques[c.Profile.EmailAddress]++
	}

	ret := []string{}
	for e,_ := range uniques {
		ret = append(ret, e)
	}
	
	return ret, nil
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

	keys, err := q.GetAll(cdb.C, nil)
	return keys,err
}

// }}}
// {{{ cdb.GetComplaintKeysInSpanByEmailAddress

func (cdb ComplaintDB)GetComplaintKeysInSpanByEmailAddress(start,end time.Time, ea string) ([]*datastore.Key, error) {
	q := cdb.QueryInSpanByEmailAddress(start, end, ea).KeysOnly()
	keys, err := q.GetAll(cdb.C, nil)
	return keys,err
}

// }}}

// {{{ BytesFromShardedMemcache

const chunksize = 950000

// bool means 'found'
func BytesFromShardedMemcache(c appengine.Context, key string) ([]byte, bool) {
	keys := []string{}
	for i:=0; i<32; i++ { keys = append(keys, fmt.Sprintf("=%d=%s",i*chunksize,key)) }

	if items,err := memcache.GetMulti(c, keys); err != nil {
		c.Errorf("fdb memcache multiget: %v", err)
		return nil,false

	} else {
		b := []byte{}
		for i:=0; i<32; i++ {
			if item,exists := items[keys[i]]; exists==false {
				break
			} else {
				c.Infof(" #=== Found '%s' !", item.Key)
				b = append(b, item.Value...)
			}
		}

		c.Infof(" #=== Final read len: %d", len(b))

		/*
		buf := bytes.NewBuffer(b)
		flights := []Flight{}
		if err := gob.NewDecoder(buf).Decode(&flights); err != nil {
			db.C.Errorf("fdb memcache multiget decode: %v", err)
			return nil,false
		}
*/
		if len(b) > 0 {
			return b, true
		} else {
			return nil, false
		}
	}
}

// }}}
// {{{ BytesToShardedMemcache

// Object usually too big (1MB limit), so shard.
// http://stackoverflow.com/questions/9127982/
func BytesToShardedMemcache(c appengine.Context, key string, b []byte) {
/*
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(f); err != nil {
		db.C.Errorf("fdb error encoding item: %v", err)
		return
	}
	b := buf.Bytes()
*/

	items := []*memcache.Item{}
	for i:=0; i<len(b); i+=chunksize {
		k := fmt.Sprintf("=%d=%s",i,key)
		s,e := i, i+chunksize-1
		if e>=len(b) { e = len(b)-1 }
		c.Infof(" #=== [%7d, %7d] (%d) %s", s, e, len(b), k)
		items = append(items, &memcache.Item{ Key:k , Value:b[s:e+1] }) // slice sytax is [s,e)
	}

	if err := memcache.SetMulti(c, items); err != nil {
		c.Errorf(" #=== cdb sharded store fail: %v", err)
	}

	c.Infof(" #=== Stored '%s' (len=%d)!", key, len(b))
}

// }}}

// {{{ cdb.getMaybeCachedComplaintsByQuery

type memResults struct {
	Keys []*datastore.Key
	Vals []types.Complaint
}

func (cdb ComplaintDB)getMaybeCachedComplaintsByQuery(q *datastore.Query, memKey string) ([]*datastore.Key, []types.Complaint, error) {

	if cdb.Memcache && memKey != "" {
		if b,found := BytesFromShardedMemcache(cdb.C, memKey); found == true {
			buf := bytes.NewBuffer(b)
			results := memResults{}
			if err := gob.NewDecoder(buf).Decode(&results); err != nil {
				cdb.C.Errorf("cdb memcache multiget decode: %v", err)
			} else {
				cdb.C.Infof(" #=== Found all items ? Considered cache hit !")
				return results.Keys, results.Vals, nil
			}
		}
	}

	var data = []types.Complaint{}
	//cdb.C.Infof(" #=== Fetching[%s] from DS :(", memKey)

	//tolerantContext := appengine.Timeout(cdb.C, 30*time.Second)  // Default context has a 5s timeout

	keys, err := q.GetAll(cdb.C, &data)
	if err != nil { return nil, nil, err }

	if cdb.Memcache && memKey != "" {
		var buf bytes.Buffer
		dataToCache := memResults{Keys:keys, Vals:data}
		if err := gob.NewEncoder(&buf).Encode(dataToCache); err != nil {
			cdb.C.Errorf(" #=== cdb error encoding item: %v", err)
		} else {
			b := buf.Bytes()
			BytesToShardedMemcache(cdb.C, memKey, b)
		}
	}

	return keys, data, nil
}

// }}}
// {{{ cdb.getComplaintsByQuery

func (cdb ComplaintDB)getComplaintsByQuery(q *datastore.Query, memKey string) ([]types.Complaint, error) {
	keys,complaints,err := cdb.getMaybeCachedComplaintsByQuery(q,memKey)
	if err != nil { return nil, err}
	
	// Data fixups !
	for i, _ := range complaints {
		FixupComplaint(&complaints[i], keys[i])
	}

	sort.Sort(types.ComplaintsByTimeDesc(complaints))

	return complaints, nil
}

// }}}

// {{{ cdb.GetComplaintsByEmailAddress

func (cdb ComplaintDB) GetComplaintsByEmailAddress(ea string) ([]types.Complaint, error) {
	q := datastore.
		NewQuery(kComplaintKind).
		Ancestor(cdb.emailToRootKey(ea)).
		Order("Timestamp")

	cdb.C.Infof(" ##== all-comp")

	return cdb.getComplaintsByQuery(q,"")
}

// }}}
// {{{ cdb.GetComplaintsInSpanByEmailAddress

func (cdb ComplaintDB) GetComplaintsInSpanByEmailAddress(ea string, start,end time.Time) ([]types.Complaint, error) {

	//cdb.C.Infof(" ##== comp-in-span [%s  -->  %s]", start, end)
	memKey := ""
	todayStart,_ := date.WindowForToday()
	if (end.Before(todayStart) || end.Equal(todayStart)) {
		memKey = fmt.Sprintf("comp-in-span:%s:%d-%d", ea, start.Unix(), end.Unix())
		//cdb.C.Infof(" ##== comp-in-span cacheable [%s]", memKey)
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
	cdb.C.Infof(" ##== comp-in-span [%s  -->  %s]", start, end)
	memKey := ""
	todayStart,_ := date.WindowForToday()
	if (end.Before(todayStart) || end.Equal(todayStart)) {
		memKey = fmt.Sprintf("comp-in-span:__all__:%d-%d", start.Unix(), end.Unix())
		//cdb.C.Infof(" ##== comp-in-span cacheable [%s]", memKey)
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
	if _,err := q.GetAll(cdb.C, &data); err != nil {
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

// {{{ cdb.GetAnyComplaintByKey

func (cdb ComplaintDB) GetAnyComplaintByKey(keyString string) (*types.Complaint, error) {
	k,err := datastore.DecodeKey(keyString)
	if err != nil { return nil,err }

	complaint := types.Complaint{}
	if err := datastore.Get(cdb.C, k, &complaint); err != nil {
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
	if k.Parent().StringID() != ownerEmail {
		return nil,fmt.Errorf("Get: key <%v> owned by %s, not %s", k, k.Parent().StringID(), ownerEmail)
	}

	complaint := types.Complaint{}
	if err := datastore.Get(cdb.C, k, &complaint); err != nil {
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

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
