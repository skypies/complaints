package complaints

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"appengine"
	"appengine/urlfetch"
	"appengine/taskqueue"
	"appengine/mail"

	"github.com/skypies/date"

	"github.com/skypies/complaints/bksv"
	"github.com/skypies/complaints/complaintdb"
	"github.com/skypies/complaints/complaintdb/types"
	"github.com/skypies/complaints/sessions"
)

const(
	kSenderEmail = "reporters@jetnoise.net"
	kAdminEmail = "adam-serfr1@worrall.cc"
	kOfficalComplaintEmail = "sfo.noise@flysfo.com"
)

func init() {
	http.HandleFunc("/_ah/bounce", bounceHandler)
	http.HandleFunc("/email", emailHandler)
	//http.HandleFunc("/email-update", emailUpdateHandler)
	http.HandleFunc("/emails-for-yesterday", sendEmailsForYesterdayHandler)

	http.HandleFunc("/bksv/submit-user",    bksvSubmitUserHandler)
	http.HandleFunc("/bksv/scan-yesterday", bksvScanYesterdayHandler)
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
// {{{ SendEmailToAllUsers

func SendEmailToAllUsers(c appengine.Context, subject string) int {
	cdb := complaintdb.ComplaintDB{C: c}

	if cps,err := cdb.GetAllProfiles(); err != nil {
		c.Errorf("SendEmailToAllUsers/GetAllProfiles: %v", err)
		return 0

	} else {
		buf := new(bytes.Buffer)	
		params := map[string]interface{}{}
		if err := templates.ExecuteTemplate(buf, "email-update", params); err != nil {
			return 0
		}

		n := 0
		for _,cp := range cps {

			// This message update goes only to the opt-outers ...
			if cp.CcSfo == true && cp.CallerCode != "WOR005" { continue }

			msg := &mail.Message{
				Sender:   kSenderEmail,
				ReplyTo:  kSenderEmail,
				To:       []string{cp.EmailAddress},
				Subject:  subject,
				HTMLBody: buf.String(),
			}
			if err := mail.Send(c, msg); err != nil {
				c.Errorf("Could not send useremail to <%s>: %v", cp.EmailAddress, err)
			}
			n++
		}
		return n
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
		ReplyTo:  cap.Profile.EmailAddress,
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

func SendComplaintsWithSpan(c appengine.Context, start,end time.Time) (err error) {
	c.Infof("--- Emails, %s -> %s", start, end)

	blacklist := map[string]bool{}
	for _,e := range blacklistAddrs { blacklist[e] = true }
	
	cdb := complaintdb.ComplaintDB{C:c, Memcache:true}
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

		// Emailing disabled; last run was Fri Oct 9, 4am, with data for Oct 8. BKSV is now live.
		if false {
			if cp.CcSfo == true {
				for _,complaint := range complaints {
					if msg,err := GenerateSingleComplaintEmail(c, cp, complaint); err != nil {
						c.Errorf("Could not generate single email to <%s>: %v", cp.EmailAddress, err)
						sent_single_fail++
						continue
					} else {
						if blacklist[cp.EmailAddress] {
							sent_single_fail++
						} else {
							if err := mail.Send(c, msg); err != nil {
								c.Errorf("Could not send email to <%s>: %v", cp.EmailAddress, err)
								sent_single_fail++
								continue
							} else {
								sent_single_ok++
							}
						}
					}
				}
			}
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

		useGateway := false
		
		if useGateway {
			if err = sendViaHTTPGateway(c, msg); err != nil {
				c.Errorf("Could not gateway email to <%s>: %v", cp.EmailAddress, err)
				sent_fail++
				continue
			}
		} else {
			if blacklist[cp.EmailAddress] {
				sent_fail++
			} else {
				if err = mail.Send(c, msg); err != nil {
					c.Errorf("Could not send email to <%s>: %v", cp.EmailAddress, err)
					sent_fail++
					continue
				}
			}
		}

		if cap.Profile.CcSfo == true {
			complaints_submitted += len(cap.Complaints)
		} else {
			complaints_private += len(cap.Complaints)
		}
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
	
	c.Infof("--- email wrapup: %d ok, %d fail (%d no data) : %d reports submitted (%d kept back)  single[%d/%d]",
		sent_ok, sent_fail, no_data, complaints_submitted, complaints_private, sent_single_ok, sent_single_fail)
	
	return
}

// }}}

// {{{ sendEmailsForWindow

func sendEmailsForWindow(w http.ResponseWriter, r *http.Request, start,end time.Time) {
	c := appengine.NewContext(r)
	if err := SendComplaintsWithSpan(c, start, end); err != nil {
		c.Errorf("Couldn't send email: %v", err)
	}
	w.Write([]byte("OK"))
}

// }}}

// {{{ bounceHandler

func bounceHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	c.Errorf("Received a bounce: %v", r)
	w.Write([]byte("OK"))
}

// }}}
// {{{ emailHandler

func emailHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	session := sessions.Get(r)
	cdb := complaintdb.ComplaintDB{C: c}

	cp, err := cdb.GetProfileByEmailAddress(session.Values["email"].(string))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	start,end := date.WindowForYesterday()
	// end = time.Now()
	complaints, err2 := cdb.GetComplaintsInSpanByEmailAddress(cp.EmailAddress, start, end)
	if err2 != nil {
		http.Error(w, err2.Error(), http.StatusInternalServerError)
		return
	}

	var cap = types.ComplaintsAndProfile{
		Profile: *cp,
		Complaints: complaints,
	}

	if err := templates.ExecuteTemplate(w, "email-update", map[string]interface{}{}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else {
		return
	}
	
	msg,err3 := GenerateEmail(c, cap)
	if err3 != nil {
		http.Error(w, err3.Error(), http.StatusInternalServerError)
		return
	}
	cap.Profile.CcSfo = true
	if len(cap.Complaints) == 0 {
		http.Error(w, "No complaints found ?!", http.StatusInternalServerError)
		return
	}
	msg2,err4 := GenerateSingleComplaintEmail(c, cap.Profile, cap.Complaints[len(cap.Complaints)-1])
	if err4 != nil {
		http.Error(w, err4.Error(), http.StatusInternalServerError)
		return
	}
	
	var params = map[string]interface{}{
		"Cap": cap,
		"EmailBundle": msg,
		"EmailSingle": msg2,
		"EmailBundleBody": template.HTML(msg.HTMLBody),
		"EmailSingleBody": template.HTML(msg2.HTMLBody),
	}

	/* if err4 := sendViaHTTPGateway(c, msg); err4 != nil {
		http.Error(w, err4.Error(), http.StatusInternalServerError)
		return
	} */
	
	if err := templates.ExecuteTemplate(w, "email-debug", params); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// }}}
// {{{ sendEmailsForYesterdayHandler

func sendEmailsForYesterdayHandler(w http.ResponseWriter, r *http.Request) {
	start,end := date.WindowForYesterday()
	sendEmailsForWindow(w, r, start, end)
}

// }}}
// {{{ emailUpdateHandler

func emailUpdateHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	subject := "Stop.jetnoise.net: a proposal to auto-submit your complaints"
	n := SendEmailToAllUsers(c, subject)

	w.Write([]byte(fmt.Sprintf("Email update, OK (%d)\n", n)))
}

// }}}

// {{{ bksvSubmitUserHandler

func bksvSubmitUserHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	cdb := complaintdb.ComplaintDB{C:c, Memcache:true}
	start,end := date.WindowForYesterday()
	bksv_ok,bksv_not_ok := 0,0

	email := r.FormValue("user")

	if cp,err := cdb.GetProfileByEmailAddress(email); err != nil {
		c.Errorf(" /bksv/submit-user(%s): getprofile: %v", email, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	
	} else if complaints,err := cdb.GetComplaintsInSpanByEmailAddress(email, start, end); err != nil {
		c.Errorf(" /bksv/submit-user(%s): getcomplaints: %v", email, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return

	} else {
		for i,complaint := range complaints {
			time.Sleep(time.Millisecond * 200)
			if debug,err := bksv.PostComplaint(urlfetch.Client(c), *cp, complaint); err != nil {
				//cdb.C.Infof("pro: %v", cp)
				//cdb.C.Infof("comp: %#v", complaint)
				cdb.C.Errorf("BKSV posting error: %v", err)
				cdb.C.Infof("BKSV Debug\n------\n%s\n------\n", debug)
				bksv_not_ok++
			} else {
				if (i == 0) { cdb.C.Infof("BKSV [OK] Debug\n------\n%s\n------\n", debug) }
				bksv_ok++
			}
		}
	}

	c.Infof("bksv for %s, %d/%d", email, bksv_ok, bksv_not_ok)
	if (bksv_not_ok > 0) {
		c.Errorf("bksv for %s, %d/%d", email, bksv_ok, bksv_not_ok)
	}
	w.Write([]byte("OK"))
}

// }}}
// {{{ bksvScanYesterdayHandler

// Examine all users. If they had any complaints, throw them in the queue.
func bksvScanYesterdayHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	cdb := complaintdb.ComplaintDB{C:c, Memcache:true}
	var cps = []types.ComplainerProfile{}
	cps, err := cdb.GetAllProfiles()
	if err != nil {
		c.Errorf(" /bksv/scan-yesterday: getallprofiles: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	start,end := date.WindowForYesterday()
	bksv_ok := 0
	
	for _,cp := range cps {
		if cp.CcSfo == false { continue }

		var complaints = []types.Complaint{}
		complaints, err = cdb.GetComplaintsInSpanByEmailAddress(cp.EmailAddress, start, end)
		if err != nil {
			c.Errorf(" /bksv/scan-yesterday: getbyemail(%s): %v", cp.EmailAddress, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		} 
		if len(complaints) > 0 {
			t := taskqueue.NewPOSTTask("/bksv/submit-user", map[string][]string{
				"user": {cp.EmailAddress},
			})
			if _,err := taskqueue.Add(c, t, "submitreports"); err != nil {
				c.Errorf(" /bksv/scan-yesterday: enqueue: %v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			bksv_ok++
		}
	}
	c.Infof("enqueued %d bksv", bksv_ok)
	w.Write([]byte(fmt.Sprintf("OK, enqueued %d", bksv_ok)))
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
