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
	url, err := user.LoginURL(ctx, destURL.String())
	if err != nil {
		log.Errorf(ctx, "g.GetLoginUrl, %v", err)
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
		log.Errorf(ctx, "sessions.Get: %v", err)
	}
	session.Values["email"] = u.Email
	session.Values["tstamp"] = time.Now().Format(time.RFC3339)
	if err := session.Save(r,w); err != nil {
		log.Errorf(ctx, "session.Save: %v", err)
	}

	// Now head back to the main page
	http.Redirect(w, r, "/", http.StatusFound)
}
