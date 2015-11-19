package g

import (	
	"net/http"

	"appengine"
	"appengine/user"

	"sessions"
)

var (
	callbackUrlPath = "/login/google"
)

func init() {
	http.HandleFunc(callbackUrlPath, loginHandler)
}

// If the user wants to login via Google, we will redirect them to this URL
func GetLoginUrl(r *http.Request, fromScratch bool) string {
	c := appengine.NewContext(r)

	destURL := r.URL
	destURL.Path = callbackUrlPath
	
	// Check for the case where the user remains logged in (via appengine cookie),
	// but clicked 'Logout' on our app; these guys shouldn't be bounced back to the "do you
	// grant this app permissions" page.
	u := user.Current(c)
	if !fromScratch && u != nil {
		return destURL.String()
	}

	// This is a 'start from scratch' Google login URL, that involves re-selecting the google acct.
	url, err := user.LoginURL(c, destURL.String())
	if err != nil {
		c.Errorf("Oh no, g.GetLoginUrl, %v", err)
		return ""
	}
	return url
}

// If the user logs in (and grants permission), they will be redirected here
func loginHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	u := user.Current(c)
	if u == nil {
		c.Errorf("No identifiable Google user; is this browser in privacy mode ?")
		http.Error(w, "No identifiable Google user; is this browser in privacy mode ?", http.StatusInternalServerError)
		return
	}
	//c.Infof(" ** Google user logged in ! [%s]", u.Email)

	// Snag their email address forever more
	session := sessions.Get(r)
	session.Values["email"] = u.Email
	session.Save(r,w)

	// Now head back to the main page
	http.Redirect(w, r, "/", http.StatusFound)
}
