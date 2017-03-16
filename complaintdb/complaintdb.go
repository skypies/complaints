package complaintdb

import (
	"fmt"
	"net/http"
	"time"

	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/urlfetch"
	"golang.org/x/net/context"

	"github.com/skypies/util/date"

	"github.com/skypies/complaints/complaintdb/types"
)

var(
	kComplaintKind = "ComplaintKind"
	kComplainerKind = "ComplainerKind"
	KMaxComplaintsPerDay = 200
)

// {{{ ComplaintDB{}, NewDB(), cdb.Ctx(), cdb.HTTPClient()

// ComplaintDB is a transient handle to the database
type ComplaintDB struct {
	ctx context.Context
	StartTime time.Time
}
func (cdb ComplaintDB)Ctx() context.Context { return cdb.ctx }
func (cdb ComplaintDB)HTTPClient() *http.Client { return urlfetch.Client(cdb.Ctx()) }

func NewDB(ctx context.Context) ComplaintDB {
	return ComplaintDB{
		ctx: ctx,
		StartTime: time.Now(),
	}
}

// }}}

// {{{ cdb.Debugf

// Debugf is has a 'step' arg, and adds its own latency timings
func (cdb ComplaintDB)Debugf(step string, fmtstr string, varargs ...interface{}) {
	payload := fmt.Sprintf(fmtstr, varargs...)
	log.Debugf(cdb.Ctx(), "[%s] %9.6f %s", step, time.Since(cdb.StartTime).Seconds(), payload)
}

func (cdb ComplaintDB)Infof(fmtstr string, varargs ...interface{}) {
	log.Infof(cdb.Ctx(), fmtstr, varargs...)
}

func (cdb ComplaintDB)Errorf(fmtstr string, varargs ...interface{}) {
	log.Errorf(cdb.Ctx(), fmtstr, varargs...)
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

// {{{ cdb.EmailToRootKey

func (cdb ComplaintDB) emailToRootKey(email string) *datastore.Key {
	return datastore.NewKey(cdb.Ctx(), kComplainerKind, email, 0, nil)
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
	_, err = q.GetAll(cdb.Ctx(), &cps)
	return
}

// }}}
// {{{ cdb.TouchAllProfiles

// Does a Get and a Put on all the profile objects. This seems to be necessary to fully
// undo the historic effects of a `datastore=noindex`.
func (cdb ComplaintDB) TouchAllProfiles() (int,error) {
	profiles, err := cdb.GetAllProfiles()
	if err != nil {
		return 0,err
	}

	for i,cp := range profiles {
		if err := cdb.PutProfile(cp); err != nil {
			return i,err
		}
	}

	return len(profiles), nil
}

// }}}
// {{{ cdb.GetEmailCityMap

func (cdb ComplaintDB) GetEmailCityMap() (map[string]string, error) {
	cities := map[string]string{}

	q := datastore.NewQuery(kComplainerKind).Project("EmailAddress", "StructuredAddress.City")
	profiles := []types.ComplainerProfile{}
	if _,err := q.GetAll(cdb.Ctx(), &profiles); err != nil {
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
	return datastore.DeleteMulti(cdb.Ctx(), keys)
}

// }}}

// {{{ cdb.GetComplainersCurrentlyOptedOut

func (cdb ComplaintDB)GetComplainersCurrentlyOptedOut() (map[string]int, error) {
	q := datastore.
		NewQuery(kComplainerKind).
		Project("EmailAddress").
		Filter("DataSharing =", -1).
		Limit(-1)

	var data = []types.ComplainerProfile{}
	if _,err := q.GetAll(cdb.Ctx(), &data); err != nil {
		return map[string]int{}, err
	}
	
	ret := map[string]int{}
	for _,cp := range data {
		ret[cp.EmailAddress]++
	}
	
	return ret, nil
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
	if _,err := q.GetAll(cdb.Ctx(), &data); err != nil {
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

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
