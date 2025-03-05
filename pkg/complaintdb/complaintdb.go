package complaintdb

import (
	"fmt"
	pkglog "log"
	"net/http"
	"os"
	"sort"
	"time"

	"golang.org/x/net/context"

	"github.com/skypies/util/gcp/ds"
)

var(
	kComplaintKind = "ComplaintKind"
	kComplainerKind = "ComplainerKind"
	KMaxComplaintsPerDay = 200
)

// {{{ ComplaintDB{}, NewDB(), cdb.Ctx(), cdb.HTTPClient()

// ComplaintDB is a transient handle to the database
type ComplaintDB struct {
	ctx       context.Context
	StartTime time.Time
	admin     bool
	Provider  ds.DatastoreProvider
	Logger   *pkglog.Logger
}
func (cdb ComplaintDB)Ctx() context.Context { return cdb.ctx }
func (cdb ComplaintDB)HTTPClient() *http.Client { return &http.Client{} }

func NewDB(ctx context.Context) ComplaintDB {
	props,propsOk := GetContextProperties(ctx)

	projectId := "serfr0-1000"
	if propsOk && props.ProjectId != "" {
		projectId = props.ProjectId
	}
	
	if p,err := ds.NewCloudDSProvider(ctx, projectId); err != nil {
		panic(fmt.Errorf("NewDB: could not get a clouddsprovider (projectId=%s): %v\n", projectId, err))

	} else {
		return ComplaintDB{
			ctx: ctx,
			StartTime: time.Now(),
			admin: (propsOk && props.IsAdmin==true),
			Provider: p,
			Logger: pkglog.New(os.Stderr, "", pkglog.Ldate|pkglog.Ltime), //|log.Lshortfile)
		}
	}
}

// }}}

// {{{ cdb.Debugf

// Debugf is has a 'step' arg, and adds its own latency timings
func (cdb ComplaintDB)Debugf(step string, fmtstr string, varargs ...interface{}) {

	return // THIS IS WHY ... DebugF is sinkholed

	payload := fmt.Sprintf(fmtstr, varargs...)
	str := fmt.Sprintf("[%s] %9.6f %s", step, time.Since(cdb.StartTime).Seconds(), payload)
	if cdb.Logger != nil {
		cdb.Logger.Print(str)
	} else {
		fmt.Printf(fmtstr, varargs...)
	}
}

func (cdb ComplaintDB)Infof(fmtstr string, varargs ...interface{}) {
	if cdb.Logger != nil {
		cdb.Logger.Printf(fmtstr, varargs...)
	} else {
		fmt.Printf(fmtstr, varargs...)
	}
}

func (cdb ComplaintDB)Errorf(fmtstr string, varargs ...interface{}) {
	if cdb.Logger != nil {
		cdb.Logger.Printf("ERROR: "+fmtstr, varargs...)
	} else {
		fmt.Printf(fmtstr, varargs...)
	}
}

// }}}

// {{{ cdb.emailToRootKeyer

func (cdb ComplaintDB)emailToRootKeyer(email string) ds.Keyer {
	return cdb.Provider.NewNameKey(cdb.Ctx(), kComplainerKind, email, nil)
}

// }}}
// {{{ cdb.findOrGenerateComplaintKeyer

func (cdb ComplaintDB)findOrGenerateComplaintKeyer(c Complaint) (ds.Keyer, error) {
	if c.DatastoreKey != "" {
		return cdb.Provider.DecodeKey(c.DatastoreKey)
	}

	// The obj must be brand new - so new key
	rootKeyer := cdb.emailToRootKeyer(c.Profile.EmailAddress)
	keyer := cdb.Provider.NewIncompleteKey(cdb.Ctx(), kComplaintKind, rootKeyer)
	return keyer,nil
}

// }}}

// {{{ cdb.ComplaintKeyOwnedBy

// We need to assert this in a few places
func (cdb ComplaintDB)ComplaintKeyOwnedBy(keyer ds.Keyer, owner string) (bool,error) {
	parentKeyer := cdb.Provider.KeyParent(keyer)
	if parentKeyer == nil {
		// Insist on a parent, else we can't do owner checks
		return false, fmt.Errorf("LookupKey: key <%v> had no parent", keyer)
	}

	if owner != "" && !cdb.admin && cdb.Provider.KeyName(parentKeyer) != owner {
		return false,fmt.Errorf("LookupKey: key <%v> owned by %s, not %s",
			keyer, cdb.Provider.KeyName(parentKeyer), owner)
	}

	return true, nil
}

