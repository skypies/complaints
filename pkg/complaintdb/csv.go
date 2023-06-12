package complaintdb

import (
	"encoding/csv"
	"fmt"
	"io"
	"time"
)

// {{{ CSVHeaders

func (cdb ComplaintDB)CSVHeaders() []string {
	return []string{
		"CallerCode", "Name", "Address", "Zip", "Email",
		"HomeLat", "HomeLong", "UnixEpoch", "Date", "Time(PDT)",
		"Notes", "Flightnumber", "ActivityDisturbed", "Loudness", "HeardSpeedbrakes",
	}
}

func (cdb ComplaintDB)ComplaintToCSVFunc() func(c *Complaint) []string {
	return func(c *Complaint) []string {
		r := []string{
			c.Profile.CallerCode,
			c.Profile.FullName,
			c.Profile.Address,
			c.Profile.StructuredAddress.Zip,
			c.Profile.EmailAddress,

			fmt.Sprintf("%.4f",c.Profile.Lat),
			fmt.Sprintf("%.4f",c.Profile.Long),
			fmt.Sprintf("%d", c.Timestamp.UTC().Unix()),
			c.Timestamp.Format("2006/01/02"),
			c.Timestamp.Format("15:04:05"),

			c.Description,
			c.AircraftOverhead.FlightNumber,
			c.Activity,
			fmt.Sprintf("%d", c.Loudness),
			fmt.Sprintf("%v", c.HeardSpeedbreaks),
		}
		return r
	}
}

// }}}

// {{{ WriteCQueryToCSV

func (cdb ComplaintDB)WriteCQueryToCSV(cq *CQuery, w io.Writer, headers bool) (int, error) {
	cols := []string{}

	if headers {
		cols = cdb.CSVHeaders()
	}

	f := cdb.ComplaintToCSVFunc()

	return cdb.FormattedWriteCQueryToCSV(cq, w, cols, f)
}

// }}}
// {{{ FormattedWriteCQueryToCSV

func (cdb ComplaintDB)FormattedWriteCQueryToCSV(cq *CQuery, w io.Writer, headers []string, f func(*Complaint) []string) (int, error) {
	csvWriter := csv.NewWriter(w)

	if len(headers) > 0 {
		csvWriter.Write(headers)
	}

	tIter := time.Now()
	iter := cdb.NewComplaintIterator(cq)
	iter.PageSize = 200

	n := 0
	for iter.Iterate(cdb.Ctx()) {
		c := iter.Complaint()
		r := f(c)

		if err := csvWriter.Write(r); err != nil {
			return 0,err
		}

		n++
	}
	if iter.Err() != nil {
		return n,fmt.Errorf("iterator failed after %s: %v", iter.Err(), time.Since(tIter))
	}

	csvWriter.Flush()

	return n,nil
}

// }}}

// {{{ AddHeadersToCSV

func (cdb ComplaintDB)AddHeadersToCSV(w io.Writer) {
	csvWriter := csv.NewWriter(w)
	csvWriter.Write(cdb.CSVHeaders())
	csvWriter.Flush()
}

// }}}
// {{{ AddComplaintSliceToCSV

func (cdb ComplaintDB)AddComplaintSliceToCSV(complaints []Complaint, w io.Writer) error {
	csvWriter := csv.NewWriter(w)

	for _, c := range complaints {
		f := cdb.ComplaintToCSVFunc()
		r := f(&c)

		if err := csvWriter.Write(r); err != nil {
			return err
		}
	}

	csvWriter.Flush()

	return nil
}

// }}}


// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
