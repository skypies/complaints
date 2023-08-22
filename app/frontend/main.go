package main

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http/httputil"
	"io"
	"log"
	"net/http"
	"time"

	"golang.org/x/net/context"

	"github.com/skypies/util/date"
	hw "github.com/skypies/util/handlerware"
	"github.com/skypies/util/login"	

	"github.com/skypies/complaints/pkg/complaintdb"
)

// This is loopy. Need a real logging system.
func logPrintf(r *http.Request, fmtstr string, varargs ...interface{}) {
	payload := fmt.Sprintf(fmtstr, varargs...)
	prefix := fmt.Sprintf("ip:%s", r.Header.Get("x-appengine-user-ip"))
	log.Printf("%s %s", prefix, payload)
}

// {{{ HintedComplaints

// A complaint, plus hints about how to render it
type HintedComplaint struct {
	C complaintdb.Complaint
	BestIdent string
	Notes []string
	Omit bool
}

func hintComplaints(in []complaintdb.Complaint, isSuperHinter bool) []HintedComplaint {
	out := []HintedComplaint{}
	
	for _,c := range in {
		hc := HintedComplaint{C: c}

		if c.AircraftOverhead.FlightNumber != "" {
			hc.BestIdent = c.AircraftOverhead.BestIdent()
		}
		
		if c.Description != "" {
				hc.Notes = append(hc.Notes,fmt.Sprintf("Your notes: %s", c.Description))
		}

		if c.HeardSpeedbreaks {hc.Notes = append(hc.Notes, "Flight used speedbrakes")}
		if c.Loudness == 2 {hc.Notes = append(hc.Notes, "Flight was LOUD")}
		if c.Loudness == 3 {hc.Notes = append(hc.Notes, "Flight was INSANELY LOUD")}
		if c.Activity != "" {
			hc.Notes = append(hc.Notes,fmt.Sprintf("Activity disturbed: %s", c.Activity))
		}

		out = append(out, hc)
	}

	return out
}

// }}}

// {{{ landingPageHandler

func landingPageHandler (ctx context.Context, w http.ResponseWriter, r *http.Request) {	
	// The .GetLoginURL calls mutate the response - they add cookies, which we hope to eventually
	// compare against values returned to use from the OAuth services. If those cookies get dropped,
	// then we will get the dreaded "no oauth google cookie http: named cookie not present" error
	// instead of a clean login.

	// So, let's log the entire response fopr this page, to see exactly
	// how we're dropping the cookie headers.
	var respLog bytes.Buffer
	rsp := io.MultiWriter(w, &respLog)
	
	var params = map[string]interface{}{
		"google": login.Goauth2.GetLoginUrl(w,r),
		"googlefromscratch": login.Goauth2.GetLogoutUrl(w,r),
		"facebook": login.Fboauth2.GetLoginUrl(w,r),
		"host": r.Host,
	}

	w.Header().Write(&respLog)

	template := "landing"
	if r.Host == "complaints.serfr1.org" {
		// Tell users to get off this URL - it causes the infamous cookie bug
		template = "landing-serfr1"
	}
	
	if err := templates.ExecuteTemplate(rsp, template, params); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	reqLog,_ := httputil.DumpRequest(r,true)
	
	logPrintf(r, "landingpage HTTP>>>>\n%s\n<<<<\n%s====\n", reqLog, respLog.String())
}

// }}}
// {{{ rootHandler

