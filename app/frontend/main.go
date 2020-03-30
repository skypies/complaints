package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"golang.org/x/net/context"

	"github.com/skypies/util/date"
	"github.com/skypies/complaints/complaintdb"
	"github.com/skypies/complaints/complaintdb/types"
	"github.com/skypies/complaints/config"
	"github.com/skypies/complaints/flightid"
	"github.com/skypies/complaints/login"
	"github.com/skypies/complaints/ui"
)

var(
	// Whenever a handler is required to have a session, but doesn't have one, it will
	// invoke this handler instead.
	fallbackHandler = landingPageHandler
)

func init() {
	ui.InitSessionStore(config.Get("sessions.key"), config.Get("sessions.prevkey"))

	login.OnSuccessCallback = func(w http.ResponseWriter, r *http.Request, email string) error {
		ui.CreateSession(r.Context(), w, r, ui.UserSession{Email:email})
		return nil
	}
	//login.Host                  = "https://frontend-dot-serfr0-1000.appspot.com"
	login.Host                  = "https://stop.jetnoise.net"
	login.RedirectUrlStem       = "/login" // oauth2 callbacks will register  under here
	login.AfterLoginRelativeUrl = "/" // where the user finally ends up, after being logged in
	login.Init()

	http.HandleFunc("/",                      ui.WithCtxTlsSession(rootHandler, fallbackHandler))
	http.HandleFunc("/masq",                  ui.WithCtxTlsAdmin(masqueradeHandler))
	http.HandleFunc("/logout",                ui.WithCtx(logoutHandler))
	http.HandleFunc("/faq",                   faqHandler)
	http.HandleFunc("/intro",                 gettingStartedHandler)
	http.HandleFunc("/down",                  flatPageHandler)

	http.HandleFunc("/cdb/list",              ui.WithCtxTlsAdmin(listUsersComplaintsHandler))
	http.HandleFunc("/cdb/airspace",          ui.HasAdmin(flightid.AirspaceHandler))
	http.HandleFunc("/cdb/comp/debug",        ui.HasAdmin(complaintdb.ComplaintDebugHandler))

	http.HandleFunc("/download-complaints",   ui.WithCtxTlsSession(DownloadHandler,fallbackHandler))
	http.HandleFunc("/personal-report",       ui.WithCtxTlsSession(personalReportHandler,fallbackHandler))
	http.HandleFunc("/personal-report/results", makeRedirectHandler("/personal-report"))

	http.HandleFunc("/profile",               ui.WithCtxTlsSession(profileFormHandler,fallbackHandler))
	http.HandleFunc("/profile-update",        ui.WithCtxTlsSession(profileUpdateHandler,fallbackHandler))
	http.HandleFunc("/profile-buttons",       ui.WithCtxTlsSession(profileButtonsHandler,fallbackHandler))
	http.HandleFunc("/profile-button-add",    ui.WithCtxTlsSession(profileButtonAddHandler,fallbackHandler))
	http.HandleFunc("/profile-button-delete", ui.WithCtxTlsSession(profileButtonDeleteHandler,fallbackHandler))

	http.HandleFunc("/button",                buttonHandler)
	http.HandleFunc("/add-complaint",         ui.WithCtxTlsSession(addComplaintHandler, fallbackHandler))
	http.HandleFunc("/add-historical-complaint", ui.WithCtxTlsSession(addHistoricalComplaintHandler,fallbackHandler))
	http.HandleFunc("/update-complaint",      ui.WithCtxTlsSession(updateComplaintHandler,fallbackHandler))
	http.HandleFunc("/delete-complaints",     ui.WithCtxTlsSession(deleteComplaintsHandler,fallbackHandler))
	http.HandleFunc("/view-complaint",        ui.WithCtxTlsSession(viewComplaintHandler,fallbackHandler))
	http.HandleFunc("/complaint-updateform",  ui.WithCtxTlsSession(complaintUpdateFormHandler,fallbackHandler))

	http.HandleFunc("/heatmap",               heatmapHandler)
	http.HandleFunc("/aws-iot",               awsIotHandler)
	http.HandleFunc("/stats",                 statsHandler)
	http.HandleFunc("/complaints-for",        complaintsForFlightHandler)
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
	var params = map[string]interface{}{
		"google": login.Goauth2.GetLoginUrl(w,r),
		"googlefromscratch": login.Goauth2.GetLogoutUrl(w,r),
		"facebook": login.Fboauth2.GetLoginUrl(w,r),
	}

	if err := templates.ExecuteTemplate(w, "landing", params); err != nil {
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

	// FIXME: how to check if complaints user is admin user / 
	//modes["admin"] = user.Current(ctx)!=nil && user.Current(ctx).Admin
	//modes["superuser"] = modes["admin"]

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

	log.Printf("masq into [%s]", email)

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

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
