package complaintdb

import(
	"golang.org/x/net/context"
	"github.com/skypies/util/dsprovider"
	"github.com/skypies/complaints/complaintdb/types"
)

// A shim on the dsprovider iterator that can talk flights
type ComplaintIterator dsprovider.Iterator

func (cdb *ComplaintDB)NewComplaintIterator(cq *CQuery) *ComplaintIterator {
	it := dsprovider.NewIterator(cdb.Ctx(), cdb.Provider, (*dsprovider.Query)(cq), types.Complaint{})
	return (*ComplaintIterator)(it)
}

func (ci *ComplaintIterator)Iterate(ctx context.Context) bool {
	it := (*dsprovider.Iterator)(ci)
	return it.Iterate(ctx)
}

func (ci *ComplaintIterator)Remaining() int {
	it := (*dsprovider.Iterator)(ci)
	return it.Remaining()
}

func (ci *ComplaintIterator)Err() error {
	it := (*dsprovider.Iterator)(ci)
	return it.Err()
}

func (ci *ComplaintIterator)Complaint() *types.Complaint {
	c := types.Complaint{}

	it := (*dsprovider.Iterator)(ci)
	keyer := it.Val(&c)

	FixupComplaint(&c, keyer.Encode())

	return &c
}
