package complaintdb

import(
	"golang.org/x/net/context"
	"github.com/skypies/util/gcp/ds"
)

// A shim on the datastoreprovider iterator that can talk flights
type ComplaintIterator ds.Iterator

func (cdb *ComplaintDB)NewComplaintIterator(cq *CQuery) *ComplaintIterator {
	it := ds.NewIterator(cdb.Ctx(), cdb.Provider, (*ds.Query)(cq), Complaint{})
	return (*ComplaintIterator)(it)
}

func (ci *ComplaintIterator)Iterate(ctx context.Context) bool {
	it := (*ds.Iterator)(ci)
	return it.Iterate(ctx)
}

func (ci *ComplaintIterator)Remaining() int {
	it := (*ds.Iterator)(ci)
	return it.Remaining()
}

func (ci *ComplaintIterator)Err() error {
	it := (*ds.Iterator)(ci)
	return it.Err()
}

func (ci *ComplaintIterator)Complaint() *Complaint {
	c := Complaint{}

	it := (*ds.Iterator)(ci)
	keyer := it.Val(&c)

	FixupComplaint(&c, keyer.Encode())

	return &c
}
