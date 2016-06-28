// This file is a central place for routines that generate various datastore.Queries
package complaintdb

import (
	"time"

	"google.golang.org/appengine/datastore"
)

func (cdb ComplaintDB) QueryInSpan(start, end time.Time) *datastore.Query {
	return datastore.
		NewQuery(kComplaintKind).
		Filter("Timestamp >= ", start).
		Filter("Timestamp < ", end).
		Order("Timestamp").
		Limit(-1)
}

func (cdb ComplaintDB) QueryInSpanInZip(start, end time.Time, zip string) *datastore.Query {
	return datastore.
		NewQuery(kComplaintKind).
		Filter("Profile.StructuredAddress.Zip = ", zip).
		Filter("Timestamp >= ", start).
		Filter("Timestamp < ", end).
		Order("Timestamp").
		Limit(-1)
}

func (cdb ComplaintDB) QueryInSpanByEmailAddress(start,end time.Time, email string) *datastore.Query {
	return datastore.
		NewQuery(kComplaintKind).
		Ancestor(cdb.emailToRootKey(email)).
		Filter("Timestamp >= ", start).
		Filter("Timestamp < ", end).
		Order("Timestamp").
		Limit(-1)
}

func (cdb ComplaintDB) QueryAllByEmailAddress(email string) *datastore.Query {
	return datastore.
		NewQuery(kComplaintKind).
		Ancestor(cdb.emailToRootKey(email)).
		Order("Timestamp").
		Limit(-1)
}
