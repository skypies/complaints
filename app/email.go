package main

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
	ctx := req2ctx(r)
	cdb := complaintdb.NewDB(ctx)

	if cps,err := cdb.LookupAllProfiles(cdb.NewProfileQuery()); err != nil {
		cdb.Errorf("SendEmailToAllUsers/GetProfiles: %v", err)
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

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
