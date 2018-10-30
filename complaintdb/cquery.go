package complaintdb

// This package contains complaint query builders

import(
	"time"
	ds "github.com/skypies/util/gcp/ds"
)

type CQuery ds.Query // Create our own type, so we can hang a fluent API off it

func NewQuery(kind string) *CQuery { return (*CQuery)(ds.NewQuery(kind)) }
func NewComplaintQuery() *CQuery { return NewQuery(kComplaintKind) }
func NewProfileQuery() *CQuery { return NewQuery(kComplainerKind) }

func (cdb *ComplaintDB)NewQuery(kind string) *CQuery { return NewQuery(kind) }
func (cdb *ComplaintDB)NewComplaintQuery() *CQuery { return NewComplaintQuery() }
func (cdb *ComplaintDB)NewProfileQuery() *CQuery { return NewProfileQuery() }

// Thin wrapper in util/dsprovider.Query
func (cq *CQuery)String() string { return (*ds.Query)(cq).String() }
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
func (cq *CQuery)Distinct() *CQuery {
	return (*CQuery)((*ds.Query)(cq).Distinct())
}
func (cq *CQuery)KeysOnly() *CQuery {
	return (*CQuery)((*ds.Query)(cq).KeysOnly())
}


// Query builders for Complaint
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
func (cq *CQuery)ByFlight(flightnumber string) *CQuery {
	return cq.Filter("AircraftOverhead.FlightNumber = ", flightnumber)
}
func (cq *CQuery)ByIcaoId(icaoid string) *CQuery {
	return cq.Filter("AircraftOverhead.Id2 = ", icaoid)
}
func (cq *CQuery)OrderTimeAsc() *CQuery  { return cq.Order("Timestamp") }
func (cq *CQuery)OrderTimeDesc() *CQuery { return cq.Order("-Timestamp") }
//
// func (cq *CQuery)ByEmail(e string) *CQuery { return nil }
//   This isn't possible with just a CQuery; it needs access to the cdb
//   to generate the key needed for the ancestor query.

// Query builders for ComplainerProfile
//
func (cq *CQuery)ByButton(id string) *CQuery { return cq.Filter("ButtonId = ", id) }
func (cq *CQuery)ByCallerCode(cc string) *CQuery { return cq.Filter("CallerCode = ", cc) }


// Canned queries
//
func (cdb ComplaintDB)CQByEmail(email string) *CQuery {
	return NewComplaintQuery().
		Ancestor(cdb.emailToRootKeyer(email)).
		Order("Timestamp").
		Limit(-1)
}

func (cdb ComplaintDB)CPQByEmail(email string) *CQuery {
	return NewProfileQuery().
		Ancestor(cdb.emailToRootKeyer(email))
}
