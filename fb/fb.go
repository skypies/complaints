package fb

import (	
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	// "google.golang.org/ appengine"
	// "google.golang.org/ appengine/log"
	// "google.golang.org/ appengine/urlfetch"

	"golang.org/x/oauth2"
	fboauth2 "golang.org/x/oauth2/facebook"	

	"github.com/skypies/complaints/ui"
)

var (
	callbackURLPath = "/login/facebook"

	// These globals need to be set before the functions below are executed
	AppId = ""
	AppSecret = ""
)

func init() {
	http.HandleFunc(callbackURLPath, loginHandler)
}

func getConfig(r *http.Request) *oauth2.Config {
	// Use current URL as a template for the redirect
	destURL := r.URL
	destURL.Path = callbackURLPath
	destURL.RawQuery = ""

	return &oauth2.Config{
 		ClientID:     AppId,
 		ClientSecret: AppSecret,
 		RedirectURL:  destURL.String(),
 		Scopes:       []string{"email"},
 		Endpoint:     fboauth2.Endpoint,
 	}
}

// If the user wants to login via facebook, we will redirect them to this URL
func GetLoginUrl(r *http.Request) string {
 	return getConfig(r).AuthCodeURL("foo")  // "foo" is opaque state FB will send back to us.
}

// If the user logs in (and grants permission), they will be redirected here
func loginHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	code := r.FormValue("code")

	// Unpack the token
	token, err := getConfig(r).Exchange(ctx, code)
	if err != nil {
		log.Printf("/fb/login: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	// Use the token to access the FB API on the user's behalf; simply get their email address
	client := &http.Client{}
	var resp *http.Response
	url := "https://graph.facebook.com/me?fields=email&access_token=" + token.AccessToken
	if resp,err = client.Get(url); err != nil {
		log.Printf("/fb/login: client.Get: %v", err)		
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf ("Bad FB fetch status: %v", resp.Status)
		log.Printf("/fb/login: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}	
	var jsonMap map[string]interface{}
	if err = json.NewDecoder(resp.Body).Decode(&jsonMap); err != nil {
		log.Printf("/fb/login: bad resp parse%v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	} else if jsonMap["email"] == nil {
		log.Printf("/fb/login: no email field found:\n%#v", jsonMap)
		http.Error(w, "fb login, no email field", http.StatusInternalServerError)
		return
	}

	// Snag their email address forever more; writes cookie into w
	ui.CreateSession(ctx, w, r, ui.UserSession{Email: jsonMap["email"].(string)})

	log.Printf("new session saved for %s (FB)", jsonMap["email"])
	http.Redirect(w, r, "/", http.StatusFound)
}
