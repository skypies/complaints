package g

import (
	"net/http"
	"time"

	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/user"

	"github.com/skypies/complaints/sessions"
)

var (
	callbackUrlPath = "/login/google"
)

func init() {
	// context.ClearHandler(loginHandler)
	http.HandleFunc(callbackUrlPath, loginHandler)
}

// If the user wants to login via Google, we will redirect them to this URL
func GetLoginUrl(r *http.Request, fromScratch bool) string {
	ctx := appengine.NewContext(r)

	destURL := r.URL
	destURL.Path = callbackUrlPath
	
	// Check for the case where the user remains logged in (via appengine cookie),
	// but clicked 'Logout' on our app; these guys shouldn't be bounced back to the "do you
	// grant this app permissions" page.
	u := user.Current(ctx)
	if !fromScratch && u != nil {
		return destURL.String()
	}

	// This is a 'start from scratch' Google login URL, that involves re-selecting the google acct.
	// Google multiple accounts aren't working as they used to; simply redirecting to the loginURL
	// no longer allows a user to select. Instead force them to entirely log out of all their google
	// accounts (ouch) and then login again (with desired account), and finally back to the site.
	loginUrl, err := user.LoginURL(ctx, destURL.String())
	if err != nil {
		log.Errorf(ctx, "g.GetLoginUrl, %v", err)
		return ""
	}
	url, err := user.LogoutURL(ctx, loginUrl)
	if err != nil {
		log.Errorf(ctx, "g.GetLogoutUrl, %v", err)
		return ""
	}

	return url
}

// If the user logs in (and grants permission), they will be redirected here
func loginHandler(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	u := user.Current(ctx)
	if u == nil {
		log.Errorf(ctx, "No identifiable Google user; is this browser in privacy mode ?")
		http.Error(w, "No identifiable Google user; is this browser in privacy mode ?", http.StatusInternalServerError)
		return
	}
	//c.Infof(" ** Google user logged in ! [%s]", u.Email)

	// Snag their email address forever more
	session,err := sessions.Get(r)
	if err != nil {
		// This isn't usually an important error (the session was most likely expired, which is why
		// we're logging in) - so log as Info, not Error.
		log.Debugf(ctx, "sessions.Get [failing is OK for this call] had err: %v", err)
	}
	session.Values["email"] = u.Email
	session.Values["tstamp"] = time.Now().Format(time.RFC3339)
	if err := session.Save(r,w); err != nil {
		log.Errorf(ctx, "session.Save: %v", err)
	}

	// Now head back to the main page
	log.Infof(ctx, "new session saved for %s (G)", u.Email)
	http.Redirect(w, r, "/", http.StatusFound)
}
