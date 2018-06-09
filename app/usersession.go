package complaints

/* Common code for pulling out a user session cookie, populating a Context, etc.
 * Users that aren't logged in will be redirected to the specified URL.

func init() {
  http.HandleFunc("/deb", HandleWithSession(debHandler, "/")) // If no cookie, redirects to "/"
}

func debHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	sesh,ok := GetUserSession(ctx)
	str := fmt.Sprintf("OK\nemail=%s, ok=%v\n", sesh.Email, ok) 
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(str))
}

 */

import(
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"

	"github.com/skypies/complaints/sessions"
)

// Pretty much all handlers should expect to be able to pluck this object out of their Context
type UserSession struct {
	Email        string     // case sensitive, sadly
	CreatedAt    time.Time  // when the user last went through the OAuth2 dance
	hasCreatedAt bool
}

func (us UserSession)HasCreatedAt() bool { return us.hasCreatedAt }

// To prevent other libs colliding in the context.Value keyspace, use this private key
type contextKey int
const sessionEmailKey contextKey = 0

type baseHandler    func(http.ResponseWriter, *http.Request)
type contextHandler func(context.Context, http.ResponseWriter, *http.Request)

func HandleWithSession(ch contextHandler, ifNoSessionRedirectTo string) baseHandler {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx,_ := context.WithTimeout(appengine.NewContext(r), 55 * time.Second)

		session,err := sessions.Get(r)
		if err != nil {
			log.Errorf(ctx, "session.Get failed with err: %v", err)
			log.Errorf(ctx, "cookies found: %d", len(r.Cookies()))
			for _,c := range r.Cookies() {
				if c.Name != "serfr0" { continue }
				log.Errorf(ctx, "serfr0 cookie: expires=%v (remain=%s), maxage=%v, secure=%v, httponly=%v %s",
					c.Expires, time.Until(c.Expires), c.MaxAge, c.Secure, c.HttpOnly, c)
			}
		}
		
		if strings.HasPrefix(r.UserAgent(), "Google") {
			// Robot - do nothing

		} else if session.Values["email"] == nil {
			reqBytes,_ := httputil.DumpRequest(r, true)
			log.Errorf(ctx, "session was empty (sessions.Debug=%v)", sessions.Debug)
			//for _,c := range r.Cookies() {
			//	log.Errorf(ctx, "cookie: %s", c)
			//}
			log.Errorf(ctx, "req: %s", reqBytes)

			// If we have a URL to redirect to, in cases of no session, then do it
			if ifNoSessionRedirectTo != "" {
				http.Redirect(w, r, ifNoSessionRedirectTo, http.StatusFound)
				return
			}

		} else {
			sesh := UserSession{Email: session.Values["email"].(string)}

			if session.Values["tstamp"] != nil {
				tstampStr := session.Values["tstamp"].(string)
				tstamp,_ := time.Parse(time.RFC3339, tstampStr)
				sesh.CreatedAt = tstamp
				sesh.hasCreatedAt = true // time.IsZero seems useless
			}
			
			ctx = context.WithValue(ctx, sessionEmailKey, sesh)
		}

		// Call the underlying handler, with our shiny context
		ch(ctx, w, r)
	}
}

// Underlying handlers should call this to get their session object
func GetUserSession(ctx context.Context) (UserSession,bool) {
	sesh, ok := ctx.Value(sessionEmailKey).(UserSession)
	return sesh, ok
}
