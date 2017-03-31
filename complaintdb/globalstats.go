package complaintdb

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"sort"
	"time"

	"github.com/skypies/util/date"
	"github.com/skypies/util/dsprovider"
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

// {{{ cdb.DeleteAllGlobalStats

func (cdb ComplaintDB)DeletAllGlobalStats() error {
	fgs := []FrozenGlobalStats{}

	//q := datastore.NewQuery(kGlobalStatsKind).KeysOnly()
	//if keys,err := q.GetAll(cdb.Ctx(), &fgs); err != nil {
	q := cdb.NewQuery(kGlobalStatsKind).KeysOnly()
	dsq := (*dsprovider.Query)(q)
	if keyers,err := cdb.Provider.GetAll(cdb.Ctx(), dsq, &fgs); err != nil {
		return err
	} else {
		return cdb.Provider.DeleteMulti(cdb.Ctx(), keyers)
	}
}

// }}}
// {{{ cdb.SaveGlobalStats

func (cdb ComplaintDB)SaveGlobalStats(gs GlobalStats) error {
	if gs.DatastoreKey == "" {
		return fmt.Errorf("SaveGlobalStats: no .DatstoreKey")
	}
	//key,_ := datastore.DecodeKey(gs.DatastoreKey)
	keyer,_ := cdb.Provider.DecodeKey(gs.DatastoreKey)
	
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(gs); err != nil { return err }
	_,err := cdb.Provider.Put(cdb.Ctx(), keyer, &FrozenGlobalStats{Bytes: buf.Bytes()} )

	return err
}

// }}}
// {{{ cdb.LoadGlobalStats

func (cdb ComplaintDB)LoadGlobalStats() (*GlobalStats, error) {
	gs := GlobalStats{}

	fgs := []FrozenGlobalStats{}
	//q := datastore.NewQuery(kGlobalStatsKind).Limit(10)
	//if keys, err := q.GetAll(cdb.Ctx(), &fgs); err != nil {

	q := cdb.NewQuery(kGlobalStatsKind).Limit(10)
	dsq := (*dsprovider.Query)(q)
	if keyers,err := cdb.Provider.GetAll(cdb.Ctx(), dsq, &fgs); err != nil {
		return nil, err
	} else if len(fgs) != 1 {
		return nil, fmt.Errorf("LoadGlobalStats: found %d, expected 1", len(fgs))
	} else {
		buf := bytes.NewBuffer(fgs[0].Bytes)
		err := gob.NewDecoder(buf).Decode(&gs)
		gs.DatastoreKey = keyers[0].Encode()  // Store this, so we can overwrite

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
		cdb.Errorf("Reset/DeleteAll fail, %v", err)
		return
	}

	profiles, err := cdb.LookupAllProfiles(cdb.NewProfileQuery())
	if err != nil { return }

	// Upon reset (writing a fresh new singleton), we need to generate a key
	rootKey := cdb.Provider.NewNameKey(cdb.Ctx(), kGlobalStatsKind, "foo", nil)
	keyer := cdb.Provider.NewIncompleteKey(cdb.Ctx(), kGlobalStatsKind, rootKey)
	gs := GlobalStats{
		DatastoreKey: keyer.Encode(),
	}
	
	// This is too slow to recalculate this way; it runs into the 10m timeout
	//start := date.ArbitraryDatestring2MidnightPdt("2015/08/09", "2006/01/02").Add(-1 * time.Second)
	start := date.ArbitraryDatestring2MidnightPdt("2016/03/15", "2006/01/02").Add(-1 * time.Second)
	end,_ := date.WindowForYesterday()  // end is the final day we count for; yesterday

	midnights := date.IntermediateMidnights(start, end.Add(time.Minute))
	for _,m := range midnights {
		dayStart,dayEnd := date.WindowForTime(m)

		dc := DailyCount{Datestring: date.Time2Datestring(dayStart)}
		
		for _,p := range profiles {
			q := cdb.CQByEmail(p.EmailAddress).ByTimespan(dayStart,dayEnd)
			if keyers,err := cdb.LookupAllKeys(q); err != nil {
				cdb.Errorf("Reset/Lookup fail, %v", err)
			} else if len(keyers) > 0 {
				dc.NumComplaints += len(keyers)
				dc.NumComplainers += 1
			}
		}
		gs.Counts = append(gs.Counts, dc)
	}

	if err := cdb.SaveGlobalStats(gs); err != nil {
		cdb.Errorf("Reset/Save fail, %v", err)		
	}
}

// }}}
	
// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
