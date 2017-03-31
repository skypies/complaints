package complaintdb

import (
	"fmt"
	"sort"

	"google.golang.org/appengine/datastore"  // for ErrFieldMismatch

	"github.com/skypies/util/dsprovider"
	"github.com/skypies/complaints/complaintdb/types"
)

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

// If owner is non-empty, return error if the looked-up key doesn't have that owner. Unless
// the admin flag is set on the DB handle.
func (cdb ComplaintDB)LookupKey(keyerStr string, owner string) (*types.Complaint, error) {
	keyer,err := cdb.Provider.DecodeKey(keyerStr)
	if err != nil {
		return nil, fmt.Errorf("LookupKey: %v", err)
	}

	parentKeyer := cdb.Provider.KeyParent(keyer)
	if parentKeyer == nil {
		// Insist on a parent, else we can't do owner checks
		return nil, fmt.Errorf("LookupKey: key <%v> had no parent", keyer)
	}

	c := types.Complaint{}

	if err := cdb.Provider.Get(cdb.Ctx(), keyer, &c); err != nil {
		return nil, fmt.Errorf("LookupKey: %v", err)
	}

	if owner != "" && !cdb.admin && cdb.Provider.KeyName(parentKeyer) != owner {
		return nil,fmt.Errorf("LookupKey: key <%v> owned by %s, not %s",
			keyer, cdb.Provider.KeyName(parentKeyer), owner)
	}
	
	FixupComplaint(&c, keyer.Encode())

	return &c, nil
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
// {{{ cdb.LookupFirst

func (cdb ComplaintDB)LookupFirst(cq *CQuery) (*types.Complaint, error) {
	if complaints,err := cdb.LookupAll(cq.Limit(1)); err != nil {
		return nil, err
	} else if len(complaints) == 0 {
		return nil, nil
	} else {
		return &complaints[0], nil
	}
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

0. Kill off profilelookups (reconsider ComplaintsAndProfile ?)
1. Do some kind of cdb.UpdateComplaint, maybe; not sure what the logic is there.

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
