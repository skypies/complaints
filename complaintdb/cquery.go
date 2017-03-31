package complaintdb

// This package contains complaint query builders

import(
	"time"
	ds "github.com/skypies/util/dsprovider"
	//"github.com/skypies/complaints/complaintdb/types"
)

type CQuery ds.Query // Create our own type, so we can hang a fluent API off it

func (cdb *ComplaintDB)NewComplaintQuery() *CQuery { return NewComplaintQuery() }
func (cdb *ComplaintDB)NewComplainerQuery() *CQuery { return NewComplainerQuery() }

func NewComplaintQuery() *CQuery { return (*CQuery)(ds.NewQuery(kComplaintKind)) }
func NewComplainerQuery() *CQuery { return (*CQuery)(ds.NewQuery(kComplainerKind)) }

// Thin wrapper in util/dsprovider.Query
func (cq *CQuery)Order(str string) *CQuery { return (*CQuery)((*ds.Query)(cq).Order(str)) }
func (cq *CQuery)Limit(val int) *CQuery { return (*CQuery)((*ds.Query)(cq).Limit(val)) }
func (cq *CQuery)Filter(str string, val interface{}) *CQuery {
	return (*CQuery)((*ds.Query)(cq).Filter(str,val))
}
func (cq *CQuery)Ancestor(rootkey ds.Keyer) *CQuery {
	return (*CQuery)((*ds.Query)(cq).Ancestor(rootkey))
}
func (cq *CQuery)Project(fields ...string) *CQuery {
	return (*CQuery)((*ds.Query)(cq).Project(fields...))
}

// Query builders
//
func (cq *CQuery)ByTimespan(start,end time.Time) *CQuery {
	return cq.
		Filter("Timestamp >= ", start).
		Filter("Timestamp < ", end).
		Order("Timestamp") // This is a side effect; not sure if I like hardwiring it in here
}

func (cq *CQuery)ByZip(zip string) *CQuery {
	return cq.Filter("Profile.StructuredAddress.Zip = ", zip)
}


// Canned queries
//
func (cdb ComplaintDB)CQueryByEmailAddress(email string) *CQuery {
	return NewComplaintQuery().
		Ancestor(cdb.EmailToRootKeyer(email)).
		Order("Timestamp").
		Limit(-1)
}
