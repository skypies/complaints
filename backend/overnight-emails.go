package backend

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"appengine"
	"appengine/urlfetch"
	"appengine/mail"

	"github.com/skypies/util/date"

	"github.com/skypies/complaints/complaintdb"
	"github.com/skypies/complaints/complaintdb/types"
)

const(
	kSenderEmail = "reporters@jetnoise.net"
	kAdminEmail = "adam-serfr1@worrall.cc"
	kOfficalComplaintEmail = "sfo.noise@flysfo.com"
)

func init() {
	http.HandleFunc("/backend/emails-for-yesterday", sendEmailsForYesterdayHandler)
}	

// {{{ SendEmailToAdmin

func SendEmailToAdmin(c appengine.Context, subject, htmlbody string) {
	msg := &mail.Message{

		Sender:   kSenderEmail, // cap.Profile.EmailAddress,
		To:       []string{kAdminEmail},
		Subject:  subject,
		HTMLBody: htmlbody,
	}

	if err := mail.Send(c, msg); err != nil {
		c.Errorf("Could not send adminemail to <%s>: %v", kAdminEmail, err)
	}
}

// }}}

// {{{ GenerateSingleComplaintEmail

func GenerateSingleComplaintEmail(c appengine.Context, profile types.ComplainerProfile, complaint types.Complaint) (*mail.Message, error) {
	if profile.CcSfo == false {
		return nil, fmt.Errorf("singlecomplaint called, but CcSFO false")
	}

	// OK, let's parse the hell out of things ...
	speedbreaks := ""
	if complaint.HeardSpeedbreaks { speedbreaks = "Speedbrakes used" }

	airline := ""
	notes := ""
	if complaint.Loudness == 2 {
		notes = "Very loud. "
	} else if complaint.Loudness == 3 {
		notes = "Incredibly loud. "
	}
	if complaint.AircraftOverhead.FlightNumber != "" {
		notes += "Flight believed to be " + complaint.AircraftOverhead.FlightNumber + ". "
		airline = regexp.MustCompile("^(..)\\d+$").ReplaceAllString(complaint.AircraftOverhead.FlightNumber, "$1")
	}
	notes += complaint.Description

	zip := regexp.MustCompile("^.*(\\d{5}(-\\d{4})?).*$").ReplaceAllString(profile.Address, "$1")
	
	buf := new(bytes.Buffer)	
	params := map[string]interface{}{
		"Profile": profile,
		"Complaint": complaint,
		"Operation": speedbreaks,
		"Airline": airline,
		"Notes": notes,
		"Zip": zip,
	}
	if err := templates.ExecuteTemplate(buf, "email-single", params); err != nil {
		return nil,err
	}

	msg := &mail.Message{
		ReplyTo:  profile.EmailAddress,
		Sender:   kSenderEmail,
		To:       []string{kOfficalComplaintEmail},
//		Bcc:      []string{"complainers+bcc@serfr1.org"},
		Subject:  fmt.Sprintf("An SFO.NOISE complaint from %s", profile.FullName),
		HTMLBody: buf.String(),
	}
	
	return msg, nil
}

// }}}
// {{{ GenerateEmail

func GenerateEmail(c appengine.Context, cap types.ComplaintsAndProfile) (*mail.Message, error) {
	buf := new(bytes.Buffer)	
	err := templates.ExecuteTemplate(buf, "email-bundle", cap)
	if err != nil { return nil,err }

	var bcc = []string{
		fmt.Sprintf("complainers+bcc@serfr1.org"), //, cap.Profile.CallerCode),
	}
	var dests = []string{
		cap.Profile.EmailAddress,
	}
	//if cap.Profile.CcSfo == true {
	//	dests = append(dests, kOfficalComplaintEmail)
	//}
	
	// In ascending order
	sort.Sort(sort.Reverse(types.ComplaintsByTimeDesc(cap.Complaints)))

	subject := fmt.Sprintf("Daily report summary for %s", cap.Profile.FullName)
	if cap.Profile.CallerCode != "" {
		subject = fmt.Sprintf("Daily report summary for [%s]", cap.Profile.CallerCode)
	}

	msg := &mail.Message{
		ReplyTo:  kSenderEmail, // cap.Profile.EmailAddress,
		Sender:   kSenderEmail, // cap.Profile.EmailAddress,
		To:       dests,
		Bcc:      bcc,
		Subject:  subject,
		HTMLBody: buf.String(),
	}

	return msg, nil
}

