package complaintdb

import (
	"appengine"
	"appengine/datastore"
	"github.com/skypies/complaints/complaintdb/types"
)

// TODO: Iter.EOF, for better for loops

type ComplaintIterator struct {
	C      appengine.Context
	Query *datastore.Query
	Iter  *datastore.Iterator

	EOF    bool
}

// Runs at ~1000/sec; watch for appengine timeouts
func (ci *ComplaintIterator)NextWithErr() (*types.Complaint, error) {
	var complaint types.Complaint
	k, err := ci.Iter.Next(&complaint)
	
	if err == datastore.Done {
		ci.EOF = true
		return nil,nil // We're all done
	}
	if err != nil {
		ci.EOF = true
		ci.C.Errorf("iter.Next: %v", err)
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
		C:     cdb.C,
		Query: q,
		Iter:  q.Run(cdb.C),
		EOF:   false,
	}
	return &ci
}
