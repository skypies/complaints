package complaintdb

import (
	"google.golang.org/appengine/datastore"
	"golang.org/x/net/context"

	"github.com/skypies/complaints/complaintdb/types"
)

// LongBatchingIterator is a drop-in replacement, to allow the consumer to spend more than 60s
// iterating over the result set; it fetches results in batches.
type LongBatchingIterator struct {
	Ctx         context.Context

	BatchSize   int          // How many items to pull down per datastore.GetAll

	keys []*datastore.Key   // Full set of keys, yet to be fetched (mutated each page fetch)
	vals []*types.Complaint // A cache of unreturned results (mutated each iteration)

	val        *types.Complaint  // The currently returned result (== Keys[iResults])
	err         error
}

func (iter *LongBatchingIterator)NextWithErr() (*types.Complaint, error) {
	if iter.err != nil { return nil, iter.err }

	if len(iter.vals) == 0 && len(iter.keys) == 0 {
		return nil,nil // We're all done !
	}

	// No new vals left in the cache; fetch some
	if len(iter.vals) == 0 {
		var keysForThisBatch []*datastore.Key

		if len(iter.keys) < iter.BatchSize {
			// Remaining keys not enough for a full page; grab all of 'em
			keysForThisBatch = iter.keys
			iter.keys = []*datastore.Key{}
		} else {
			keysForThisBatch  = iter.keys[0:iter.BatchSize]
			iter.keys         = iter.keys[iter.BatchSize:]
		}

		// Fetch the complaints for the keys in this batch
		complaints := make ([]types.Complaint, len(keysForThisBatch))
		if err := datastore.GetMulti(iter.Ctx, keysForThisBatch, complaints); err != nil {
			iter.err = err
			return nil, err
		}

		iter.vals = make([]*types.Complaint, len(keysForThisBatch))
		for i,_ := range complaints {
			FixupComplaint(&complaints[i], keysForThisBatch[i])
			iter.vals[i] = &complaints[i]
		}
	}

	// We have unreturned results in the cache; shift & return it
	complaint := iter.vals[0]
	iter.vals = iter.vals[1:]

	return complaint, nil
}


// Snarf down all the keys from the get go.
func (cdb *ComplaintDB)NewLongBatchingIter(q *datastore.Query) *LongBatchingIterator {
	ctx := cdb.Ctx()
	keys,err := q.KeysOnly().GetAll(ctx, nil)
	i := LongBatchingIterator{
		Ctx:       ctx,
		BatchSize: 100,
		keys:      keys,
		vals:   []*types.Complaint{},
		err:       err,
	}

	return &i
}


func (iter *LongBatchingIterator)Next() *types.Complaint {
	c,_ := iter.NextWithErr()
	return c
}


func (iter *LongBatchingIterator)Iterate() bool {
	iter.val,iter.err = iter.NextWithErr()
	return iter.val != nil
}
func (iter *LongBatchingIterator)Complaint() *types.Complaint { return iter.val }
func (iter *LongBatchingIterator)Err() error { return iter.err }