// }}}

// {{{ SendComplaintsWithSpan

var blacklistAddrs = []string{}

func SendComplaintsWithSpan(c appengine.Context, start,end time.Time) (err error, str string) {
	c.Infof("--- Emails, %s -> %s", start, end)

	blacklist := map[string]bool{}
	for _,e := range blacklistAddrs { blacklist[e] = true }
	
	cdb := complaintdb.ComplaintDB{C:c, Req:r, Memcache:true}
	var cps = []types.ComplainerProfile{}
	cps, err = cdb.GetAllProfiles()
	if err != nil { return }

	complaints_private,complaints_submitted,no_data,sent_ok,sent_fail := 0,0,0,0,0
	sent_single_ok,sent_single_fail := 0,0
	
	for _,cp := range cps {
		var complaints = []types.Complaint{}
		complaints, err = cdb.GetComplaintsInSpanByEmailAddress(cp.EmailAddress, start, end)

		if err != nil {
			c.Errorf("Could not get complaints [%v->%v] for <%s>: %v", start, end, cp.EmailAddress, err)
			no_data++
			continue
		}
		if len(complaints) == 0 {
			no_data++
			continue
		}
		
		var cap = types.ComplaintsAndProfile{
			Profile: cp,
			Complaints: complaints,
		}

		var msg *mail.Message
		if msg,err = GenerateEmail(c,cap); err != nil {
			c.Errorf("Could not generate email to <%s>: %v", cp.EmailAddress, err)
			sent_fail++
			continue
		}

		if blacklist[cp.EmailAddress] {
			sent_fail++
		} else {
			if err = mail.Send(c, msg); err != nil {
				c.Errorf("Could not send email to <%s>: %v", cp.EmailAddress, err)
				sent_fail++
				continue
			}
		}

		complaints_submitted += len(cap.Complaints)
		sent_ok++
	}

	subject := fmt.Sprintf("Daily report stats: users:%d/%d  reports:%d/%d  emails:%d:%d",
		sent_ok, (sent_ok+no_data),
		complaints_submitted, (complaints_submitted+complaints_private),
		sent_single_ok, sent_single_fail)

	SendEmailToAdmin(c, subject, "")

	dc := complaintdb.DailyCount{
		Datestring: date.Time2Datestring(start.Add(time.Hour)),
		NumComplaints: complaints_submitted+complaints_private,
		NumComplainers: sent_ok,
	}
	cdb.AddDailyCount(dc)

	str = fmt.Sprintf("email wrapup: %d ok, %d fail (%d no data) : %d reports submitted (%d kept back)  single[%d/%d]",sent_ok, sent_fail, no_data, complaints_submitted, complaints_private, sent_single_ok, sent_single_fail)
	
	c.Infof("--- %s", str)

	return
}

// }}}

// {{{ sendEmailsForWindow

func sendEmailsForWindow(w http.ResponseWriter, r *http.Request, start,end time.Time) {
	c := appengine.NewContext(r)
	err,deb := SendComplaintsWithSpan(c, start, end)

	if err != nil {
		c.Errorf("Couldn't send email: %v", err)
		w.Write([]byte(fmt.Sprintf("Not OK: %v\n%s\n", err, deb)))
	} else {
		w.Write([]byte(fmt.Sprintf("OK\n%s\n", deb)))
	}
}

// }}}
// {{{ sendEmailsForYesterdayHandler

func sendEmailsForYesterdayHandler(w http.ResponseWriter, r *http.Request) {
	start,end := date.WindowForYesterday()
	sendEmailsForWindow(w, r, start, end)
}

// }}}

// {{{ sendViaHTTPGateway

func sendViaHTTPGateway(c appengine.Context, msg *mail.Message) error {
	client := urlfetch.Client(c)

	gatewayUrl := "http://worrall.cc/cgi-bin/gae-bites"
	data := url.Values{
		"pwqiry":  {"o0ashaknjsof81boaskjal2dfpuigskguwfgl8xgfo"},
		"to":      {strings.Join(msg.To, ",")},
		"bcc":     {strings.Join(msg.Bcc, ",")},
		"replyto": {msg.ReplyTo},
		"subject": {msg.Subject},
		"body":    {msg.HTMLBody},
	}
	
	resp, err := client.PostForm(gatewayUrl, data)
	if err != nil {	return err }

	c.Infof("Mail gateway response:-\n%v", resp)

	return nil
}

// }}}
	
// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
