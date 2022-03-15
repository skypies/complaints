package main

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	mailjet "github.com/mailjet/mailjet-apiv3-go"

	"github.com/skypies/util/date"
	"github.com/skypies/util/gcp/gcs"
	"github.com/skypies/util/widget"

	"github.com/skypies/complaints/complaintdb"
	"github.com/skypies/complaints/config"
)

// {{{ formValueMonthDefaultToPrev

// Gruesome. This pseudo-widget looks at 'year' and 'month', or defaults to the previous month.
// Everything is in Pacific Time.
func formValueMonthDefaultToPrev(r *http.Request) (month, year int, err error){
	// Default to the previous month
	oneMonthAgo := date.NowInPdt().AddDate(0,-1,0)
	month = int(oneMonthAgo.Month())
	year  = int(oneMonthAgo.Year())

	// Override with specific values, if present
	if r.FormValue("year") != "" {
		if y,err2 := strconv.ParseInt(r.FormValue("year"), 10, 64); err2 != nil {
			err = fmt.Errorf("need arg 'year' (2015)")
			return
		} else {
			year = int(y)
		}
		if m,err2 := strconv.ParseInt(r.FormValue("month"), 10, 64); err2 != nil {
			err = fmt.Errorf("need arg 'month' (1-12)")
			return
		} else {
			month = int(m)
		}
	}

	return
}

// }}}

// {{{ csvHandler

// Dumps the monthly CSV file into Google Cloud Storage, 
// Defaults to the previous month; else can specify an explicit year & month.
// If zip=no, just dumps the raw CSV into GCS. Otherwise, will Zip it, and
// also email it out to flysfo.

// https://overnight-dot-serfr0-1000.appspot.com/overnight/csv
//   ?year=2016&month=4
//   ?date=range&range_from=2006/01/01&range_to=2018/01/01
//  [?zip=no]


func csvHandler(w http.ResponseWriter, r *http.Request) {
	ctx := req2ctx(r)
	cdb := complaintdb.NewDB(ctx)
	tStart := time.Now()
	
	var s,e time.Time
	
	if r.FormValue("date") == "range" {
		s,e,_ = widget.FormValueDateRange(r)

	} else {
		month,year,err := formValueMonthDefaultToPrev(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	
		now := date.NowInPdt()
		s = time.Date(int(year), time.Month(month), 1, 0,0,0,0, now.Location())
		e = s.AddDate(0,1,0).Add(-1 * time.Second)
	}

	var filename string
	var n int
	var err error

	if r.FormValue("zip") == "no" {
		filename,n,err = generateComplaintsCSV(cdb, s, e)
	} else {
		filename,n,err = generateComplaintsCSVZip(cdb, s, e)
	}

	if err != nil {
		http.Error(w, fmt.Sprintf("monthly %s->%s: %v", s,e,err), http.StatusInternalServerError)
		return
	} else {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(fmt.Sprintf("OK!\nGCS file %s written, %d rows, took %s", filename, n,
			time.Since(tStart))))
	}
}

// }}}

// {{{ generateComplaintsCSV

// Generates a report and puts in GCS

func generateComplaintsCSV(cdb complaintdb.ComplaintDB, s,e time.Time) (string, int, error) {
	ctx := cdb.Ctx()
	bucketname := "serfr0-reports"

	log.Printf("Starting generateComplaintsCSV: %s -> %s", s, e)

	filename := s.Format("complaints-20060102") + e.Format("-20060102.csv")
	gcsName := "gs://"+bucketname+"/"+filename

	if exists,err := gcs.Exists(ctx, bucketname, filename); err != nil {
		return gcsName,0,fmt.Errorf("gcs.Exists=%v for gs://%s/%s (err=%v)", exists, bucketname, filename, err)
	} else if exists {
		return gcsName,0,nil
	}

	gcsHandle,err := gcs.OpenRW(ctx, bucketname, filename, "text/csv")
	if err != nil {
		return gcsName,0,err
	}

	w := gcsHandle.IOWriter()

	n, err := writeCSV(cdb, s, e, w)

	if err := gcsHandle.Close(); err != nil {
		return gcsName,0,err
	}

	log.Printf("monthly CSV successfully written to %s, %d rows", gcsName, n)

	return gcsName,n,nil
}

// }}}
// {{{ generateComplaintsCSVZip

// Generates a report and puts in GCS, Zipped; also emails it to flysfo

