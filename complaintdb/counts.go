package complaintdb

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"sort"
	"time"

	"github.com/skypies/util/date"
	"github.com/skypies/util/gaeutil"

	newappengine "google.golang.org/appengine"
	newdatastore "google.golang.org/appengine/datastore"
)

// {{{ DailyCount

type DailyCount struct {
	Datestring       string
	NumComplaints    int
	NumComplainers   int
	IsMaxComplaints  bool
	IsMaxComplainers bool
}
func (dc DailyCount)Timestamp() time.Time {
	return date.Datestring2MidnightPdt(dc.Datestring)
}

func (dc DailyCount)String() string {
	str := fmt.Sprintf("%s: % 4d complaints by % 3d people", dc.Datestring, dc.NumComplaints, dc.NumComplainers)

	if dc.IsMaxComplainers { str += " (max complainers!)" }
	if dc.IsMaxComplaints { str += " (max complaints!)" }

	return str
}

type DailyCountDesc []DailyCount
func (a DailyCountDesc) Len() int           { return len(a) }
func (a DailyCountDesc) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a DailyCountDesc) Less(i, j int) bool { return a[i].Datestring > a[j].Datestring}

// }}}

// {{{ cdb.fetchDailyCountSingleton

// 	might return: datastore.ErrNoSuchEntity
func  (cdb *ComplaintDB)fetchDailyCountSingleton(key string) ([]DailyCount, error) {
	newctx := newappengine.NewContext(cdb.Req)

	if data,err := gaeutil.LoadSingletonFromDatastore(newctx, key); err == nil {
		buf := bytes.NewBuffer(data)
		dcs := []DailyCount{}
		if err := gob.NewDecoder(buf).Decode(&dcs); err != nil {
			// Log, but swallow this error
			cdb.C.Errorf("fetchDailyCountSingleton: could not decode: %v", err)
		}
		return dcs,nil
	} else {
		return []DailyCount{}, err
	}
}

// }}}
// {{{ cdb.putDailyCountSingleton

func  (cdb *ComplaintDB)putDailyCountSingleton(key string, dcs []DailyCount) error {
	newctx := newappengine.NewContext(cdb.Req)

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(dcs); err != nil {
		return fmt.Errorf("putDailyCountSingleton encode: %v", err)
	} else if err := gaeutil.SaveSingletonToDatastore(newctx, key, buf.Bytes()); err != nil {
		return fmt.Errorf("putDailyCountSingleton save: %v", err)
	}
	return nil
}

// }}}

// {{{ GetDailyCounts

func (cdb *ComplaintDB) GetDailyCounts(email string) ([]DailyCount, error) {
	// cdb.C.Infof("-- GetDaily for %s", email)

	k := fmt.Sprintf("%s:dailycounts", email)  // The 'counts' is so we can have diff memcache objects
	c := []DailyCount{}

	cdb.Debugf("GDC_001", "GetDailyCounts() starting")

	// 	might return: datastore.ErrNoSuchEntity
	if dcs,err := cdb.fetchDailyCountSingleton(k); err == newdatastore.ErrNoSuchEntity {
		// Singleton not found; we don't care; treat same as empty singleton.
	} else if err != nil {
    cdb.C.Errorf("error getting item: %v", err)
		return c, err
	} else {
		c = dcs
	}
	cdb.Debugf("GDC_002", "memcache lookup done (%d entries)", len(c))

	end,_ := date.WindowForYesterday()  // end is the final day we count for; yesterday
	start := end  // by default, this will trigger no lookups (start=end means no missing)

	if len(c) > 0 {
		start = date.Datestring2MidnightPdt(c[0].Datestring)
	} else {
		cdb.Debugf("GDC_003", "counts empty ! track down oldest every, to start iteration range")
		if complaint,err := cdb.GetOldestComplaintByEmailAddress(email); err != nil {
			cdb.C.Errorf("error looking up first complaint for %s: %v", email, err)
			return c, err
		} else if complaint != nil {
			// We move a day into the past; the algo below assumes we have data for the day 'start',
			// but in this case we don't; so trick it into generating data for today.
			start = date.AtLocalMidnight(complaint.Timestamp).AddDate(0,0,-1)
			//cdb.C.Infof("  - lookup first ever, %s", complaint.Timestamp)
		} else {
			// cdb.C.Infof("  - lookup first ever, but empty\n")
		}
		cdb.Debugf("GDC_004", "start point found")
	}

	// Right after the first complaint: it set start to "now", but end is still yesterday.
	if start.After(end) {
		// cdb.C.Infof("--- s>e {%s} > {%s}\n", start, end)	
		return c, nil
	}
	
	// We add a minute, to ensure that the day that contains 'end' is included
	missing := date.IntermediateMidnights(start, end.Add(time.Minute))
	// cdb.C.Infof("--- missing? --- {%s} -> {%s} == %d\n", start, end.Add(time.Minute), len(missing))	
	if len(missing) > 0 {
		for _,m := range missing {
			cdb.Debugf("GDC_005", "looking up a single span")
			dayStart,dayEnd := date.WindowForTime(m)
			if comp, err := cdb.GetComplaintsInSpanByEmailAddress(email, dayStart, dayEnd); err!=nil {
				return []DailyCount{}, err
			} else {
				// cdb.C.Infof("  -  {%s}  n=%d [%v]\n", dayStart, len(comp), m)
				c = append(c, DailyCount{date.Time2Datestring(dayStart),len(comp),1,false,false})
			}
		}
		sort.Sort(DailyCountDesc(c))

		// Now push back into memcache
		if err := cdb.putDailyCountSingleton(k,c); err != nil {
			cdb.C.Errorf("error storing counts singleton item: %v", err)
		}
	}
	cdb.Debugf("GDC_006", "all done")
	
	// cdb.C.Infof("--- done")	
	return c,nil
}

