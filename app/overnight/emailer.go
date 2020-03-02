package main

// Please REVISE THE URL STEM, and fixup the HackTemplates

import(
	"bytes"
	"fmt"
	"net/http"
	"time"

	 mailjet "github.com/mailjet/mailjet-apiv3-go"

	"github.com/skypies/util/date"

	"github.com/skypies/complaints/complaintdb"
	"github.com/skypies/complaints/complaintdb/types"
	"github.com/skypies/complaints/config"

	hackbackend "github.com/skypies/complaints/backend" // Just for HackTemplates
)

var(
	senderEmail = "reporters@jetnoise.net"
)

// {{{ emailYesterdayHandler

func emailYesterdayHandler(w http.ResponseWriter, r *http.Request) {
	start,end := date.WindowForYesterday()
	err,str := sendEmailsForTimeRange(r, start, end)

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(fmt.Sprintf("OK emailer\n\nErr: %v\n\nOutput:-\n%s", err,str)))
}

// }}}

// {{{ sendEmailsForTimeRange

func sendEmailsForTimeRange(r *http.Request, s,e time.Time) (error, string) {
	ctx := req2ctx(r)
	cdb := complaintdb.NewDB(ctx)

	str := "---- sendEmailsForTimeRange\n"
	
	// What we'd like to do, is Project("Profile.EmailAddress").Distinct().ByTimespan()
	// to just find out which email addresses sent emails during the
	// timespan. But we can't ... see cdb.GetUniqueUsersAndCountsIn()
	// for the gory details.

	// So instead we do something really dumb, which is iterate over
	// every single possible user, and count how many complaints they
	// filed in the timespan. This is a quick and free query, but dumb,
	// and doing thousands of them in sequence eats a lot of time. Sigh.
	profiles,err := cdb.LookupAllProfiles(cdb.NewProfileQuery())
	if err != nil { return err, str }

	for _,p := range profiles {
		if p.SendDailyEmailOK() == false { continue }

		if p.EmailAddress != "adam@worrall.cc" { continue }  // FIXME: remove this to go live

		var complaints = []types.Complaint{}
		complaints, err = cdb.LookupAll(cdb.CQByEmail(p.EmailAddress).ByTimespan(s,e))
		if err != nil {
			cdb.Errorf("Could not get complaints [%v->%v] for <%s>: %v", s,e, p.EmailAddress, err)
		}
		if len(complaints) == 0 { continue }

		cap := types.ComplaintsAndProfile{
			Profile: p,
			Complaints: complaints,
		}

		err := sendEmail(cap)

		str += fmt.Sprintf(" * %-50.50s : % 3d (%v)\n", p.EmailAddress, len(complaints), err)
	}

	return nil, str
}

// }}}
// {{{ sendEmail

func sendEmail(cap types.ComplaintsAndProfile) error {
	buf := new(bytes.Buffer)	

	if err := hackbackend.HackTemplates.ExecuteTemplate(buf, "email-bundle", cap); err != nil {
		return err
	}

	subject := fmt.Sprintf("TESTING Daily report summary for %s", cap.Profile.FullName)
	
  client := mailjet.NewMailjetClient(config.Get("mailjet.apikey"), config.Get("mailjet.privatekey"))

  messagesInfo := []mailjet.InfoMessagesV31 {
    mailjet.InfoMessagesV31{
      From: &mailjet.RecipientV31{
        Email: senderEmail,
      },
      To: &mailjet.RecipientsV31{
        mailjet.RecipientV31 {
          Email: cap.Profile.EmailAddress,
        },
      },
      Subject: subject,
      HTMLPart: buf.String(),
    },
  }

  messages := mailjet.MessagesV31{Info: messagesInfo}
  _,err := client.SendMailV31(&messages)

	return err
}

// }}}


// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
