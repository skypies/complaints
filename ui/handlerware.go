package ui

import(
	"fmt"
	"log"
	"net/http"
	"time"
	
	"golang.org/x/net/context"
)

/* Common code for pulling out a user session cookie, populating a Context, etc. */

type baseHandler    func(http.ResponseWriter, *http.Request)
type contextHandler func(context.Context, http.ResponseWriter, *http.Request)

// To prevent other libs colliding with us in the context.Value keyspace, use these private keys
type contextKey int
const(
	sessionKey contextKey = iota
	templatesKey
)


// Underlying handlers should call this to get their session object
func GetUserSession(ctx context.Context) (UserSession, bool) {
	opt, ok := ctx.Value(sessionKey).(UserSession)
	return opt, ok
}
func SetUserSession(ctx context.Context, sesh UserSession) context.Context {
	return context.WithValue(ctx, sessionKey, sesh)
}

// Some convenience combos
func WithCtxSession(ch,fallback contextHandler) baseHandler {
	return WithCtx(WithSession(ch,fallback))
}

// Some convenience combos
func WithCtxTlsSession(ch,fallback contextHandler) baseHandler {
	return WithCtx(WithTLS(WithSession(ch,fallback)))
}


// Outermost wrapper; all other wrappers take (and return) contexthandlers
func WithCtx(ch contextHandler) baseHandler {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx,_ := context.WithTimeout(r.Context(), 550 * time.Second)
		ch(ctx,w,r)
	}
}

// Redirects to a https:// version of the URL, if needed
func WithTLS(ch contextHandler) contextHandler {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		if r.URL.Scheme == "http" {
			tlsUrl := r.URL
			tlsUrl.Scheme = "https"
			http.Redirect(w, r, tlsUrl.String(), http.StatusFound)
			return
		}

		ch(ctx, w, r)
	}
}

// If there is a user session, runs the specified handler; else runs
// the fallback handler (which presumably starts a login flow). Adds some debug
// logging, to try and illuminate how users end up without sessions.
func WithSession(ch contextHandler, fallback contextHandler) contextHandler {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		crumbs := CrumbTrail{}
		
		// First, extract prev breadcrumbs and log them
		cookies := map[string]string{}
		for _,c := range r.Cookies() {
			crumbs.Add("C:"+c.Name)
			cookies[c.Name] = c.Value
		}
		if val,exists := cookies["serfr0crumbs"]; exists {
			log.Printf("serfr0crumbs in : %s", val)
		} 

		handler := fallback

		if _,exists := cookies["serfr0"]; exists {
			sesh,err := Req2Session(r, &crumbs)
			if err == nil && !sesh.IsEmpty() {
				// Stash the session in the context, and move on to the proper handler
				ctx = SetUserSession(ctx, sesh)
				handler = ch

			} else {
				if err != nil { log.Printf("req2session err: " + err.Error()) }
				log.Printf("crumbs: " + crumbs.String())
			}

		} else {
			crumbs.Add("NoSerfrCookie")
		}
		
		// Before invoking final handler, log breadcrumb trail, and stash in cookie
		log.Printf("serfr0crumbs out: %s", crumbs)
		cookie := http.Cookie{
			Name: "serfr0crumbs",
			Value: crumbs.String(),
			Expires:time.Now().AddDate(1,0,0),
		}
		http.SetCookie(w, &cookie)

		if handler == nil {
			log.Printf("WithSession had no session, no fallbackHandler")
			http.Error(w, fmt.Sprintf("no session, no fallbackHandler (%s)", r.URL), http.StatusInternalServerError)
			return
		}
		
		handler(ctx, w, r)
	}
}
