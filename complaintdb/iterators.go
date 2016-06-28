package complaintdb

import (
	"google.golang.org/appengine/datastore"

	"github.com/skypies/complaints/complaintdb/types"
)

// TODO: Iter.EOF, for better for loops

type ComplaintIterator struct {
	//ComplaintDB CDB
	Query *datastore.Query
	Iter  *datastore.Iterator
}

// Runs at ~1000/sec; watch for appengine timeouts
func (ci *ComplaintIterator)NextWithErr() (*types.Complaint, error) {
	var complaint types.Complaint
	k, err := ci.Iter.Next(&complaint)
	
	if err == datastore.Done {
		return nil,nil // We're all done
	}
	if err != nil {
		return nil,err
	}

	FixupComplaint(&complaint, k)
	
	return &complaint, nil
}

func (ci ComplaintIterator)Next() *types.Complaint {
	c,_ := ci.NextWithErr()
	return c
}

func (cdb ComplaintDB)NewIter(q *datastore.Query) *ComplaintIterator {
	ci := ComplaintIterator{
		//CDB:   cdb,
		Query: q,
		Iter:  q.Run(cdb.Ctx()),
	}
	return &ci
}