func (cdb ComplaintDB)ComplaintKeyStrOwnedBy(keyStr, owner string) (bool,error) {
	keyer,err := cdb.Provider.DecodeKey(keyStr)
	if err != nil {
		return false, fmt.Errorf("LookupKey: %v", err)
	}
	return cdb.ComplaintKeyOwnedBy(keyer, owner)
}

// }}}

// {{{ cdb.GetKeyer

func (cdb ComplaintDB)GetKeyerOrNil(c Complaint) ds.Keyer {
	if c.DatastoreKey != "" {
		keyer, err := cdb.Provider.DecodeKey(c.DatastoreKey)
		if err == nil {
			return keyer
		}
	}
	return nil
}

// }}}
// {{{ cdb.PersistComplaint

func (cdb ComplaintDB)PersistComplaint(c Complaint) error {
	keyer,err := cdb.findOrGenerateComplaintKeyer(c)
	if err != nil {
		return fmt.Errorf("PersistComplaint/findKey: %v", err)
	}

	if _,err := cdb.Provider.Put(cdb.Ctx(), keyer, &c); err != nil {
		return fmt.Errorf("PersistComplaint/Put: %v", err)
	}

	return nil
}

// }}}
// {{{ cdb.PersistComplaints

func (cdb ComplaintDB)PersistComplaints(complaints []Complaint) error {
	keyers := make([]ds.Keyer, len(complaints))
	for i,c := range complaints {
		keyer,err := cdb.findOrGenerateComplaintKeyer(c)
		if err != nil {
			return fmt.Errorf("PersistComplaints/findKey: %v", err)
		}
		keyers[i] = keyer
	}

	if _,err := cdb.Provider.PutMulti(cdb.Ctx(), keyers, complaints); err != nil {
		return fmt.Errorf("PersistComplaints/PutMulti: %v (%d,%d)", err,
			len(keyers), len(complaints))
	}

	return nil
}

// }}}
// {{{ cdb.LookupKey

// If owner is non-empty, return error if the looked-up key doesn't have that owner. Unless
// the admin flag is set on the DB handle.
func (cdb ComplaintDB)LookupKey(keyerStr string, owner string) (*Complaint, error) {
	keyer,err := cdb.Provider.DecodeKey(keyerStr)
	if err != nil {
		return nil, fmt.Errorf("LookupKey/DecodeKey: %v", err)
	}

	if _,err := cdb.ComplaintKeyOwnedBy(keyer, owner); err != nil {
		return nil, fmt.Errorf("LookupKey: ACL failure: %v\n", err)
	}

	c := Complaint{}
	if err := cdb.Provider.Get(cdb.Ctx(), keyer, &c); err != nil {
		return nil, fmt.Errorf("LookupKey/Get: %v", err)
	}
	
	FixupComplaint(&c, keyer.Encode())

	return &c, nil
}

// }}}
// {{{ cdb.RawLookupAll

func (cdb ComplaintDB)RawLookupAll(cq *CQuery) ([]Complaint, error) {
	complaints := []Complaint{}

	cdb.Debugf("cdbRLA_201", "calling GetAll() ...")
	_, err := cdb.Provider.GetAll(cdb.Ctx(), (*ds.Query)(cq), &complaints)
	cdb.Debugf("cdbRLA_202", "... call done (n=%d)", len(complaints))

	// We tolerate missing fields, because the DB is full of old objects with dead fields
	if err != nil && err != ds.ErrFieldMismatch {
		return nil, fmt.Errorf("cdbRLA: %v", err)
	}

	return complaints,nil
}

// }}}
// {{{ cdb.LookupAll

func (cdb ComplaintDB)LookupAll(cq *CQuery) ([]Complaint, error) {
	complaints := []Complaint{}

	cdb.Debugf("cdbLA_201", "calling GetAll() ...")
	keyers, err := cdb.Provider.GetAll(cdb.Ctx(), (*ds.Query)(cq), &complaints)
	cdb.Debugf("cdbLA_202", "... call done (n=%d)", len(keyers))

	// We tolerate missing fields, because the DB is full of old objects with dead fields
	if err != nil && err != ds.ErrFieldMismatch {
		return nil, fmt.Errorf("cdbLA: %v", err)
	}
	
	for i,_ := range complaints {
		FixupComplaint(&complaints[i], keyers[i].Encode())
	}

	sort.Sort(ComplaintsByTimeDesc(complaints))

	return complaints, nil
}

