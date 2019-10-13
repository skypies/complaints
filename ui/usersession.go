package ui

import(
	"fmt"
	"log"
	"net/http"
	"time"

	"golang.org/x/net/context"

	// "google.golang.org/ appengine/log"

	gsessions "github.com/gorilla/sessions"
)

var sessionStore *gsessions.CookieStore
var Debug string

func InitSessionStore(key, prevkey string) {
	sessionStore = gsessions.NewCookieStore(
		[]byte(key), nil,
		[]byte(prevkey), nil)

	sessionStore.MaxAge(86400 * 180)

	Debug = fmt.Sprintf("ui.usersessions.Init (%d,%d bytes)", len(key), len(prevkey))
}

// Pretty much all handlers should expect to be able to pluck this object out of their Context
type UserSession struct {
	Email        string     // case sensitive, sadly
	CreatedAt    time.Time  // when the user last went through the OAuth2 dance
}

func (us UserSession)IsEmpty() bool { return us.Email == "" }

// Assumes the serfr0 cookie is present
func Req2Session(r *http.Request, crumbs *CrumbTrail) (UserSession, error) {
	// If not found, returns an empty session
	session,err := sessionStore.Get(r, "serfr0")
	if err != nil {
		crumbs.Add("GDecodeFailed")
		return UserSession{}, fmt.Errorf("Req2Session: sessionStore.Get: %v", err)
	}

	if session.IsNew {
		crumbs.Add("NewGSession")
		return UserSession{}, nil

	} else if session.Values["email"] == nil {
		crumbs.Add("LoggedOutSession")
		return UserSession{}, nil
	}

	crumbs.Add("SessionRetrieved")

	// crumbs.Add("E:"+session.Values["email"].(string))
	
	tstampStr := session.Values["tstamp"].(string)
	tstamp,_ := time.Parse(time.RFC3339, tstampStr)

	crumbs.Add(fmt.Sprintf("Age:%s", time.Since(tstamp)))
	
	userSesh := UserSession{
		Email: session.Values["email"].(string),
		CreatedAt: tstamp,
	}

	// In case of a new session object, give it a long cookie lifetime
	//session.Options.MaxAge = 86400 * 180 // Default is 4w.

	return userSesh, nil
}

func CreateSession(ctx context.Context, w http.ResponseWriter, r *http.Request, sesh UserSession) {
	session,err := sessionStore.Get(r, "serfr0")
	if err != nil {
		// This isn't usually an important error (the session was most likely expired, which is why
		// we're logging in) - so log as Info, not Error.
		log.Printf("CreateSession: sessionStore.Get [failing is OK for this call] had err: %v", err)
	}

	session.Values["email"] = sesh.Email
	session.Values["tstamp"] = time.Now().Format(time.RFC3339)
	if err := session.Save(r,w); err != nil {
		log.Printf("CreateSession: session.Save: %v", err)
	}
	log.Printf("CreateSession OK for %s", sesh.Email)
}

func OverwriteSessionToNil(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	// Ignore errors; we just want an empty one
	session,_ := sessionStore.Get(r, "serfr0")

	session.Values["email"] = nil
	session.Values["tstamp"] = nil

	session.Save(r, w)	
	log.Printf("OverwriteSessionToNil done")
}
