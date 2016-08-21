package complaintdb

import (
	"fmt"

	"google.golang.org/appengine/datastore"

	"github.com/skypies/complaints/complaintdb/types"
)

// {{{ cdb.GetProfileByButtonId

func (cdb ComplaintDB) GetProfileByButtonId(id string) (cp *types.ComplainerProfile, err error) {
	q := datastore.NewQuery(kComplainerKind).Filter("ButtonId =", id)
	var results = []types.ComplainerProfile{}
	_, err = q.GetAll(cdb.Ctx(), &results)
	if err != nil { return }
	if len(results) == 0 { return } // No match
	/*for _,v := range results {
		cdb.Infof(">>> RESULT: %v", v)
	}*/
	if len(results) > 1 {
		err = fmt.Errorf ("lookup(%s) found %d results", id, len(results))
		return
	}

	cp = &results[0]
	return 
}

// }}}
// {{{ cdb.GetProfileByCallerCode

func (cdb ComplaintDB) GetProfileByCallerCode(cc string) (cp *types.ComplainerProfile, err error) {
	q := datastore.NewQuery(kComplainerKind).Filter("CallerCode =", cc)
	var results = []types.ComplainerProfile{}
	_, err = q.GetAll(cdb.Ctx(), &results)
	if err != nil { return }
	if len(results) == 0 { return } // No match
	/*for _,v := range results {
		cdb.Infof(">>> RESULT: %v", v)
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
	err := datastore.Get(cdb.Ctx(), cdb.emailToRootKey(ea), &cp)
	return &cp, err
}

// }}}
// {{{ cdb.PutProfile

func (cdb ComplaintDB) PutProfile(cp types.ComplainerProfile) error {
	_, err := datastore.Put(cdb.Ctx(), cdb.emailToRootKey(cp.EmailAddress), &cp)
	return err
}

// }}}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
