package sessions

import (
	"net/http"
	sessions "github.com/gorilla/sessions"
)

var sessionStore = sessions.NewCookieStore([]byte("0xdeadbeef"))

func Get(r *http.Request) *sessions.Session {
	session, _ := sessionStore.Get(r, "serfr0")
	return session
}

// Need to call `session.Save(r,w)` to update it
