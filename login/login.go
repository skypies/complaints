package login

// Generic oauth2 stuff, and impls for Google and Facebook

import(
	"net/http"
)

type Oauth2er interface {
	GetLoginUrl(w http.ResponseWriter, r *http.Request) string
	GetLogoutUrl(w http.ResponseWriter, r *http.Request) string
	CallbackToEmail(r *http.Request) (string, error)
}

var(
	Host                        = "https://stop.jetnoise.net"
	//host                        = "http://localhost:8080"
	GoogleCallbackRelativeUrl   = "/tmp/login/google"
	FacebookCallbackRelativeUrl = "/tmp/login/facebook"

	Goauth2 Oauth2er
	Fboauth2 Oauth2er
)

func init() {
	Goauth2  = NewGoogleOauth2  (Host + GoogleCallbackRelativeUrl)
	Fboauth2 = NewFacebookOauth2(Host + FacebookCallbackRelativeUrl)

	http.HandleFunc(GoogleCallbackRelativeUrl,    NewOauth2Handler(Goauth2))
	http.HandleFunc(FacebookCallbackRelativeUrl,  NewOauth2Handler(Fboauth2))
}

func NewOauth2Handler(oauth2 Oauth2er) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if email,err := oauth2.CallbackToEmail(r); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return

		} else {
			//ui.CreateSession(ctx, w, r, ui.UserSession{Email:u.Email})

			// Now head back to the main page
			// log.Printf("new session saved for %s (G)", u.Email)
			http.Redirect(w, r, "/#"+email, http.StatusFound)
		}
	}
}

// Helper, because r.URL is weirdly unpopulated most of the time
func makeUrlAbsolute(r *http.Request, relativePath string) string {
	new := r.URL
	new.Path = relativePath

	if new.Scheme == "" { new.Scheme = "https" }
	if new.Host == "" { new.Host = r.Host }

	return new.String()
}
