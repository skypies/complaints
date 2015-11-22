package sessions

import (
	"net/http"
	sessions "github.com/gorilla/sessions"
)

var sessionStore *sessions.CookieStore

func Init(key, prevkey string) {
	sessionStore = sessions.NewCookieStore(
		[]byte(key), nil,
		[]byte(prevkey), nil)

}

func Get(r *http.Request) *sessions.Session {
	session, _ := sessionStore.Get(r, "serfr0")
	return session
}

// Need to call `session.Save(r,w)` to update it