// }}}

/*
// {{{ GetDailyCounts

// http://stackoverflow.com/questions/13264555/store-an-object-in-memcache-of-gae-in-go
func (cdb *ComplaintDB) GetDailyCounts(email string) ([]DailyCount, error) {
	// cdb.C.Infof("-- GetDaily for %s", email)

	k := fmt.Sprintf("%s:daily", email)
	c := []DailyCount{}
	
	cdb.Debugf("GDC_001", "GetDailyCounts() starting")
	if _,err := memcache.Gob.Get(cdb.C, k, &c); err == memcache.ErrCacheMiss {
    // cache miss, but we don't care
	} else if err != nil {
    cdb.C.Errorf("error getting item: %v", err)
		return c, err
	}
	cdb.Debugf("GDC_002", "memcache lookup done")

	end,_ := date.WindowForYesterday()  // end is the final day we count for; yesterday
	start := end  // by default, this will trigger no lookups (start=end means no missing)

	if len(c) > 0 {
		start = date.Datestring2MidnightPdt(c[0].Datestring)
	} else {
		cdb.Debugf("GDC_003", "counts empty ! track down oldest every, to start iteration range")
		if complaint,err := cdb.GetOldestComplaintByEmailAddress(email); err != nil {
			cdb.C.Errorf("error looking up first complaint for %s: %v", email, err)
			return c, err
		} else if complaint != nil {
			// We move a day into the past; the algo below assumes we have data for the day 'start',
			// but in this case we don't; so trick it into generating data for today.
			start = date.AtLocalMidnight(complaint.Timestamp).AddDate(0,0,-1)
			//cdb.C.Infof("  - lookup first ever, %s", complaint.Timestamp)
		} else {
			// cdb.C.Infof("  - lookup first ever, but empty\n")
		}
		cdb.Debugf("GDC_004", "start point found")
	}

	// Right after the first complaint: it set start to "now", but end is still yesterday.
	if start.After(end) {
		// cdb.C.Infof("--- s>e {%s} > {%s}\n", start, end)	
		return c, nil
	}
	
	// We add a minute, to ensure that the day that contains 'end' is included
	missing := date.IntermediateMidnights(start, end.Add(time.Minute))
	// cdb.C.Infof("--- missing? --- {%s} -> {%s} == %d\n", start, end.Add(time.Minute), len(missing))	
	if len(missing) > 0 {
		for _,m := range missing {
			cdb.Debugf("GDC_005", "looking up a single span")
			dayStart,dayEnd := date.WindowForTime(m)
			if comp, err := cdb.GetComplaintsInSpanByEmailAddress(email, dayStart, dayEnd); err!=nil {
				return []DailyCount{}, err
			} else {
				// cdb.C.Infof("  -  {%s}  n=%d [%v]\n", dayStart, len(comp), m)
				c = append(c, DailyCount{date.Time2Datestring(dayStart),len(comp),1,false,false})
			}
		}
		sort.Sort(DailyCountDesc(c))

		// Now push back into memcache
		item := memcache.Item{Key:k, Object:c}
		if err := memcache.Gob.Set(cdb.C, &item); err != nil {
			cdb.C.Errorf("error setting item: %v", err)
		}
	}
	cdb.Debugf("GDC_006", "all done")
	
	// cdb.C.Infof("--- done")	
	return c,nil
}

// }}}
*/

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
