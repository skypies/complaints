package complaints

import (
	"net/http"
	"strconv"
	
	"appengine"

	"github.com/skypies/complaints/complaintdb"
	"github.com/skypies/complaints/complaintdb/types"
	"github.com/skypies/complaints/sessions"
)
//import "fmt"

func init() {
	http.HandleFunc("/profile", profileFormHandler)
	http.HandleFunc("/profile-update", profileUpdateHandler)
}

// {{{ profileFormHandler

func profileFormHandler(w http.ResponseWriter, r *http.Request) {
	// https, yay
	if r.URL.Host == "stop.jetnoise.net" {
		// We're behind cloudflare, so we always see http. This is how we can tell if the user is
		// using https ...
		if r.Header.Get("Cf-Visitor") != `{"scheme":"https"}` {
			safeUrl := r.URL
			safeUrl.Scheme = "https"
			http.Redirect(w, r, safeUrl.String(), http.StatusFound)
			return
		}
	}

	c := appengine.NewContext(r)
	session := sessions.Get(r)
	if session.Values["email"] == nil {
		http.Error(w, "session was empty; no cookie ? is this browser in privacy mode ?",
			http.StatusInternalServerError)
		return
	}
	email := session.Values["email"].(string)

	cdb := complaintdb.ComplaintDB{C: c}
	cp, _ := cdb.GetProfileByEmailAddress(email)

	if cp.EmailAddress == "" {
		// First ever visit - empty profile !
		cp.EmailAddress = email
		cp.CcSfo = true
	}

	var params = map[string]interface{}{
		"Profile": cp,
		"MapsAPIKey": kGoogleMapsAPIKey, // For autocomplete & latlong goodness
	}
	params["Message"] = r.FormValue("msg")
	
	if err := templates.ExecuteTemplate(w, "profile-edit", params); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// }}}
// {{{ profileUpdateHandler

func profileUpdateHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	session := sessions.Get(r)
	if session.Values["email"] == nil {
		c.Errorf("profileUpdate:, session was empty; no cookie ?")
		http.Error(w, "session was empty; no cookie ? is this browser in privacy mode ?",
			http.StatusInternalServerError)
		return
	}
	email := session.Values["email"].(string)

	r.ParseForm()
	
	lat,err := strconv.ParseFloat(r.FormValue("Lat"), 64)
	if err != nil {
		c.Errorf("profileUpdate:, parse lat '%s': %v", r.FormValue("Lat"), err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}		
	long,err2 := strconv.ParseFloat(r.FormValue("Long"), 64)
	if err2 != nil {
		c.Errorf("profileUpdate:, parse long '%s': %v", r.FormValue("Long"), err)
		http.Error(w, err2.Error(), http.StatusInternalServerError)
		return
	}

	// Maybe make a call to fetch the elevation ??
	// https://developers.google.com/maps/documentation/elevation/intro
	
	cp := types.ComplainerProfile{
		EmailAddress: email,
		CallerCode: r.FormValue("CallerCode"),
		FullName: r.FormValue("FullName"),
		Address: r.FormValue("AutoCompletingMagic"),
		StructuredAddress: types.PostalAddress{
			Number: r.FormValue("AddrNumber"),
			Street: r.FormValue("AddrStreet"),
			City: r.FormValue("AddrCity"),
			State: r.FormValue("AddrState"),
			Zip: r.FormValue("AddrZip"),
			Country: r.FormValue("AddrCountry"),
		},
		CcSfo: FormValueCheckbox(r, "CcSfo"),
		Lat: lat,
		Long: long,
	}
	
	cdb := complaintdb.ComplaintDB{C: c}
	err = cdb.PutProfile(cp)
	if err != nil {
		c.Errorf("profileUpdate: cdb.Put: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusFound)
}

// lat,long := 37.060312,-121.990814  // 1 Cielo

// }}}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
