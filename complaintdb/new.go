package complaintdb

import (
	"fmt"
	"sort"

	"google.golang.org/appengine/datastore"  // for ErrFieldMismatch :/

	"github.com/skypies/util/dsprovider"
	"github.com/skypies/complaints/complaintdb/types"
)

// New API
//
// {{{ cdb.PersistComplaint

func (cdb ComplaintDB)PersistComplaint(c types.Complaint) error {
	rootKeyer := cdb.EmailToRootKeyer(c.Profile.EmailAddress)
	keyer := cdb.Provider.NewIncompleteKey(cdb.Ctx(), kComplaintKind, rootKeyer)
	if _,err := cdb.Provider.Put(cdb.Ctx(), keyer, &c); err != nil {
		return fmt.Errorf("PersistComplaint/Put: %v", err)
	}
	return nil
}

// }}}
// {{{ cdb.LookupKey

func (cdb ComplaintDB)LookupKey(keyerStr string) (*types.Complaint, error) {
	keyer,err := cdb.Provider.DecodeKey(keyerStr)
	if err != nil {
		return nil, fmt.Errorf("LookupKey: %v", err)
	}

	c := types.Complaint{}

	if err := cdb.Provider.Get(cdb.Ctx(), keyer, &c); err != nil {
		return nil, fmt.Errorf("LookupKey: %v", err)
	}

	FixupComplaint(&c, keyer.Encode())

	return &c,nil
}

// }}}
// {{{ cdb.LookupAll

func (cdb ComplaintDB)LookupAll(cq *CQuery) ([]types.Complaint, error) {
	complaints := []types.Complaint{}

	cdb.Debugf("cdbLA_201", "calling GetAll() ...")
	keyers, err := cdb.Provider.GetAll(cdb.Ctx(), (*dsprovider.Query)(cq), &complaints)
	cdb.Debugf("cdbLA_202", "... call done (n=%d)", len(keyers))

	// We tolerate missing fields, because the DB is full of old objects with dead fields
	if err != nil {
		if _,assertionOk := err.(*datastore.ErrFieldMismatch); !assertionOk {
			return nil, fmt.Errorf("cdbLA: %v", err)
		}
	}

	for i, _ := range complaints {
		FixupComplaint(&complaints[i], keyers[i].Encode())
	}

	sort.Sort(types.ComplaintsByTimeDesc(complaints))

	return complaints,nil
}

// }}}
// {{{ cdb.LookupAllKeys

func (cdb ComplaintDB)LookupAllKeys(cq *CQuery) ([]dsprovider.Keyer, error) {
	q := (*dsprovider.Query)(cq)
	return cdb.Provider.GetAll(cdb.Ctx(), q.KeysOnly(), nil)
}

// }}}
// {{{ cdb.DeleteByKey

func (cdb ComplaintDB)DeleteByKey(keyer dsprovider.Keyer) error {
	return cdb.Provider.Delete(cdb.Ctx(), keyer)
}

// }}}
// {{{ cdb.DeleteAllKeys

func (cdb ComplaintDB)DeleteAllKeys(keyers []dsprovider.Keyer) error {
	return cdb.Provider.DeleteMulti(cdb.Ctx(), keyers)
}

// }}}


/* TODO

2. remove memcache.go (if required, use gaeutil/)
3. Move ./types/types.go into ./<type>.go ?
5. Rewrite queries.go to sit on top of the basic impl
6. Retire most of complaintlookups; call sites should compose queries

7. counts.go: rename to "usersummary" or something; make generation less magical, more explicit
7a. consider renaming the DailyCount{} struct
7b. house cdb.getDailyCounts somewhere with counts
8. globalstats.go: rename to "sitesummary" ? Add something for monthly totals ? (unqiue users:/)

9. Look at all the primary functions in complaintdb.go; grep them; can we retire/generalize ?

10. Kill off the address inference stuff ?
11. Kill off kComplaintVersion
12. Make use of that __datastorekey__ trick ?
13. Remove cdb.Ctx() ?
14. Kill off any memoization magic that is YAGNI

 */



// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
