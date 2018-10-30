package complaints

import (
	"fmt"
	"html/template"
	"net/http"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/user"

	"github.com/skypies/util/date"
	"github.com/skypies/complaints/complaintdb"
	"github.com/skypies/complaints/complaintdb/types"
	"github.com/skypies/complaints/fb"
	"github.com/skypies/complaints/g"
	"github.com/skypies/complaints/ui"
)

var(
	// Whenever a handler is required to have a session, but doesn't have one, it will
	// invoke this handler instead.
	fallbackHandler = landingPageHandler
)

func init() {
	http.HandleFunc("/",       ui.WithCtxTlsSession(rootHandler, fallbackHandler))
	http.HandleFunc("/masq",   ui.WithCtxTlsSession(masqueradeHandler, fallbackHandler))
	http.HandleFunc("/logout", ui.WithCtx(logoutHandler))

	http.HandleFunc("/faq",    faqHandler)
	http.HandleFunc("/intro",  gettingStartedHandler)
	
	http.HandleFunc("/zip",                     makeRedirectHandler("/report/zip"))
	http.HandleFunc("/personal-report/results", makeRedirectHandler("/personal-report"))

	ui.InitSessionStore(kSessionsKey,kSessionsPrevKey)
}

// {{{ HintedComplaints

// A complaint, plus hints about how to render it
type HintedComplaint struct {
	C types.Complaint
	BestIdent string
	Notes []string
	Omit bool
}

func hintComplaints(in []types.Complaint, isSuperHinter bool) []HintedComplaint {
	out := []HintedComplaint{}
	
	for _,c := range in {
		hc := HintedComplaint{C: c}

		if c.AircraftOverhead.FlightNumber != "" {
			hc.BestIdent = c.AircraftOverhead.BestIdent()
		}
		
		if c.Description != "" {
				hc.Notes = append(hc.Notes,fmt.Sprintf("Your notes: %s", c.Description))
		}
		if c.Version >= 2 {
			if c.HeardSpeedbreaks {hc.Notes = append(hc.Notes, "Flight used speedbrakes")}
			if c.Loudness == 2 {hc.Notes = append(hc.Notes, "Flight was LOUD")}
			if c.Loudness == 3 {hc.Notes = append(hc.Notes, "Flight was INSANELY LOUD")}
			if c.Activity != "" {
				hc.Notes = append(hc.Notes,fmt.Sprintf("Activity disturbed: %s", c.Activity))
			}	
		}

		out = append(out, hc)
	}

	return out
}

// }}}

// {{{ landingPageHandler

func landingPageHandler (ctx context.Context, w http.ResponseWriter, r *http.Request) {	
	fb.AppId = kFacebookAppId
	fb.AppSecret = kFacebookAppSecret
	loginUrls := map[string]string{
		"googlefromscratch": g.GetLoginUrl(r, true),
		"google": g.GetLoginUrl(r, false),
		"facebook": fb.GetLoginUrl(r),
	}

	if err := templates.ExecuteTemplate(w, "landing", loginUrls); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
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
	sesh,_ := ui.GetUserSession(ctx)
	
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

	modes["admin"] = user.Current(ctx)!=nil && user.Current(ctx).Admin
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
	ui.OverwriteSessionToNil(ctx, w, r)
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

	log.Infof(ctx, "masq into [%s]", email)

	ui.CreateSession(ctx, w, r, ui.UserSession{Email:email})
	
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

// {{{ rootHandler

/*

func rootHandler (ctx context.Context, w http.ResponseWriter, r *http.Request) {
	if r.URL.Scheme == "http" {
		safeUrl := r.URL
		safeUrl.Scheme = "https"
		http.Redirect(w, r, safeUrl.String(), http.StatusFound)
		return
	}

	cdb := complaintdb.NewDB(ctx)
	sesh,_ := GetUserSession(ctx)

	cdb.Debugf("root_001", "session obtained")
	for _,c := range r.Cookies() {
		cdb.Debugf("root_002", "cookie %s: expires=%s (remain=%s), maxage=%v, secure=%v, httponly=%v %s",
			c.Name, c.Expires.String(), time.Until(c.Expires), c.MaxAge, c.Secure, c.HttpOnly, c)
	}
	cdb.Debugf("root_002", "num cookies: %d", len(r.Cookies()))
	cdb.Debugf("root_002a", "Cf-Connecting-Ip: %s", r.Header.Get("Cf-Connecting-Ip"))
	
	// No session ? Get them to login
	if sesh.Email == "" {
		fb.AppId = kFacebookAppId
		fb.AppSecret = kFacebookAppSecret
		loginUrls := map[string]string{
			"googlefromscratch": g.GetLoginUrl(r, true),
			"google": g.GetLoginUrl(r, false),
			"facebook": fb.GetLoginUrl(r),
		}

		if err := templates.ExecuteTemplate(w, "landing", loginUrls); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	if sesh.HasCreatedAt() {
		cdb.Debugf("root_003", "tstamp=%s, age=%s", sesh.CreatedAt, time.Since(sesh.CreatedAt))
	}
	
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

	modes["admin"] = user.Current(ctx)!=nil && user.Current(ctx).Admin
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

*/

// }}}


// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
