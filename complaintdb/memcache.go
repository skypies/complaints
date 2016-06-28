package complaintdb

import (
	"bytes"
	"encoding/gob"
	"fmt"

	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/memcache"
	"golang.org/x/net/context"

	"github.com/skypies/complaints/complaintdb/types"
)

const chunksize = 950000

// Memcache has a 1MB per-object limit. This file contains a few routines for transparent
// sharding across 32 memcache objects, to allow for objects up to 32MB.

// {{{ BytesFromShardedMemcache

// bool means 'found'
func BytesFromShardedMemcache(c context.Context, key string) ([]byte, bool) {
	keys := []string{}
	for i:=0; i<32; i++ { keys = append(keys, fmt.Sprintf("=%d=%s",i*chunksize,key)) }

	if items,err := memcache.GetMulti(c, keys); err != nil {
		log.Errorf(c, "fdb memcache multiget: %v", err)
		return nil,false

	} else {
		b := []byte{}
		for i:=0; i<32; i++ {
			if item,exists := items[keys[i]]; exists==false {
				break
			} else {
				log.Infof(c, " #=== Found '%s' !", item.Key)
				b = append(b, item.Value...)
			}
		}

		log.Infof(c," #=== Final read len: %d", len(b))

		if len(b) > 0 {
			return b, true
		} else {
			return nil, false
		}
	}
}

// }}}
// {{{ BytesToShardedMemcache

// Object usually too big (1MB limit), so shard.
// http://stackoverflow.com/questions/9127982/
func BytesToShardedMemcache(c context.Context, key string, b []byte) {
	items := []*memcache.Item{}
	for i:=0; i<len(b); i+=chunksize {
		k := fmt.Sprintf("=%d=%s",i,key)
		s,e := i, i+chunksize-1
		if e>=len(b) { e = len(b)-1 }
		log.Infof(c," #=== [%7d, %7d] (%d) %s", s, e, len(b), k)
		items = append(items, &memcache.Item{ Key:k , Value:b[s:e+1] }) // slice sytax is [s,e)
	}

	if err := memcache.SetMulti(c, items); err != nil {
		log.Errorf(c," #=== cdb sharded store fail: %v", err)
	}

	log.Infof(c," #=== Stored '%s' (len=%d)!", key, len(b))
}

// }}}

// {{{ cdb.getMaybeCachedComplaintsByQuery

type memResults struct {
	Keys []*datastore.Key
	Vals []types.Complaint
}

// This whole thing is no longer invoked; memcache gets evicted every hour or so, so data
// the cache hit rate is too low to be worth it.
func (cdb ComplaintDB)getMaybeCachedComplaintsByQuery(q *datastore.Query, memKey string) ([]*datastore.Key, []types.Complaint, error) {

	useMemcache := false
	
	cdb.Debugf("gMCCBQ_100", "getMaybeCachedComplaintsByQuery (memcache=%v)", useMemcache)

	if useMemcache && memKey != "" {
		cdb.Debugf("gMCCBQ_102", "checking memcache '%s'", memKey)
		if b,found := BytesFromShardedMemcache(cdb.Ctx(), memKey); found == true {
			buf := bytes.NewBuffer(b)
			cdb.Debugf("gMCCBQ_103", "memcache hit (%d bytes)", buf.Len())
			results := memResults{}
			if err := gob.NewDecoder(buf).Decode(&results); err != nil {
				cdb.Errorf("cdb memcache multiget decode: %v", err)
			} else {
				cdb.Infof(" #=== Found all items ? Considered cache hit !")
				return results.Keys, results.Vals, nil
			}
		} else {
			cdb.Debugf("gMCCBQ_103", "memcache miss")
		}
	}

	keys, data, err := cdb.getComplaintsByQueryFromDatastore(q)
	if err != nil { return nil, nil, err }

	if useMemcache && memKey != "" {
		var buf bytes.Buffer
		dataToCache := memResults{Keys:keys, Vals:data}
		if err := gob.NewEncoder(&buf).Encode(dataToCache); err != nil {
			cdb.Errorf(" #=== cdb error encoding item: %v", err)
		} else {
			b := buf.Bytes()
			cdb.Debugf("gMCCBQ_106", "storing to memcache ...")
			BytesToShardedMemcache(cdb.Ctx(), memKey, b)
			cdb.Debugf("gMCCBQ_106", "... stored")
		}
	}

	cdb.Debugf("gMCCBQ_106", "all done with getMaybeCachedComplaintsByQuery")
	return keys, data, nil
}

// }}}


// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
