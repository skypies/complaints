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


// Convenience combos

// Assert that the user is logged in, and is an admin user; if so, then run the handler,
// else return a 401. E.g.:
// 	 http.HandleFunc("/foo/bar", ui.HasAdmin(SomeBaseHandler))
func HasAdmin(bh baseHandler) baseHandler {
	return WithCtx(WithSession(WithAdmin(WithoutCtx(bh)), authFallback))	
}

// 	 http.HandleFunc("/foo/bar", ui.WithCtxAdmin(SomeContextHandler))
func WithCtxAdmin(ch contextHandler) baseHandler {
	return WithCtx(WithSession(WithAdmin(ch), authFallback))	
}


// The main func

func WithAdmin(ch contextHandler) contextHandler {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		sesh,hadSesh := GetUserSession(ctx)
		if !hadSesh || !IsAdmin(sesh.Email){
			http.Error(w, "This URL requires admin access", http.StatusUnauthorized)
			return
		}
				
		// We have admin rights - run the handler
		ch(ctx,w,r)
	}
}

func authFallback(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	http.Error(w, "This URL requires you to be logged in", http.StatusUnauthorized)
}