func rootHandler (ctx context.Context, w http.ResponseWriter, r *http.Request) {
	if r.URL.Scheme == "http" {
		safeUrl := r.URL
		safeUrl.Scheme = "https"
		http.Redirect(w, r, safeUrl.String(), http.StatusFound)
		return
	}

	cdb := complaintdb.NewDB(ctx)
	sesh,_ := hw.GetUserSession(ctx)
	
	if sesh.Email == "" {
		// "this should never happen", 'cos WithSession takes care of it all
		http.Error(w, "newRoot: invoked, but no sesh.Email", http.StatusInternalServerError)
		return
	}
	
	cdb.Debugf("root_001", "session obtained: tstamp=%s, age=%s", sesh.CreatedAt, time.Since(sesh.CreatedAt))
	
	modes := map[string]bool{}

	// The rootHandler is the URL wildcard. Except Fragments, which are broken.
	if r.URL.Path == "/full" {
		modes["expanded"] = true
	} else if r.URL.Path == "/edit" {
		modes["edit"] = true
	} else if r.URL.Path == "/debug" {
		modes["debug"] = true
	} else if r.URL.Path != "/" {
		// This is a request for apple_icon or somesuch junk. Just say no.
		http.NotFound(w, r)
		return
	}
	
	cdb.Debugf("root_004", "about get cdb.GetAllByEmailAddress")
	cap, err := cdb.GetAllByEmailAddress(sesh.Email, modes["expanded"])
	if cap==nil && err==nil {
		// No profile exists; daisy-chain into profile page
		http.Redirect(w, r, "/profile", http.StatusFound)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cdb.Debugf("root_005", "cdb.GetAllByEmailAddress done")

	modes["admin"] = sesh.IsAdmin()
	modes["superuser"] = modes["admin"]

	// Default to "", unless we had a complaint in the past hour.
	lastActivity := ""
	if len(cap.Complaints) > 0 && time.Since(cap.Complaints[0].Timestamp) < time.Hour {
		lastActivity = cap.Complaints[0].Activity
	}

	var complaintDefaults = map[string]interface{}{
		"ActivityList": kActivities,  // lives in add-complaint
		"DefaultActivity": lastActivity,
		"DefaultLoudness": 1,
		"NewForm": true,
	}

	message := ""
	disableReporting := false
	if cap.Profile.FullName == "" {
		message += "<li>We don't have your full name</li>"
		disableReporting = true
	}
	if cap.Profile.StructuredAddress.Zip == "" {
		message += "<li>We don't have an accurate address</li>"
		disableReporting = true
	}
	if message != "" {
		message = fmt.Sprintf("<p><b>We've found some problems with your profile:</b></p><ul>%s</ul>"+
			"<p> Without this data, your complaints won't be counted, so please "+
			"<a href=\"/profile\"><b>update your profile</b></a> before submitting any more complaints !</p>", message)
	}
	
	var params = map[string]interface{}{
		//"Message": template.HTML("Hi!"),
		"Cap": *cap,
		"Complaints": hintComplaints(cap.Complaints, modes["superuser"]),
		"Now": date.NowInPdt(),
		"Modes": modes,
		"ComplaintDefaults": complaintDefaults,
		"Message": template.HTML(message),
		//"Info": template.HTML("Hi!"),
		"DisableReporting": disableReporting,
	}
	
	if err := templates.ExecuteTemplate(w, "main", params); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// }}}
// {{{ logoutHandler

func logoutHandler (ctx context.Context, w http.ResponseWriter, r *http.Request) {
	hw.OverwriteSessionToNil(ctx, w, r)
	http.Redirect(w, r, "/", http.StatusFound)
}

// }}}
// {{{ masqueradeHandler

func masqueradeHandler (ctx context.Context, w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("e")
	if email == "" {
		http.Error(w, "masq needs 'e'", http.StatusInternalServerError)
		return
	}

	log.Printf("masq into [%s]", email)

	hw.CreateSession(ctx, w, r, hw.UserSession{Email:email})
	
	http.Redirect(w, r, "/", http.StatusFound)
}

// }}}

// {{{ downHandler

func downHandler (w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/down", http.StatusFound)
}

// }}}
// {{{ redirects

func makeRedirectHandler(target string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target, http.StatusFound)
	}
}

func faqHandler (w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "https://sites.google.com/a/jetnoise.net/how-to/faq", http.StatusFound)
}

func gettingStartedHandler (w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "https://sites.google.com/a/jetnoise.net/how-to/", http.StatusFound)
}

// }}}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
