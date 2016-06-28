package complaintdb

import (
	"google.golang.org/appengine/datastore"
	"golang.org/x/net/context"

	"github.com/skypies/complaints/complaintdb/types"
)

var KResultsPageSize = 1

// LongIterator is a drop-in replacement, to allow the consumer to spend more than 60s
// iterating over the result set.
type LongIterator struct {
	Ctx     context.Context

	Keys []*datastore.Key // The full result set

	i           int  // Index into Keys
	val        *types.Complaint
	err         error
}

// Snarf down all the keys from the get go.
func (cdb *ComplaintDB)NewLongIter(q *datastore.Query) *LongIterator {
	ctx := cdb.Ctx()
	keys,err := q.KeysOnly().GetAll(ctx, nil)
	i := LongIterator{
		Ctx:  ctx,
		Keys: keys,
		err:  err,
	}

	return &i
}

func (iter *LongIterator)Iterate() bool {
	iter.val,iter.err = iter.NextWithErr()
	return iter.val != nil
}
func (iter *LongIterator)Complaint() *types.Complaint { return iter.val }
func (iter *LongIterator)Err() error { return iter.err }


func (iter *LongIterator)NextWithErr() (*types.Complaint, error) {
	if iter.err != nil { return nil, iter.err }

	if iter.i >= len(iter.Keys) {
		return nil,nil // We're all done !
	}
	
	key := iter.Keys[iter.i]
	iter.i++
	
	complaint := types.Complaint{}
	if err := datastore.Get(iter.Ctx, key, &complaint); err != nil {
		iter.err = err
		return nil, err
	}

	FixupComplaint(&complaint, key)
	
	return &complaint, nil
}

func (iter *LongIterator)Next() *types.Complaint {
	c,_ := iter.NextWithErr()
	return c
}
