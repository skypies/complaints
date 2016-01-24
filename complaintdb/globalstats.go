package complaintdb

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"sort"
	"time"
	
	"appengine"
	"appengine/datastore"
	"appengine/memcache"

	"github.com/skypies/date"
)

const (
	kGlobalStatsKind = "GlobalStats"
	kMemcacheGlobalStatsKey = "singleton:globalstats"
)

type GlobalStats struct {
	DatastoreKey string
	Counts []DailyCount
}

type FrozenGlobalStats struct { Bytes []byte }

// {{{ ToMemcache

func (gs GlobalStats)ToMemcache(c appengine.Context) {
	item := memcache.Item{Key:kMemcacheGlobalStatsKey, Object:gs}
	if err := memcache.Gob.Set(c, &item); err != nil {
		c.Errorf("error setting item %s: %v", kMemcacheGlobalStatsKey, err)
	}
}

// }}}
// {{{ FromMemcache

func (gs *GlobalStats)FromMemcache(c appengine.Context) bool {
	if _,err := memcache.Gob.Get(c, kMemcacheGlobalStatsKey, gs); err == memcache.ErrCacheMiss {
    // cache miss, but we don't care
		return false
	} else if err != nil {
    c.Errorf("memcache error getting globalstats: %v", err)
		return false
	}
	//c.Infof("** Global stats from MC:")
	//for _,v := range gs.Counts { c.Infof(" * %s", v) }
	return true
}

// }}}

// {{{ cdb.DeleteAllGlobalStats

func (cdb ComplaintDB)DeletAllGlobalStats() error {
	fgs := []FrozenGlobalStats{}
	q := datastore.NewQuery(kGlobalStatsKind).KeysOnly()

	if keys,err := q.GetAll(cdb.C, &fgs); err != nil {
		return err
	} else {
		cdb.C.Infof("Found %d keys", len(keys))
		return datastore.DeleteMulti(cdb.C, keys)
	}
}

// }}}
// {{{ cdb.SaveGlobalStats

func (cdb ComplaintDB)SaveGlobalStats(gs GlobalStats) error {
	if gs.DatastoreKey == "" {
		return fmt.Errorf("SaveGlobalStats: no .DatstoreKey")
	}
	key,_ := datastore.DecodeKey(gs.DatastoreKey)
	
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(gs); err != nil { return err }
	_,err := datastore.Put(cdb.C, key, &FrozenGlobalStats{Bytes: buf.Bytes()} )

	//gs.ToMemcache(cdb.C)

	return err
}

// }}}
// {{{ cdb.LoadGlobalStats

func (cdb ComplaintDB)LoadGlobalStats() (*GlobalStats, error) {
	gs := GlobalStats{}
	//if gs.FromMemcache(cdb.C) { return &gs, nil }

	fgs := []FrozenGlobalStats{}
	q := datastore.NewQuery(kGlobalStatsKind).Limit(10)

	if keys, err := q.GetAll(cdb.C, &fgs); err != nil {
		return nil, err
	} else if len(fgs) != 1 {
		return nil, fmt.Errorf("LoadGlobalStats: found %d, expected 1", len(fgs))
	} else {
		buf := bytes.NewBuffer(fgs[0].Bytes)
		err := gob.NewDecoder(buf).Decode(&gs)
		gs.DatastoreKey = keys[0].Encode()  // Store this, so we can overwrite

		// Pick out the high water marks ...
		if len(gs.Counts) > 0 {
			iMaxComplaints,iMaxComplainers := 0,0
			maxComplaints,maxComplainers := 0,0
			for i,_ := range gs.Counts {
				if gs.Counts[i].NumComplaints > maxComplaints {
					iMaxComplaints,maxComplaints = i, gs.Counts[i].NumComplaints
				}
				if gs.Counts[i].NumComplainers > maxComplainers {
					iMaxComplainers,maxComplainers = i, gs.Counts[i].NumComplainers
				}
			}
			gs.Counts[iMaxComplaints].IsMaxComplaints = true
			gs.Counts[iMaxComplainers].IsMaxComplainers = true
		}
		
		//gs.ToMemcache(cdb.C)
		//cdb.C.Infof("** Global stats from DS:")
		//for _,v := range gs.Counts { cdb.C.Infof(" * %s", v) }
		return &gs,err
	}
}

// }}}

// {{{ cdb.AddDaily

func (cdb ComplaintDB)AddDailyCount(dc DailyCount) {
	if gs,err := cdb.LoadGlobalStats(); err != nil {
		return
	} else {
		gs.Counts = append(gs.Counts, dc)
		sort.Sort(DailyCountDesc(gs.Counts))	
		cdb.SaveGlobalStats(*gs)
	}
}

// }}}
// {{{ cdb.ResetGlobalStats

// This is unsafe - may cause an extra item to appear in the datastore.

func (cdb ComplaintDB)ResetGlobalStats() {
	if err := cdb.DeletAllGlobalStats(); err != nil {
		cdb.C.Errorf("Reset/DeleteAll fail, %v", err)
		return
	}

	profiles, err := cdb.GetAllProfiles()
	if err != nil { return }

	// Upon reset (writing a fresh new singleton), we need to generate a key
	rootKey := datastore.NewKey(cdb.C, kGlobalStatsKind, "foo", 0, nil)
	key := datastore.NewIncompleteKey(cdb.C, kGlobalStatsKind, rootKey)
	gs := GlobalStats{
		DatastoreKey: key.Encode(),
	}
	
	end,_ := date.WindowForYesterday()  // end is the final day we count for; yesterday
	start := end.AddDate(0,0,-100)
	midnights := date.IntermediateMidnights(start, end.Add(time.Minute))
	for _,m := range midnights {
		dayStart,dayEnd := date.WindowForTime(m)

		dc := DailyCount{Datestring: date.Time2Datestring(dayStart)}
		
		for _,p := range profiles {
			if comp,err := cdb.GetComplaintsInSpanByEmailAddress(p.EmailAddress, dayStart, dayEnd); err!=nil {
				cdb.C.Errorf("Reset/Lookup fail, %v", err)
			} else if len(comp) > 0 {
				dc.NumComplaints += len(comp)
				dc.NumComplainers += 1
			}
		}
		gs.Counts = append(gs.Counts, dc)
	}

	if err := cdb.SaveGlobalStats(gs); err != nil {
		cdb.C.Errorf("Reset/Save fail, %v", err)		
	}
	cdb.C.Infof("-- reset !--")
	//cdb.LoadGlobalStats();
}

// }}}
	
// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
