package complaints

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"regexp"
	"time"
	
	"appengine"
	"appengine/user"

	"github.com/skypies/date"
	
	"github.com/skypies/complaints/complaintdb"
	"github.com/skypies/complaints/complaintdb/types"
	"github.com/skypies/complaints/fb"
	"github.com/skypies/complaints/g"
	"github.com/skypies/complaints/sessions"
)

var (
	templates = template.Must(template.New("").Funcs(template.FuncMap{
		"add": templateAdd,
		"spacify": templateSpacifyFlightNumber,
		"dict": templateDict,
		"formatPdt": templateFormatPdt,
	}).ParseGlob("templates/*"))
)
func templateAdd(a int, b int) int { return a + b }
func templateSpacifyFlightNumber(s string) string {
	s2 := regexp.MustCompile("^r:(.+)$").ReplaceAllString(s, "Registration:$1")
	s3 := regexp.MustCompile("^(..)(\\d\\d\\d)$").ReplaceAllString(s2, "$1 $2")
	return regexp.MustCompile("^(..)(\\d\\d)$").ReplaceAllString(s3, "$1  $2")
}
func templateDict(values ...interface{}) (map[string]interface{}, error) {
	if len(values)%2 != 0 { return nil, errors.New("invalid dict call")	}
	dict := make(map[string]interface{}, len(values)/2)
	for i := 0; i < len(values); i+=2 {
		key, ok := values[i].(string)
		if !ok { return nil, errors.New("dict keys must be strings") }
		dict[key] = values[i+1]
	}
	return dict, nil
}
func templateFormatPdt(t time.Time, format string) string {
	return date.InPdt(t).Format(format)
}

func init() {
	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/logout", logoutHandler)
	http.HandleFunc("/faq", faqHandler)
	http.HandleFunc("/intro", gettingStartedHandler)
	http.HandleFunc("/masq", masqueradeHandler)
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

// {{{ rootHandler

func rootHandler (w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	session := sessions.Get(r)
	
	// No session ? Get them to login
	if session.Values["email"] == nil {
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
	
	cdb := complaintdb.ComplaintDB{C: c}
	cap, err := cdb.GetAllByEmailAddress(session.Values["email"].(string), modes["expanded"])
	if cap==nil && err==nil {
		// No profile exists; daisy-chain into profile page
		http.Redirect(w, r, "/profile", http.StatusFound)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	modes["admin"] = user.Current(c)!=nil && user.Current(c).Admin
	modes["superuser"] = modes["admin"] || cap.Profile.EmailAddress == "meekGee@gmail.com"

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

	var params = map[string]interface{}{
		//"Message": template.HTML("Hi!"),
		"Cap": *cap,
		"Complaints": hintComplaints(cap.Complaints, modes["superuser"]),
		"Now": date.NowInPdt(),
		"Modes": modes,
		"ComplaintDefaults": complaintDefaults,
	}
	
	if err := templates.ExecuteTemplate(w, "main", params); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// }}}

// {{{ logoutHandler

func logoutHandler (w http.ResponseWriter, r *http.Request) {
	session := sessions.Get(r)
	session.Values["email"] = nil
	session.Save(r, w)	
	http.Redirect(w, r, "/", http.StatusFound)
}

// }}}
// {{{ masqueradeHandler

func masqueradeHandler (w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	email := r.FormValue("e")
	if email == "" {
		http.Error(w, "masq needs 'e'", http.StatusInternalServerError)
		return
	}

	c.Infof("masq into [%s]", email)

	session := sessions.Get(r)
	session.Values["email"] = email
	session.Save(r,w)
	
	http.Redirect(w, r, "/", http.StatusFound)
}

// }}}
// {{{ downHandler

func downHandler (w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/down", http.StatusFound)
}

// }}}

// {{{ introHandler

func faqHandler (w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "https://sites.google.com/a/jetnoise.net/how-to/faq", http.StatusFound)
}

// }}}
// {{{ introHandler

func gettingStartedHandler (w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "https://sites.google.com/a/jetnoise.net/how-to/", http.StatusFound)
}

// }}}


// {{{ junk

/*

	/* Is it saturday ?
	pdt2, _ := time.LoadLocation("America/Los_Angeles")
	if time.Now().In(pdt2).Day() == 28 {
		http.Redirect(w, r, "/down", http.StatusFound)
		return
	} */

// }}}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
