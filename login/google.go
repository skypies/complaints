package login

// What I cloned from ...
// https://github.com/douglasmakey/oauth2-example/blob/master/handlers/oauth_google.go

import(
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/skypies/complaints/config"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const(
	GoogleOAuthEmailScope = "https://www.googleapis.com/auth/userinfo.email" // WTF to get this
	oauthGoogleUrlAPI = "https://www.googleapis.com/oauth2/v2/userinfo?access_token="
)

type GoogleOauth2 struct {
	oauth2.Config
}
func (g GoogleOauth2)Name() string { return "google" }

// {{{ NewGoogleOauth2

func NewGoogleOauth2() GoogleOauth2 {
	g := GoogleOauth2{
		Config: oauth2.Config{
			ClientID:     config.Get("google.oauth2.appid"),
			ClientSecret: config.Get("google.oauth2.secret"),
			Endpoint:     google.Endpoint,
			Scopes:       []string{GoogleOAuthEmailScope},
		},
	}

	// Build this from some package variables
	g.Config.RedirectURL = Host + getCallbackRelativeUrl(g)

	return g
}

// }}}

// {{{ goauth2.GetLoginUrl

func (goauth2 GoogleOauth2)GetLoginUrl(w http.ResponseWriter, r *http.Request) string {
	// AuthCodeURL is a token to protect the user from CSRF attacks. It
	// is also attached to the response as a cookie.
	oauthState := generateStateOauthCookie(w)
	return goauth2.Config.AuthCodeURL(oauthState)
}

// }}}
// {{{ goauth2.GetLogoutUrl

func (goauth2 GoogleOauth2)GetLogoutUrl(w http.ResponseWriter, r *http.Request) string {
	loginUrl := makeUrlAbsolute(r, "/")

	args1 := url.Values{}
	args1.Add("continue", loginUrl)

	args2 := url.Values{}
	args2.Add("continue", "https://appengine.google.com/_ah/logout?" + args1.Encode())
	
	// This weird thing bounces you through a "are you sure you want to redirect" notice,
	// which is fine. You can't just redirect anywhere, it has to go to a google url.
	return "https://www.google.com/accounts/Logout?" + args2.Encode()
}

// }}}
// {{{ goauth2.CallbackToEmail

func (goauth2 GoogleOauth2)CallbackToEmail(r *http.Request) (string, error) {
	// Read oauthState from Cookie
	if oauthState,err := r.Cookie("oauthstate"); err != nil {
		return "", fmt.Errorf("no oauth google cookie: %v", err)
	} else if r.FormValue("state") != oauthState.Value {
		return "", fmt.Errorf("invalid oauth google state")
	}

	email,err := goauth2.getEmailFromGoogle(r.FormValue("code"))
	return email,err
}

// }}}

// {{{ goauth2.getEmailFromGoogle

func (goauth2 GoogleOauth2)getEmailFromGoogle(code string) (string, error) {
	if data,err := goauth2.getUserDataFromGoogle(code); err != nil {
		return "", err
	} else {
		var jsonMap map[string]interface{}
		if err := json.Unmarshal([]byte(data), &jsonMap); err != nil {
			return "", fmt.Errorf("google oauth unmarshal:"+err.Error())
		}

		emailVar := jsonMap["email"];
		if emailVar == nil {
			return "", fmt.Errorf("google oauth callback: jsonmap had no 'email'\n%#v", jsonMap)
		}

		if email,ok := emailVar.(string); !ok {
			return "", fmt.Errorf("google oauth callback: 'email' not a string\n%#v", jsonMap)
		} else {
			return email, nil
		}
	}
}

// }}}
// {{{ goauth2.getUserDataFromGoogle

func (goauth2 GoogleOauth2)getUserDataFromGoogle(code string) ([]byte, error) {
	token, err := goauth2.Config.Exchange(context.Background(), code)
	if err != nil {
		return nil, fmt.Errorf("code exchange wrong: %s", err.Error())
	}

	response, err := http.Get(oauthGoogleUrlAPI + token.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed getting user info: %s", err.Error())
	}

	defer response.Body.Close()
	contents, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed read response: %s", err.Error())
	}

	return contents, nil
}

// }}}

// {{{ generateStateOauthCookie

func generateStateOauthCookie(w http.ResponseWriter) string {
	var expiration = time.Now().Add(20 * time.Minute)

	b := make([]byte, 16)
	rand.Read(b)
	state := base64.URLEncoding.EncodeToString(b)
	cookie := http.Cookie{Name: "oauthstate", Value: state, Expires: expiration}
	http.SetCookie(w, &cookie)

	return state
}

// }}}

// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
