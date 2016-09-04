package sessions

import (
	"net/http"
	gsessions "github.com/gorilla/sessions"
)

var sessionStore *gsessions.CookieStore

func Init(key, prevkey string) {
	sessionStore = gsessions.NewCookieStore(
		[]byte(key), nil,
		[]byte(prevkey), nil)
}

func Get(r *http.Request) *gsessions.Session {
	session, _ := sessionStore.Get(r, "serfr0")

	session.Options.MaxAge = 86400 * 180 // Default is 4w. This might be the wrong place to set it.

	return session
}

// Need to call `session.Save(r,w)` to update it
