package complaints

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"golang.org/x/net/context"

	"github.com/skypies/complaints/complaintdb"
	"github.com/skypies/complaints/complaintdb/types"
	"github.com/skypies/complaints/flightid"
	"github.com/skypies/complaints/ui"
)

func init() {
	http.HandleFunc("/profile", ui.WithCtxTlsSession(profileFormHandler,fallbackHandler))
	http.HandleFunc("/profile-update", ui.WithCtxTlsSession(profileUpdateHandler,fallbackHandler))
	http.HandleFunc("/profile-buttons", ui.WithCtxTlsSession(profileButtonsHandler,fallbackHandler))
	http.HandleFunc("/profile-button-add", ui.WithCtxTlsSession(profileButtonAddHandler,fallbackHandler))
	http.HandleFunc("/profile-button-delete", ui.WithCtxTlsSession(profileButtonDeleteHandler,fallbackHandler))
}

// {{{ profileFormHandler

func profileFormHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	// https, yay
	if r.URL.Scheme == "http" {
		// If we're behind cloudflare, this is how we can tell if the user is
		// using https ...
		//	if r.Header.Get("Cf-Visitor") != `{"scheme":"https"}` {
		safeUrl := r.URL
		safeUrl.Scheme = "https"
		http.Redirect(w, r, safeUrl.String(), http.StatusFound)
		return
	}

	sesh,_ := ui.GetUserSession(ctx)
	cdb := complaintdb.NewDB(ctx)
	cp,_ := cdb.LookupProfile(sesh.Email)
	if cp.EmailAddress == "" {
		// First ever visit - empty profile !
		cp.EmailAddress = sesh.Email
		cp.CcSfo = true
	}

	var params = map[string]interface{}{
		"Profile": cp,
		"Selectors": flightid.ListSelectors(),
		"MapsAPIKey": kGoogleMapsAPIKey, // For autocomplete & latlong goodness
	}
	params["Message"] = r.FormValue("msg")
	
	if err := templates.ExecuteTemplate(w, "profile-edit", params); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// }}}
// {{{ profileUpdateHandler

func profileUpdateHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	cdb := complaintdb.NewDB(ctx)

	r.ParseForm()
	
	lat,err := strconv.ParseFloat(r.FormValue("Lat"), 64)
	if err != nil {
		cdb.Errorf("profileUpdate:, parse lat '%s': %v", r.FormValue("Lat"), err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}		
	long,err2 := strconv.ParseFloat(r.FormValue("Long"), 64)
	if err2 != nil {
		cdb.Errorf("profileUpdate:, parse long '%s': %v", r.FormValue("Long"), err2)
		http.Error(w, err2.Error(), http.StatusInternalServerError)
		return
	}
	elev,err3 := strconv.ParseFloat(r.FormValue("Elevation"), 64)
	if err3 != nil {
		cdb.Errorf("profileUpdate:, parse elev '%s': %v", r.FormValue("Elevation"), err3)
		http.Error(w, err3.Error(), http.StatusInternalServerError)
		return
	}

	sesh,_ := ui.GetUserSession(ctx)

	// Maybe make a call to fetch the elevation ??
	// https://developers.google.com/maps/documentation/elevation/intro
	cp := types.ComplainerProfile{
		EmailAddress: sesh.Email,
		CallerCode: r.FormValue("CallerCode"),
		FullName: strings.TrimSpace(r.FormValue("FullName")),
		Address: strings.TrimSpace(r.FormValue("AutoCompletingMagic")),
		StructuredAddress: types.PostalAddress{
			Number: r.FormValue("AddrNumber"),
			Street: r.FormValue("AddrStreet"),
			City: r.FormValue("AddrCity"),
			State: r.FormValue("AddrState"),
			Zip: r.FormValue("AddrZip"),
			Country: r.FormValue("AddrCountry"),
		},
		CcSfo: true, //FormValueCheckbox(r, "CcSfo"),
		SelectorAlgorithm: r.FormValue("SelectorAlgorithm"),
		SendDailyEmail: FormValueTriValuedCheckbox(r, "SendDailyEmail"),
		DataSharing: FormValueTriValuedCheckbox(r, "DataSharing"),
		ThirdPartyComms: FormValueTriValuedCheckbox(r, "ThirdPartyComms"),

		Lat: lat,
		Long: long,
		Elevation: elev,
		ButtonId: []string{},
	}

	// Preserve some values from the old profile
	if origProfile,err := cdb.MustLookupProfile(sesh.Email); err == nil {
		cp.ButtonId = origProfile.ButtonId
	}
	
	if err := cdb.PersistProfile(cp); err != nil {
		cdb.Errorf("profileUpdate: cdb.Put: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/", http.StatusFound)
}

// }}}

// {{{ profileButtonsHandler

func profileButtonsHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	sesh,_ := ui.GetUserSession(ctx)
	cdb := complaintdb.NewDB(ctx)
	cp,_ := cdb.LookupProfile(sesh.Email)
	
	var params = map[string]interface{}{ "Buttons": cp.ButtonId }
	if err := templates.ExecuteTemplate(w, "buttons", params); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// }}}
// {{{ profileButtonAddHandler

func sanitizeButtonId(in string) string {
	return strings.Replace(strings.ToUpper(in), " ", "", -1)
}

func profileButtonAddHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	cdb := complaintdb.NewDB(ctx)
	sesh,_ := ui.GetUserSession(ctx)
	
	cp, _ := cdb.LookupProfile(sesh.Email)

	if r.FormValue("NewButtonId") != "" {
		id := sanitizeButtonId(r.FormValue("NewButtonId"))
		if len(id) != 16 {
			http.Error(w, fmt.Sprintf("Button ID must have sixteen characters; only got %d", len(id)),
				http.StatusInternalServerError)
			return
		}

		// Check we don't have the button registered already. This isn't super safe.
		if existing,err := cdb.LookupAllProfiles(cdb.NewProfileQuery().ByButton(id)); err != nil {
			http.Error(w, err.Error(),http.StatusInternalServerError)
			return
		} else if len(existing) != 0 {
			http.Error(w, fmt.Sprintf("Button '%s' is already claimed", id), http.StatusBadRequest)
			return
		}

		cp.ButtonId = append(cp.ButtonId, id)

		if err := cdb.PersistProfile(*cp); err != nil {
			cdb.Errorf("profileUpdate: cdb.Put: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	
	var params = map[string]interface{}{ "Buttons": cp.ButtonId }
	if err := templates.ExecuteTemplate(w, "buttons", params); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// }}}
// {{{ profileButtonDeleteHandler

func profileButtonDeleteHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	cdb := complaintdb.NewDB(ctx)
	sesh,_ := ui.GetUserSession(ctx)
	cp,_ := cdb.LookupProfile(sesh.Email)

	str := "OK\n--\n"

	// Look for the key whose value is DELETE. This is kinda lazy.
	r.ParseForm()
	removeId := ""
	for key, values := range r.Form {   // range over map
		for _, value := range values {    // range over []string
			str += fmt.Sprintf("* {%s} : {%s}\n", key, value)
			if value == "DELETE" {
				removeId = key
				break
			}
		}
	}

	if removeId == "" {
		http.Error(w, "Could not find button ID in form ?", http.StatusBadRequest)
		return
	}

	newIds := []string{}
	for _,id := range cp.ButtonId {
		if id != removeId { newIds = append(newIds, id) }
	}
	cp.ButtonId = newIds

	if err := cdb.PersistProfile(*cp); err != nil {
		cdb.Errorf("profileUpdate: cdb.Put: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var params = map[string]interface{}{ "Buttons": cp.ButtonId }
	if err := templates.ExecuteTemplate(w, "buttons", params); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// }}}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
