package main

import(
	"golang.org/x/net/context"
	"flag"
	"fmt"
	"log"
	"os"
	"reflect"
	"regexp"
	"time"

	"github.com/skypies/util/date"
	"github.com/skypies/util/gcp/ds"
	"github.com/skypies/util/gcp/gcs"

	"github.com/skypies/complaints/pkg/complaintdb"
)

const(
	ArchiveGCSBucketName = "serfr0-complaints"
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
	fListUsers      bool
	fArchiveComplaints bool
	fArchiveFrom, fArchiveTo string
	fSearchArchive  bool
)

// {{{ init()

func init() {
	flag.IntVar(&fVerbosity, "v", 0, "verbosity level")
	flag.IntVar(&fLimit, "n", 40, "how many matches to retrieve")
	flag.StringVar(&fUser, "user", "", "email address of user")
	flag.BoolVar(&fDesc, "desc", false, "descending order of timestamp")
	flag.BoolVar(&fSummary, "summary", false, "generate a summary report over the time period")
	flag.BoolVar(&fListUsers, "users", false, "report users (not complaints)")
	flag.BoolVar(&fArchiveComplaints, "archive", false, "archive complaints in timewindow to GCS freezefiles")
	flag.StringVar(&fArchiveFrom, "archivefrom", "", "2015.01.01")
	flag.StringVar(&fArchiveTo, "archiveto", "", "2015.01.02")
	//flag.BoolVar(&fPurgeFlights, "purge", false, "remove flightnumber from random() complaints")
	flag.BoolVar(&fSearchArchive, "archivesearch", false, "run queries against archive VERY SLOWLY")
	
	var s, e timeType
	flag.Var(&s, "s", "start time in PT (2006-01-02T15:04:05)")
	flag.Var(&e, "e", "end time in PT   (2006-01-02T15:04:05)")	
	flag.Parse()

	for _, e := range []string{"GOOGLE_APPLICATION_CREDENTIALS"} {
		if os.Getenv(e) == "" {
			log.Fatal("You're gonna need $"+e)
		}
	}
	
	fTStart = time.Time(s)
	fTEnd = time.Time(e)

	cdb = complaintdb.NewDB(ctx)
	cdb.Logger = log.New(os.Stderr,"", log.Ldate|log.Ltime)//|log.Lshortfile)	
}

// }}}

// {{{ type timeType

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

// }}}
// {{{ queryFromArgs

func queryFromArgs() *complaintdb.CQuery {
	cq := cdb.NewComplaintQuery()

	if fUser != "" { cq = cdb.CQByEmail(fUser) }

	if ! fTStart.IsZero() { cq = cq.Filter("Timestamp >= ", fTStart) }
	if ! fTEnd.IsZero() { cq = cq.Filter("Timestamp < ", fTEnd) }

	if fLimit > 0 {
		cq.Limit(fLimit)
	}

	if fDesc {
		cq.Order("-Timestamp")
	} else {
		cq.Order("Timestamp")
	}

	return cq
}

// }}}

// {{{ runQuery

