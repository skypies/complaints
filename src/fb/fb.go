package fb

import (	
	"encoding/json"
	"fmt"
	"net/http"

	"appengine"

	// https://github.com/golang/oauth2
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	fboauth2 "golang.org/x/oauth2/facebook"	
	newappengine "google.golang.org/appengine"
	newurlfetch "google.golang.org/appengine/urlfetch"

	"sessions"
)

var (
	callbackURLPath = "/login/facebook"

	facebookAppId = "871184712972286"
	facebookAppSecret = "2eadf024d9bdbc0bd1818d8702e59c45"
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
 		ClientID:     facebookAppId,
 		ClientSecret: facebookAppSecret,
 		RedirectURL:  destURL.String(),
 		Scopes:       []string{"email"},
 		Endpoint:     fboauth2.Endpoint,
 	}
}

// If the user wants to login via facebook, we will redirect them to this URL
func GetLoginUrl(r *http.Request) string {
 	return getConfig(r).AuthCodeURL("foo")  // "foo" is opaque state FB will send back to us. Ignore
}

// If the user logs in (and grants permission), they will be redirected here
func loginHandler(w http.ResponseWriter, r *http.Request) {
	var c context.Context = newappengine.NewContext(r)
	code := r.FormValue("code")

	// Unpack the token
	token, err := getConfig(r).Exchange(c, code)
	if err != nil {
		appengine.NewContext(r).Errorf("/fb/login: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	// Use the token to access the FB API on the user's behalf; simply get their email address
	client := newurlfetch.Client(c)
	var resp *http.Response
	url := "https://graph.facebook.com/me?fields=email&access_token=" + token.AccessToken
	if resp,err = client.Get(url); err != nil {
		appengine.NewContext(r).Errorf("/fb/login: client.Get: %v", err)		
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf ("Bad FB fetch status: %v", resp.Status)
		appengine.NewContext(r).Errorf("/fb/login: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}	
	var jsonMap map[string]interface{}
	if err = json.NewDecoder(resp.Body).Decode(&jsonMap); err != nil {
		appengine.NewContext(r).Errorf("/fb/login: bad resp parse%v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	session := sessions.Get(r)
	session.Values["email"] = jsonMap["email"]
	session.Save(r,w)
	
	// appengine.NewContext(r).Infof(" ** Facebook user logged in ! [%s]", jsonMap["email"])

	http.Redirect(w, r, "/", http.StatusFound)
}