func generateComplaintsCSVZip(cdb complaintdb.ComplaintDB, s,e time.Time) (string, int, error) {
	ctx := cdb.Ctx()
	bucketname := "serfr0-reports"
	
	log.Printf("Starting generateComplaintsCSV: %s -> %s", s, e)

	innerFilename := s.Format("complaints-20060102") + e.Format("-20060102.csv")
	zipFilename := innerFilename + ".zip"
	gcsName := "gs://"+bucketname+"/"+zipFilename

	if exists,err := gcs.Exists(ctx, bucketname, zipFilename); err != nil {
		return gcsName,0,fmt.Errorf("gcs.Exists=%v for gs://%s/%s (err=%v)", exists, bucketname, zipFilename, err)
	} else if exists {
		return gcsName,0,nil
	}

	gcsHandle,err := gcs.OpenRW(ctx, bucketname, zipFilename, "application/zip")
	if err != nil {
		return gcsName,0,err
	}

	// The two destinations for our zip
	gcsWriter := gcsHandle.IOWriter()
	var buf bytes.Buffer
	multiW := io.MultiWriter(gcsWriter, &buf)

	zipper := zip.NewWriter(multiW)
	w, err := zipper.Create(innerFilename)
	if err != nil {
		return gcsName,0,err
	}

	n, err := writeCSV(cdb, s, e, w)

	if err := zipper.Close(); err != nil {
		return gcsName,0,err
	}

	if err := gcsHandle.Close(); err != nil {
		return gcsName,0,err
	}

	log.Printf("monthly CSV.zip successfully written to %s, %d rows", gcsName, n)

	base64content := base64.StdEncoding.EncodeToString(buf.Bytes())
	subject := fmt.Sprintf("stop.jetnoise: %s", zipFilename)
	recips := []string{"Bert.Ganoung@flysfo.com", "Dave.Ong@flysfo.com", "adam@jetnoise.net"}
	from := "adam@jetnoise.net"
	
	if err := sendGCSViaEmail(zipFilename, base64content, recips, from, subject); err != nil {
		log.Printf("monthly email send failed: %s\n", err)
	}
	log.Printf("monthly CSV.zip successfully emailed to %q", recips)
	
	return gcsName,n,nil
}

// }}}
// {{{ writeCSV

// Streams a CSV of the complaints inside the date range to the provided io.Writer

func writeCSV(cdb complaintdb.ComplaintDB, s,e time.Time, w io.Writer) (int, error) {
	// One time, at 00:00, for each day of the given month
	days := date.IntermediateMidnights(s.Add(-1 * time.Second),e)

	tStart := time.Now()
	n := 0

	for _,dayStart := range days {
		dayEnd := dayStart.AddDate(0,0,1).Add(-1 * time.Second)
		q := cdb.NewComplaintQuery().ByTimespan(dayStart, dayEnd)
		log.Printf(" writeCSV: %s - %s", dayStart, dayEnd)

		if num,err := cdb.WriteCQueryToCSV(q, w, (n==0)); err != nil {
			return 0,fmt.Errorf("failed; time since start: %s. Err: %v", time.Since(tStart), err)
		} else {
			n += num
		}
	}

	return n,nil
}

// }}}
// {{{ sendGCSViaEmail

func sendGCSViaEmail(filename, base64content string, recips []string, from, subject string) error {

	to := mailjet.RecipientsV31{}
	for _, recip := range recips {
		to = append(to, mailjet.RecipientV31 {Email: recip})
	}

	messagesInfo := []mailjet.InfoMessagesV31 {
    mailjet.InfoMessagesV31{
      From: &mailjet.RecipientV31{
        Email: from,
      },

      To: &to,
      Subject: subject,
			TextPart: "Hi, SFO Noise Abatement !\n\nPlease find attached some reports from stop.jetnoise.\n\n - Adam",

			Attachments: &mailjet.AttachmentsV31{
				mailjet.AttachmentV31{
					ContentType: "application/zip",
					Filename: filename,
					Base64Content: base64content,
				},
			},
		},
  }

	messages := mailjet.MessagesV31{Info: messagesInfo}

  client := mailjet.NewMailjetClient(config.Get("mailjet.apikey"), config.Get("mailjet.privatekey"))
  resp, err := client.SendMailV31(&messages)

	log.Printf("Sent email; response was:-\n--=-\n%#v\n--=-\n", resp)

	return err
}

// }}}


// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
