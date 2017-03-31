package complaintdb

import (
	"fmt"
	"testing"
	"time"

	"golang.org/x/net/context"

	"google.golang.org/appengine"
	"google.golang.org/appengine/aetest" // Also used for testing Cloud API

	"github.com/skypies/util/dsprovider"
	"github.com/skypies/complaints/complaintdb/types"
)

const appid = "mytestapp"

// {{{ newConsistentContext

// A version of aetest.NewContext() that has a consistent datastore - so we can read our writes.
func newConsistentContext() (context.Context, func(), error) {
	inst, err := aetest.NewInstance(&aetest.Options{
		StronglyConsistentDatastore: true,
		AppID: appid,
	})
	if err != nil {
		return nil, nil, err
	}
	req, err := inst.NewRequest("GET", "/", nil)
	if err != nil {
		inst.Close()
		return nil, nil, err
	}
	ctx := appengine.NewContext(req)
	return ctx, func() {
		inst.Close()
	}, nil
}

// }}}
// {{{ getComplaints

func getComplaints(n int) []types.Complaint {
	ret := []types.Complaint{}
	for i:=0; i<n; i++ {
		ret = append(ret, types.Complaint{
			Timestamp: time.Now().Add(-1 * time.Minute * time.Duration(i)),
			Description: fmt.Sprintf("This is complaint %d of %d", i+1, n),
			Profile: types.ComplainerProfile{
				EmailAddress: "a@b.cc",
				FullName: "A Tester",
			},
		})
	}
	return ret
}

// }}}

// {{{ TestCoreAPI

func TestCoreAPI(t *testing.T) {
	p := dsprovider.AppengineDSProvider{} // can't make CloudDSProvider{} work with aetest
	ctx, done, err := newConsistentContext()
	if err != nil { t.Fatal(err) }
	defer done()

	cdb := NewDB(ctx)
	cdb.Provider = p
	
	complaints := getComplaints(5)
	for _,c := range complaints {
		if err := cdb.PersistComplaint(c); err != nil { t.Fatal(err) }
	}
	
	run := func(expected int, q *CQuery) []types.Complaint {
		if results,err := cdb.LookupAll(q); err != nil {
			t.Fatal(err)
		} else if len(results) != expected {
			t.Errorf("expected %d results, saw %d; query: %s", expected, len(results), q)
			for i,f := range results { fmt.Printf("result [%3d] %s\n", i, f) }
		} else {
			return results
		}
		return nil
	}
	run(len(complaints), cdb.NewComplaintQuery())
	run(3,               cdb.NewComplaintQuery().Limit(3))

	// Tests for the various semantic builders
	//
	tm := time.Now()
	run(2,               cdb.NewComplaintQuery().ByTimespan(tm.Add(-90*time.Second), tm))
	run(0,               cdb.NewComplaintQuery().ByTimespan(tm, tm.Add(1*time.Hour)))

	run(len(complaints), cdb.CQByEmail(complaints[0].Profile.EmailAddress))
	run(0,               cdb.CQByEmail("no@such.address.com"))

	c1 := run(1,         cdb.NewComplaintQuery().OrderTimeAsc().Limit(1))
	c2 := run(1,         cdb.NewComplaintQuery().OrderTimeDesc().Limit(1))
	if c1[0].Timestamp.After(c2[0].Timestamp) {
		t.Errorf("First from TimeAsc() was more recent than TimeDesc()")
	}

	// Check the ownership stuff
	if c,err := cdb.LookupKey(c1[0].DatastoreKey,""); c==nil || err != nil {
		t.Errorf("LookupKey with no owner; %v, %v", c, err)
	}
	if c,err := cdb.LookupKey(c1[0].DatastoreKey,c1[0].Profile.EmailAddress); c==nil || err != nil {
		t.Errorf("LookupKey with correct owner; %v, %v", c, err)
	}
	if c,err := cdb.LookupKey(c1[0].DatastoreKey,"no@such.owner.com"); c!=nil || err == nil {
		t.Errorf("LookupKey with incorrect owner; %v, %v", c, err)
	}
	
	// Now delete something
	first,err := cdb.LookupFirst(cdb.NewComplaintQuery().OrderTimeAsc())
	if err != nil || first == nil {
		t.Errorf("db.GetFirstByQuery: %v / %v\n", err, first)
	} else if keyer,err := cdb.Provider.DecodeKey(first.DatastoreKey); err != nil {
		t.Errorf("p.DecodeKey: %v\n", err)
	} else if err := cdb.DeleteByKey(keyer); err != nil {
		t.Errorf("p.Delete: %v\n", err)
	}

	nExpected := len(complaints)-1
	run(nExpected, cdb.NewComplaintQuery())

	// Test the iterator
	nSeen := 0
	it := cdb.NewComplaintIterator(cdb.NewComplaintQuery())
		for it.Iterate(ctx) {
		c := it.Complaint()
		fmt.Printf(" iterator result: %s\n", c)
		nSeen++
	}
	if it.Err() != nil {
		t.Errorf("test iterator err: %v\n", it.Err())
	}
	if nSeen != nExpected {
		t.Errorf("test expected to see %d, but saw %d\n", nExpected, nSeen)
	}
}

// }}}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
