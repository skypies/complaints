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

// {{{ testEverything

func testEverything(t *testing.T, p dsprovider.DatastoreProvider) {
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

	tm := time.Now()
	run(2,               cdb.NewComplaintQuery().ByTimespan(tm.Add(-90*time.Second), tm))
	run(0,               cdb.NewComplaintQuery().ByTimespan(tm, tm.Add(1*time.Hour)))

	run(len(complaints), cdb.CQByEmail(complaints[0].Profile.EmailAddress))
	run(0,               cdb.CQByEmail("no@such.address.com"))

	// Now delete something
	results,err := cdb.LookupAll(cdb.NewComplaintQuery().Limit(1).Order("Timestamp"))
	if err != nil || len(results) != 1 {
		t.Errorf("db.GetFirstByQuery: %v / %v\n", err, results)
	} else if keyer,err := cdb.Provider.DecodeKey(results[0].DatastoreKey); err != nil {
		t.Errorf("p.DecodeKey: %v\n", err)
	} else if err := cdb.DeleteByKey(keyer); err != nil {
		t.Errorf("p.Delete: %v\n", err)
	}

	nExpected := len(complaints)-1
	run(nExpected, cdb.NewComplaintQuery())

	// Now test the iterator
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

func TestEverything(t *testing.T) {
	testEverything(t, dsprovider.AppengineDSProvider{})
	// Sadly, the aetest framework hangs on the first Put from the cloud client
	// testEverything(t, dsprovider.CloudDSProvider{appid})
}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
