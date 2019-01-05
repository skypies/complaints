package main

import(
	"golang.org/x/net/context"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"time"

	"github.com/skypies/util/date"
	"github.com/skypies/util/gcp/ds"

	"github.com/skypies/complaints/complaintdb"
	"github.com/skypies/complaints/complaintdb/types"
)

var(
	ctx = context.Background()
	cdb             complaintdb.ComplaintDB
	fVerbosity      int
	fLimit          int
	fTStart, fTEnd  time.Time
	fUser           string
	fDesc           bool
	fPurgeFlights   bool
	fSummary        bool
)
	
func init() {
	flag.IntVar(&fVerbosity, "v", 0, "verbosity level")
	flag.IntVar(&fLimit, "n", 40, "how many matches to retrieve")
	flag.StringVar(&fUser, "user", "", "email address of user")
	flag.BoolVar(&fDesc, "desc", false, "descending order of timestamp")
	flag.BoolVar(&fSummary, "summary", false, "generate a summary report over the time period")
	//flag.BoolVar(&fPurgeFlights, "purge", false, "remove flightnumber from random() complaints")

	var s,e timeType
	flag.Var(&s, "s", "start time (2006-01-02T15:04:05)")
	flag.Var(&e, "e", "end time (2006-01-02T15:04:05)")	
	flag.Parse()

	fTStart = time.Time(s)
	fTEnd = time.Time(e)

	cdb = complaintdb.NewDB(ctx)
	cdb.Logger = log.New(os.Stderr,"", log.Ldate|log.Ltime)//|log.Lshortfile)	
	if p,err := ds.NewCloudDSProvider(ctx,"serfr0-1000"); err != nil {
		log.Fatalf("coud not get a clouddsprovider: %v\n", err)
	} else {
		cdb.Provider = p
	}
}

// timeType is a time that implements flag.Value
type timeType time.Time
func (t *timeType) String() string { return date.InPdt(time.Time(*t)).Format(time.RFC3339) }
func (t *timeType) Set(value string) error {
	format := "2006-01-02T15:04:05"  // No zoned time.RFC3339, because ParseInPdt adds one 
	if tm,err := date.ParseInPdt(format, value); err != nil {
		return err
	} else {
		*t = timeType(tm)
	}
	return nil
}

func queryFromArgs() *complaintdb.CQuery {
	cq := cdb.NewComplaintQuery()

	if fUser != "" { cq = cdb.CQByEmail(fUser) }

	if ! fTStart.IsZero() { cq = cq.Filter("Timestamp >= ", fTStart) }
	if ! fTEnd.IsZero() { cq = cq.Filter("Timestamp < ", fTEnd) }

	cq.Limit(fLimit)

	if fDesc {
		cq.Order("-Timestamp")
	} else {
		cq.Order("Timestamp")
	}

	return cq
}

func runQuery(cq *complaintdb.CQuery) {
	fmt.Printf("Running query %s\n", cq)
	
	iter := cdb.NewComplaintIterator(cq)
	iter.PageSize = 100
	fmt.Printf("%d complaints to work through\n", iter.Remaining())

	n := 0
	toWrite := []types.Complaint{}
	for iter.Iterate(ctx) {
		n++
		c := iter.Complaint()

		if fVerbosity>1 {
			fmt.Printf("%s\n", c)
		}

		if ! regexp.MustCompile("(outcome: random)").MatchString(c.Debug) {
			continue
		}
		
		if c.AircraftOverhead.FlightNumber == "" {
			continue // No work to do
		}

		// Left here as an example of batch complaint mutation
		if false && fPurgeFlights {
			c.AircraftOverhead.FlightNumber = ""
			toWrite = append(toWrite, *c)
			if len(toWrite) >= 50 {
				if err := cdb.PersistComplaints(toWrite); err != nil {
					log.Fatal(err)
				}
				toWrite = nil
			}
		}

		fmt.Printf("[%2d] %s\n", n, c)

		if fVerbosity>0 {
			fmt.Printf("%s\n", c.Debug)
		}
	}
	if iter.Err() != nil {
		log.Fatal(iter.Err())
	}

	// Stragglers
	if len(toWrite) > 0 {
		if err := cdb.PersistComplaints(toWrite); err != nil {
			log.Fatal(err)
		}
	}

	fmt.Printf("\n")
}

func runSummaryReport() {
	s,e := time.Time(fTStart), time.Time(fTEnd)
	if s.IsZero() || e.IsZero() {
		s,e = date.WindowForYesterday()
	}

	fmt.Printf("(running summary report, from %s to %s)\n", s,e)
	tStart := time.Now()
	if str,err := cdb.SummaryReport(s,e,false,map[string]int{}); err != nil {
		Log.Fatal(err)
	} else {
		fmt.Printf("\n%s\n", str)
		fmt.Printf("(report took %s to run)\n", time.Since(tStart))
	}
}

func main() {
	if fSummary == true {
		runSummaryReport()
		return
	}
	
	if len(flag.Args()) == 0 {
		runQuery(queryFromArgs())
	}

	for _,k := range flag.Args() {
		c,err := cdb.LookupKey(k,"")
		if err != nil { log.Fatal(err) }
		fmt.Printf(" * [exp] %s\n", c)
		//keyer,_ := cdb.Provider.DecodeKey(c.DatastoreKey)
		//if err := cdb.DeleteByKey(keyer); err != nil {
		//	log.Fatal(err)
		//}
	}
}


// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
