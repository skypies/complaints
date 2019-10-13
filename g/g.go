package g

import (
	"log"
	"net/http"

	// "google.golang.org/ appengine"
	// "google.golang.org/ appengine/log"
	// "google.golang.org/ appengine/user"

	_ "github.com/skypies/complaints/ui"
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
	/*
	ctx := r.Context()

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
		log.Printf("g.GetLoginUrl, %v", err)
		return ""
	}
	url, err := user.LogoutURL(ctx, loginUrl)
	if err != nil {
		log.Printf("g.GetLogoutUrl, %v", err)
		return ""
	}

	return url
*/
	// FIXME: rewrite Google login handling
	return "https://google.com/"
}

// If the user logs in (and grants permission), they will be redirected here
func loginHandler(w http.ResponseWriter, r *http.Request) {
	// FIXME: rewrite Google login handling
	// ctx := r.Context()
	// u := user.Current(ctx)
	if true { // u == nil {
		log.Printf("No identifiable Google user; is this browser in privacy mode ?")
		http.Error(w, "No identifiable Google user; is this browser in privacy mode ?", http.StatusInternalServerError)
		return
	}
	//c.Infof(" ** Google user logged in ! [%s]", u.Email)

	// Snag their email address forever more; writes cookie into w
	//	ui.CreateSession(ctx, w, r, ui.UserSession{Email:u.Email})

	// Now head back to the main page
	// log.Printf("new session saved for %s (G)", u.Email)
	http.Redirect(w, r, "/", http.StatusFound)
}
