package login

// Generic oauth2 stuff, and impls for Google and Facebook

import(
	"net/http"
)

type Oauth2er interface {
	Name() string
	GetLoginUrl(w http.ResponseWriter, r *http.Request) string
	GetLogoutUrl(w http.ResponseWriter, r *http.Request) string
	CallbackToEmail(r *http.Request) (string, error)
}

func getCallbackRelativeUrl(o Oauth2er) string {
	return RedirectUrlStem + "/" +	o.Name()
}

type Oauth2SucessCallback func(w http.ResponseWriter, r *http.Request, email string) error

var(
	// The caller should configure these values - but *if you do*, they need to call login.Init()
	Host                        = "https://stop.jetnoise.net"
	RedirectUrlStem             = "/login" // oauth2 callbacks will register  under here
	AfterLoginRelativeUrl       = "/" // where the user finally ends up, after being logged in
	OnSuccessCallback           Oauth2SucessCallback
	
	// Individual oauth2 systems
	Goauth2 Oauth2er
	Fboauth2 Oauth2er
)

// The caller *must* call this, after they've set the vars above
func Init() {
	Goauth2  = NewGoogleOauth2()
	Fboauth2 = NewFacebookOauth2()

	http.HandleFunc(getCallbackRelativeUrl(Goauth2), NewOauth2Handler(Goauth2))
	http.HandleFunc(getCallbackRelativeUrl(Fboauth2), NewOauth2Handler(Fboauth2))
}

// Returns a standard requesthandler. When run, it handles the redirect from the oauth2
// provider, and if it gets an email address from the provider, will invoke the
// callback function with it, before redirecting to the provided URL.
func NewOauth2Handler(oauth2 Oauth2er) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if email,err := oauth2.CallbackToEmail(r); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return

		} else {
			if OnSuccessCallback != nil {
				if err := OnSuccessCallback(w,r,email); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}

			http.Redirect(w, r, AfterLoginRelativeUrl +"#email:"+email, http.StatusFound)
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
