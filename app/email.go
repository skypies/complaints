package complaints

import (
	"bytes"
	"fmt"
	"net/http"

	"google.golang.org/appengine"
	"google.golang.org/appengine/mail"
	"google.golang.org/appengine/log"
	"golang.org/x/net/context"

	"github.com/skypies/complaints/complaintdb"
)

const(
	kSenderEmail = "reporters@jetnoise.net"
	kAdminEmail = "adam-serfr1@worrall.cc"
)

func init() {
	http.HandleFunc("/_ah/bounce", bounceHandler)
	http.HandleFunc("/email-update", emailUpdateHandler)
	//http.HandleFunc("/emaildebug", emailDebugHandler)
}

// {{{ SendEmailToAdmin

func SendEmailToAdmin(c context.Context, subject, htmlbody string) {
	msg := &mail.Message{

		Sender:   kSenderEmail, // cap.Profile.EmailAddress,
		To:       []string{kAdminEmail},
		Subject:  subject,
		HTMLBody: htmlbody,
	}

	if err := mail.Send(c, msg); err != nil {
		log.Errorf(c, "Could not send adminemail to <%s>: %v", kAdminEmail, err)
	}
}

// }}}
// {{{ SendEmailToAllUsers

func SendEmailToAllUsers(r *http.Request, subject string) int {
	cdb := complaintdb.NewDB(r)

	if cps,err := cdb.GetAllProfiles(); err != nil {
		cdb.Errorf("SendEmailToAllUsers/GetAllProfiles: %v", err)
		return 0

	} else {
		buf := new(bytes.Buffer)	
		params := map[string]interface{}{}
		if err := templates.ExecuteTemplate(buf, "email-update", params); err != nil {
			return 0
		}

		n := 0
		for _,cp := range cps {
			msg := &mail.Message{
				Sender:   kSenderEmail,
				ReplyTo:  kSenderEmail,
				To:       []string{cp.EmailAddress},
				Subject:  subject,
				HTMLBody: buf.String(),
			}
			if err := mail.Send(cdb.Ctx(), msg); err != nil {
				cdb.Errorf("Could not send useremail to <%s>: %v", cp.EmailAddress, err)
			}
			n++
		}
		return n
	}
}

// }}}

// {{{ bounceHandler

func bounceHandler(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	log.Errorf(ctx, "Received a bounce: %v", r)
	w.Write([]byte("OK"))
}

// }}}
// {{{ emailUpdateHandler

func emailUpdateHandler(w http.ResponseWriter, r *http.Request) {
	subject := "stop.jetnoise.net news, 2016.02"
	n := SendEmailToAllUsers(r, subject)

	w.Write([]byte(fmt.Sprintf("Email update, OK (%d)\n", n)))
}

// }}}

/*
// {{{ emailDebugHandler

func emailDebugHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	session := sessions.Get(r)
	cdb := complaintdb.NewDB(r)

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
	
	if err := templates.ExecuteTemplate(w, "email-debug", params); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// }}}
*/

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