func runQuery(cq *complaintdb.CQuery) {
	fmt.Printf("Running query %s\n", cq)
	
	iter := cdb.NewComplaintIterator(cq)
	iter.PageSize = 100
	fmt.Printf("%d complaints to work through\n", iter.Remaining())

	n := 0
	toWrite := []complaintdb.Complaint{}
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
/*
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
*/

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

// }}}
// {{{ runSummaryReport

func runSummaryReport() {
	s,e := time.Time(fTStart), time.Time(fTEnd)
	if s.IsZero() || e.IsZero() {
		s,e = date.WindowForYesterday()
	}

	fmt.Printf("(running summary report, from %s to %s)\n", s,e)
	tStart := time.Now()
	if str,err := cdb.SummaryReport(s,e,false,map[string]int{}); err != nil {
		log.Fatal(err)
	} else {
		fmt.Printf("\n%s\n", str)
		fmt.Printf("(report took %s to run)\n", time.Since(tStart))
	}
}

// }}}
// {{{ runUserReport

func runUserReport() {
	profiles, err := cdb.LookupAllProfiles(cdb.NewProfileQuery())
	if err != nil {
		log.Fatal(err)
	}
	for _,p := range profiles {
		fmt.Printf("%s\n", p.EmailAddress)
	}
	fmt.Printf("(%d profiles found)\n", len(profiles))
}

// }}}
// {{{ runArchiveQuery

// -archivesearch                                                 : all archives (v slow !!)
// -archivesearch -archivefrom=2015.08.10 -archiveto=2015.08.11   : days 2 & 3

func runArchiveQuery() {
	if fUser == "" {
		fmt.Printf("Need a user to run archive query, aborting\n")
		return
	}

	
	fmt.Printf("Running archive query - SLOW\n")
	fmt.Printf("-- user = %s\n", fUser)

	s := date.Datestring2MidnightPdt("2015.08.09")  // Day of the first complaints
	e := date.TruncateToLocalDay(date.NowInPdt())   // Today
	if fArchiveFrom != "" {
		s = date.Datestring2MidnightPdt(fArchiveFrom)
	}
	if fArchiveTo != "" {
		e = date.Datestring2MidnightPdt(fArchiveTo)
	}
	fmt.Printf("-- tStart = %s\n-- tEnd   = %s\n", s, e)

	csvFilename := s.Format("archive-20060102") + e.Format("-20060102.csv")

	writer, err := os.Create(csvFilename)
	defer writer.Close()
	if err != nil {
		fmt.Printf("open+w '%s': %v\n", csvFilename, err)
		return
	}
	fmt.Printf("-- output file: %s\n\n", csvFilename)
	cdb.AddHeadersToCSV(writer)
	
	// Nudge a second either way, else IntermediateMidnights will skip them
	s = s.Add(-1 * time.Second)
	e = e.Add(1 * time.Second)
	midnights := date.IntermediateMidnights(s, e)
	for _, m := range midnights {
		gcsFilename := m.Format("2006-01-02-archived-complaints")
		gcsFileExists,_ := gcs.Exists(ctx, ArchiveGCSBucketName, gcsFilename)
		if !gcsFileExists {
			fmt.Printf("- %s: not found, skipping\n", gcsFilename)
			continue
		}

		complaints, error := searchArchiveComplaints(ArchiveGCSBucketName, gcsFilename, fUser)
		if error != nil {
			fmt.Printf("Loading %s: err %v\n", gcsFilename, error)
			continue
		}
		if len(complaints) == 0 {
			fmt.Printf("- %s: none found\n", gcsFilename)
			continue
		}

		if err := cdb.AddComplaintSliceToCSV(complaints, writer); err != nil {
			fmt.Printf("error writing %d complaints to %s: %v\n", len(complaints), csvFilename, err)
			continue
		}

		fmt.Printf("- %s: wrote %d\n", gcsFilename, len(complaints))
	}

}

// }}}

// {{{ archiveComplaints

// -archivefrom=2015.01.01                        :  does just that day (first of Jan)
// -archivefrom=2015.01.01 -archiveto=2015.01.01  :  also does just that day (first of Jan)
// -archivefrom=2015.01.01 -archiveto=2015.01.04  :  does the first four days of Jan

// go run cdb.go -archive -archivefrom=2015.08.09 : the first day, with 31 complaints

// have archived up to 2022.06.30

func archiveComplaints() {
	gcsOverwrite := false
	deleteFromDatabase := true

	s := date.Datestring2MidnightPdt(fArchiveFrom)
	e := date.Datestring2MidnightPdt(fArchiveTo)

	if s.IsZero() {
		log.Fatal("need archivefrom")
	} else if e.IsZero() {
		log.Printf("(assuming single day of archiving)")
		e = s
	}
	// Nudge a second either way, else IntermediateMidnights will skip them
	s = s.Add(-1 * time.Second)
	e = e.Add(1 * time.Second)

	log.Printf("(archiving from %s - %s)\n", s, e)

	midnights := date.IntermediateMidnights(s, e)
	for _, m := range midnights {
		gcsFilename := m.Format("2006-01-02-archived-complaints")
		gcsFileExists,_ := gcs.Exists(ctx, ArchiveGCSBucketName, gcsFilename)

		winS, winE := date.WindowForTime(m) // start/end timestamps for the 23-25h day that follows the midnight

		// A fresh iterating query for each day, to go into its own GCS file.
		cq := cdb.NewComplaintQuery().ByTimespan(winS, winE)
		complaints,err := cdb.LookupAll(cq)
		if err != nil {
			log.Fatal(err)
		}

		if len(complaints) == 0 {
			if gcsFileExists {
				log.Printf(" -- [%s], no complaints found in DB - already archived. skipping\n", m)
				continue 
			} else {
				log.Fatal(" -- [%s], no complaints found in DB, no archive found, bad date ?!\n", m)
			}
		}
		
		if gcsFileExists && !gcsOverwrite {
			log.Printf(" --[%s], file %s/%s already exists, will verify it\n", m, ArchiveGCSBucketName, gcsFilename)

		} else {

			filehandle,err := gcs.OpenRW(ctx, ArchiveGCSBucketName, gcsFilename, "application/octet-stream")
			if err != nil {
				log.Fatal(err)
			}
			if err := cdb.MarshalComplaintSlice(complaints, filehandle.IOWriter()); err != nil {
				log.Fatal(err)
			}
			if err := filehandle.Close(); err != nil {
				log.Fatal(err)
			}
			log.Printf(" --[%s], %d complaints written to %s/%s\n", m, len(complaints),
				ArchiveGCSBucketName, gcsFilename)
		}

		// Reads 'em all back, compare to what we stated with
		if err := verifyArchiveComplaints(ArchiveGCSBucketName, gcsFilename, complaints); err != nil {
			log.Fatal(err)
		}

		// Archiving looks good - delete them from datastore !
		if deleteFromDatabase {
			keyers := make([]ds.Keyer, len(complaints))
			for i:=0; i<len(complaints); i++ {
				keyers[i] = cdb.GetKeyerOrNil(complaints[i])
			}

			log.Printf("deleting %d archived complaints from DB ...\n", len(keyers))

			// May need to make multiple calls
			maxKeyersToDeleteInOneCall := 500
			for len(keyers) > 0 {
				keyersToDelete := []ds.Keyer{}
				if len(keyers) <= maxKeyersToDeleteInOneCall {
					keyersToDelete, keyers = keyers, keyersToDelete
				} else {
					keyersToDelete, keyers = keyers[0:maxKeyersToDeleteInOneCall], keyers[maxKeyersToDeleteInOneCall:]
				}

				if err := cdb.DeleteAllKeys(keyersToDelete); err != nil {
					log.Fatal(err)
				}
			}
			log.Printf("... done\n")
		}
	}
}

// }}}
// {{{ verifyArchiveComplaints

func verifyArchiveComplaints(bucketname, filename string, origComplaints []complaintdb.Complaint) error {
	
	if exists,_ := gcs.Exists(ctx, bucketname, filename); !exists {
		return fmt.Errorf("can not find existing file %s/%s", bucketname, filename)
	}

	filehandle, err := gcs.OpenR(ctx, bucketname, filename)
	if err != nil {
		return err
	}
	defer filehandle.Close()

	rdr, err := filehandle.ToReader(ctx, bucketname, filename)
	if err != nil {
		return err
	}

	archivedComplaints, err := cdb.UnmarshalComplaintSlice(rdr)
	if err != nil {
		return err
	}


	if len(archivedComplaints) != len(origComplaints) {
		return fmt.Errorf("%s/%s: count mismatch - orig=%d, archive=%d\n", bucketname, filename,
			len(origComplaints), len(archivedComplaints))
	}

	for i:=0; i<len(origComplaints); i++ {
		cSanitizedOrig := origComplaints[i].ToCopyWithStoredDataOnly()
		if ! reflect.DeepEqual(cSanitizedOrig, archivedComplaints[i]) {
			return fmt.Errorf("%s/%s: complaint %d did not match:-\n* orig: %#v\n* arcv: %#v\n\n",
				bucketname, filename, i, cSanitizedOrig, archivedComplaints[i])
		}
	}

	log.Printf("  - %d complaints read and verified from %s/%s\n", len(archivedComplaints), bucketname, filename)

	return nil
}

// }}}
// {{{ searchArchiveComplaints

func searchArchiveComplaints(bucketname, filename string, username string) ([]complaintdb.Complaint, error) {
	ret := []complaintdb.Complaint{}

	filehandle, err := gcs.OpenR(ctx, bucketname, filename)
	if err != nil {
		return ret, err
	}
	defer filehandle.Close()

	rdr, err := filehandle.ToReader(ctx, bucketname, filename)
	if err != nil {
		return ret, err
	}

	archivedComplaints, err := cdb.UnmarshalComplaintSlice(rdr)
	if err != nil {
		return ret, err
	}

	for _, c := range archivedComplaints {
		if c.Profile.EmailAddress != username {
			continue
		}

		ret = append(ret, c)
	}
	
	return ret, nil
}

// }}}

// {{{ main()

func main() {
	if fSummary {
		runSummaryReport()
		return

	} else if fListUsers {
		runUserReport()
		return

	} else if fArchiveComplaints {
		archiveComplaints()
		return

	} else if fSearchArchive {
		runArchiveQuery()
		return
	}

	if len(flag.Args()) == 0 {
		runQuery(queryFromArgs())
	}

	// Bare args are individual complaint keys
	for _,k := range flag.Args() {
		c, err := cdb.LookupKey(k,"")
		if err != nil { log.Fatal(err) }
		fmt.Printf(" * [exp] %s\n", c)

		// TODO: add args to handle deletion
		//keyer,_ := cdb.Provider.DecodeKey(c.DatastoreKey)
		//if err := cdb.DeleteByKey(keyer); err != nil {
		//	log.Fatal(err)
		//}
	}
}

// }}}


// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
