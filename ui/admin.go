package ui

// This should (a) evolve to handle arbitrary groups, and (b) move into skypies/util

import(
	"net/http"
	"strings"
	
	"golang.org/x/net/context"

	"github.com/skypies/complaints/config"
)

var(
	adminEmails = map[string]int{}
)

func init() {
	//AdminEmails := map[string]int{}
	for _,e := range strings.Split(config.Get("users.admin"), ",") {
		adminEmails[strings.ToLower(e)] = 1
	}
}

// I'm not sure this should ever be needed outside of this file, but hey.
func IsAdmin(email string) bool {
	_,exists := adminEmails[strings.ToLower(email)]
	return exists
}

// Convenience combos.
// In all of them, use the main handler as the authFallback too, because we want to run it whether we find a
// usersession or not.
func WithCtxAdmin(ch contextHandler) baseHandler {
	return WithCtx(WithSession(WithAdmin(ch), WithAdmin(ch)))
}
func WithCtxTlsAdmin(ch contextHandler) baseHandler {
	return WithCtx(WithTLS(WithSession(WithAdmin(ch), WithAdmin(ch))))
}
func HasAdmin(bh baseHandler) baseHandler {
	handler := WithAdmin(WithoutCtx(bh))
	return WithCtx(WithSession(handler, handler))
}

// WithAdmin validates that the request has admin privileges, and runs the handler (or returns 401).
// Privileges are either that the user is logged in, and is an admin; or that the request came from
// an appengine cron job.
// https://cloud.google.com/appengine/docs/flexible/nodejs/scheduling-jobs-with-cron-yaml#validating_cron_requests
func WithAdmin(ch contextHandler) contextHandler {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		cron := r.Header.Get("x-appengine-cron")
		sesh,hadSesh := GetUserSession(ctx)
		//log.Printf("WithAdmin: hadSesh=%v, sesh=%#v, cron=%q", hadSesh, sesh, cron)

		haveAdmin := false
		if cron != "" {
			haveAdmin = true
		} else if hadSesh && IsAdmin(sesh.Email) {
			haveAdmin = true
		}

		if !haveAdmin {
			errstr := "This URL requires you to be logged in"
			if hadSesh {
				errstr = "This URL requires admin access"
			}
			http.Error(w, errstr, http.StatusUnauthorized)
			return
		}
				
		// We have admin rights - run the handler
		ch(ctx,w,r)
	}
}
