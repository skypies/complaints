package login

import (	
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/facebook"	

	"github.com/skypies/complaints/config"
)

type FacebookOauth2 struct {
	oauth2.Config
}

// {{{ NewFacebookOauth2

func NewFacebookOauth2(callbackUrl string) FacebookOauth2 {
	return FacebookOauth2{
		Config: oauth2.Config{
			RedirectURL:  callbackUrl,
			ClientID:     config.Get("facebook.oauth2.appid"),
			ClientSecret: config.Get("facebook.oauth2.secret"),
			Endpoint:     facebook.Endpoint,
			Scopes:       []string{"email"},
		},
	}
}

// }}}

// {{{ fboauth2.GetLoginUrl

// If the user wants to login via facebook, we will redirect them to this URL
func (fboauth2 FacebookOauth2)GetLoginUrl(w http.ResponseWriter, r *http.Request) string {
 	return fboauth2.Config.AuthCodeURL("foo")  // "foo" is opaque state FB will send back to us.
}

// }}}
// {{{ fboauth2.GetLogoutUrl

// We don't use this, as we don't have users juggling FB accounts and getting
// confused the way we do with Google accounts
func (fboauth2 FacebookOauth2)GetLogoutUrl(w http.ResponseWriter, r *http.Request) string {
	return "https://www.facebook.com/log.out#"
}

// }}}
// {{{ fboauth2.CallbackToEmail

func (fboauth2 FacebookOauth2)CallbackToEmail(r *http.Request) (string, error) {
	ctx := r.Context()
	code := r.FormValue("code")

	// Exchange the code for a token (can only do once, within 10m of getting the code)
	token, err := fboauth2.Config.Exchange(ctx, code)
	if err != nil {
		// str := fmt.Sprintf("\n\nStupid thing:-\n%#v\n--\ncode:%s\n--\n", fboauth2, code)
		return "", fmt.Errorf("/fb/callback.Exchange: %v", err)
	}

	// Use the token to access the FB API on the user's behalf; just get their email address
	client := &http.Client{}
	var resp *http.Response
	url := "https://graph.facebook.com/me?fields=email&access_token=" + token.AccessToken
	if resp,err = client.Get(url); err != nil {
		return "", fmt.Errorf("/fb/callback.Get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("/fb/callback bad HTTP status: %v", resp.Status)
	}

	var jsonMap map[string]interface{}
	if err = json.NewDecoder(resp.Body).Decode(&jsonMap); err != nil {
		return "", fmt.Errorf("/fb/callback bad JSON body: %v", err)
	} else if jsonMap["email"] == nil {
		return "", fmt.Errorf("/fb/callback no email in JSON body: %v", jsonMap)
	}

	email := jsonMap["email"].(string)

	return email, nil
}

// }}}


// {{{ -------------------------={ E N D }=----------------------------------

// Local variables:
// folded-file: t
// end:

// }}}
