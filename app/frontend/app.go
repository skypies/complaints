package main

import(
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"golang.org/x/net/context"

	hw "github.com/skypies/util/handlerware"
	"github.com/skypies/util/login"	

	fdbui "github.com/skypies/flightdb/ui"

	"github.com/skypies/complaints/pkg/complaintdb"
	"github.com/skypies/complaints/pkg/config"
	"github.com/skypies/complaints/pkg/flightid"
)


var(
	templates *template.Template
)

func init() {
  hw.InitTemplates("app/frontend/web/templates") // Must be relative to module root, i.e. git repo root
	templates = hw.Templates

	hw.RequireTls = true
  hw.CtxMakerCallback = req2ctx

	hw.CookieName = "serfr0"
	hw.InitSessionStore(config.Get("sessions.key"), config.Get("sessions.prevkey"))
  hw.NoSessionHandler = landingPageHandler
  hw.InitGroup(hw.AdminGroup, config.Get("users.admin"))

	login.OnSuccessCallback = func(w http.ResponseWriter, r *http.Request, email string) error {
		hw.CreateSession(r.Context(), w, r, hw.UserSession{Email:email})
		return nil
	}
	login.Host                  = "https://stop.jetnoise.net"
	login.RedirectUrlStem       = "/login" // oauth2 callbacks will register  under here
	login.AfterLoginRelativeUrl = "/" // where the user finally ends up, after being logged in
	login.GoogleClientID        = config.Get("google.oauth2.appid")
	login.GoogleClientSecret    = config.Get("google.oauth2.secret")
	login.FacebookClientID      = config.Get("facebook.oauth2.appid")
	login.FacebookClientSecret  = config.Get("facebook.oauth2.secret")
	login.Init()

	http.HandleFunc("/",                      hw.WithSession(rootHandler))
	http.HandleFunc("/masq",                  hw.WithAdmin(masqueradeHandler))
	http.HandleFunc("/logout",                hw.WithCtx(logoutHandler))
	http.HandleFunc("/faq",                   faqHandler)
	http.HandleFunc("/intro",                 gettingStartedHandler)
	http.HandleFunc("/down",                  flatPageHandler)

	http.HandleFunc("/cdb/list",              hw.WithAdmin(listUsersComplaintsHandler))
	http.HandleFunc("/cdb/airspace",          hw.WithAdmin(hw.WithoutCtx(flightid.AirspaceHandler)))
	http.HandleFunc("/cdb/comp/debug",        hw.WithAdmin(hw.WithoutCtx(complaintdb.ComplaintDebugHandler)))

	http.HandleFunc("/download-complaints",   hw.WithSession(DownloadHandler))
	http.HandleFunc("/personal-report",       hw.WithSession(personalReportHandler))
	http.HandleFunc("/personal-report/results", makeRedirectHandler("/personal-report"))

	http.HandleFunc("/profile",               hw.WithSession(profileFormHandler))
	http.HandleFunc("/profile-update",        hw.WithSession(profileUpdateHandler))
	http.HandleFunc("/profile-buttons",       hw.WithSession(profileButtonsHandler))
	http.HandleFunc("/profile-button-add",    hw.WithSession(profileButtonAddHandler))
	http.HandleFunc("/profile-button-delete", hw.WithSession(profileButtonDeleteHandler))

	http.HandleFunc("/button",                buttonHandler)
	http.HandleFunc("/add-complaint",         hw.WithSession(addComplaintHandler))
	http.HandleFunc("/add-historical-complaint", hw.WithSession(addHistoricalComplaintHandler))
	http.HandleFunc("/update-complaint",      hw.WithSession(updateComplaintHandler))
	http.HandleFunc("/delete-complaints",     hw.WithSession(deleteComplaintsHandler))
	http.HandleFunc("/view-complaint",        hw.WithSession(viewComplaintHandler))
	http.HandleFunc("/complaint-updateform",  hw.WithSession(complaintUpdateFormHandler))

	http.HandleFunc("/heatmap",               heatmapHandler)
	http.HandleFunc("/aws-iot",               awsIotHandler)
	http.HandleFunc("/stats",                 statsHandler)
	http.HandleFunc("/complaints-for",        complaintsForFlightHandler)

	// FIXME: move flightdb/ui over to the new handlerware, then it can pull templates out of the context
	http.HandleFunc("/map",                   hw.WithCtx(fdbui.MapHandler))
}


func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fs := http.FileServer(http.Dir("./app/frontend/web/static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	log.Printf("Listening on port %s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}

func req2ctx(r *http.Request) context.Context {
	ctx,_ := context.WithTimeout(r.Context(), 55 * time.Second)
	return ctx
}

func req2client(r *http.Request) *http.Client {
	return &http.Client{}
}

func dumpForm(r *http.Request) string {
	str := fmt.Sprintf("*** Form contents for {%s}:-\n", r.URL.Path)
	for k,v := range r.Form {
		str += fmt.Sprintf("  * %-20.20s : %v\n", k, v)
	}
	return str + "***\n"
}