// }}}
// {{{ cdb.LookupFirst

func (cdb ComplaintDB)LookupFirst(cq *CQuery) (*Complaint, error) {
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

func (cdb ComplaintDB)LookupAllKeys(cq *CQuery) ([]ds.Keyer, error) {
	q := (*ds.Query)(cq)
	return cdb.Provider.GetAll(cdb.Ctx(), q.KeysOnly(), nil)
}

// }}}
// {{{ cdb.DeleteByKey

func (cdb ComplaintDB)DeleteByKey(keyer ds.Keyer) error {
	return cdb.Provider.Delete(cdb.Ctx(), keyer)
}

// }}}
// {{{ cdb.DeleteAllKeys

/*
			// May need to make multiple calls
			maxKeyersToDeleteInOneCall := 500
			for len(keyers) > 0 {
				keyersToDelete := []ds.Keyer{}
				if len(keyers) <= maxKeyersToDeleteInOneCall {
					keyersToDelete, keyers = keyers, keyersToDelete
				} else {
					keyersToDelete, keyers = keyers[0:maxKeyersToDeleteInOneCall], keyers[maxKeyersToDeleteInOneCall:]
				}
				if err := cdb.DeleteAllKeys(keyersToDelete); err != nil {
					log.Fatal(err)
				}
*/

func (cdb ComplaintDB)DeleteAllKeys(keyers []ds.Keyer) error {
	return cdb.Provider.DeleteMulti(cdb.Ctx(), keyers)
}

// }}}

// {{{ cdb.PersistProfile

func (cdb ComplaintDB)PersistProfile(p ComplainerProfile) error {
	keyer := cdb.emailToRootKeyer(p.EmailAddress)
	if _,err := cdb.Provider.Put(cdb.Ctx(), keyer, &p); err != nil {
		return fmt.Errorf("PersistProfile/Put: %v", err)
	}
	return nil
}

// }}}
// {{{ cdb.[Must]LookupProfile

// If not found, returns an error
func (cdb ComplaintDB)MustLookupProfile(email string) (*ComplainerProfile, error) {
	profile := ComplainerProfile{}
	keyer := cdb.emailToRootKeyer(email)
	
	if err := cdb.Provider.Get(cdb.Ctx(), keyer, &profile); err != nil {
		return nil,err
	}

	return &profile,nil
}

// LookupProfile swallows the not-found error; and returns an empty profile on all errors.
func (cdb ComplaintDB)LookupProfile(email string) (*ComplainerProfile, error) {
	if p,err := cdb.MustLookupProfile(email); err == ds.ErrNoSuchEntity {
		return &ComplainerProfile{}, nil
	} else if err != nil {
		return &ComplainerProfile{}, fmt.Errorf("LookupProfile: %v", err)
	} else {
		return p, nil
	}
}

// }}}
// {{{ cdb.LookupAllProfiles

func (cdb ComplaintDB)LookupAllProfiles(cq *CQuery) ([]ComplainerProfile, error) {
	profiles := []ComplainerProfile{}

	cdb.Debugf("cdbLAP_201", "calling GetAll() ...")
	keyers, err := cdb.Provider.GetAll(cdb.Ctx(), (*ds.Query)(cq), &profiles)
	cdb.Debugf("cdbLAP_202", "... call done (n=%d)", len(keyers))

	return profiles,err
}

// }}}


/* TODO

1. Look in complaintdb/lookups.go - can this layer of logic live elsewhere ?

3. Move ./types/go into ./<type>.go ?

7. counts.go: rename to "usersummary" or something; make generation less magical, more explicit
7a. consider renaming the DailyCount{} struct
7b. house cdb.getDailyCounts somewhere with counts
8. globalstats.go: rename to "sitesummary" ? Add something for monthly totals ? (unqiue users:/)

10. Kill off the address inference stuff ?

12. Make use of that __datastorekey__ trick ?
13. Remove cdb.Ctx() ?
14. Kill off any memoization magic that is YAGNI

 */

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
