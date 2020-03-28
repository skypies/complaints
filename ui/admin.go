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
	ui.InitSessionStore(config.Get("sessions.key"), config.Get("sessions.prevkey"))

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

// Assert that the user is logged in, and is an admin user; if so, then run the handler,
// else return a 401. E.g.:
// 	 http.HandleFunc("/foo/bar", ui.HasAdmin(SomeNormalHandler))

func HasAdmin(bh baseHandler) baseHandler {
	return func(w http.ResponseWriter, r *http.Request) {
		fallback := func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
			http.Error(w, "This URL requires you to be logged in", http.StatusUnauthorized)
		}

		seshChecker := func(bh baseHandler) contextHandler {
			return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
				sesh,hadSesh := GetUserSession(ctx)
				if !hadSesh || !IsAdmin(sesh.Email){
					http.Error(w, "This URL requires admin access", http.StatusUnauthorized)
					return
				}
				
				// Just run the handler
				bh(w,r)
			}
		}
		
		handler := WithCtx(WithSession(seshChecker(bh),fallback))	
		handler(w,r)
	}
}
